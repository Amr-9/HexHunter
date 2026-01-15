//go:build opencl
// +build opencl

package solana

/*
#cgo CFLAGS: -I${SRCDIR}/../../../deps/opencl-headers
#cgo windows LDFLAGS: -L${SRCDIR}/../../../deps/lib -lOpenCL
#cgo linux LDFLAGS: -lOpenCL
#cgo darwin LDFLAGS: -framework OpenCL

#ifdef __APPLE__
#include <OpenCL/opencl.h>
#else
#include <CL/cl.h>
#endif

#include <stdlib.h>
#include <string.h>
*/
import "C"

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	_ "embed"
	"fmt"
	"log"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ethvanity/pkg/generator"
	"github.com/mr-tron/base58"
)

const (
	// Batch size for SolVanityCL kernel
	// The kernel uses occupied_bytes to determine how many bytes to use for GID offset
	// Increased to 1M for better GPU utilization
	solanaBatchSize     = 1 << 20 // 1,048,576 keys per batch
	solanaLocalWorkSize = 256     // Workgroup size
)

// SolanaGPUGenerator implements the Generator interface for Solana using OpenCL.
// Uses the SolVanityCL kernel with:
// - Built-in Ed25519 implementation (radix-2^25.5 field)
// - Hardcoded precomputed base tables
// - Inline SHA-512 and Base58 encoding
// - In-kernel prefix/suffix matching
type SolanaGPUGenerator struct {
	platform C.cl_platform_id
	device   C.cl_device_id
	clCtx    C.cl_context
	queue    C.cl_command_queue
	program  C.cl_program
	kernel   C.cl_kernel

	// buffers (SolVanityCL interface)
	bufSeed          C.cl_mem // Base seed (32 bytes)
	bufOutput        C.cl_mem // Output: length(1) + seed(32) = 33 bytes
	bufOccupiedBytes C.cl_mem // Number of bytes used for GID offset (1 byte)
	bufGroupOffset   C.cl_mem // Batch offset multiplier (1 byte)
	bufPrefix        C.cl_mem // Runtime prefix bytes (max 44 bytes)
	bufSuffix        C.cl_mem // Runtime suffix bytes (max 44 bytes)

	// stats
	attempts  uint64
	startTime time.Time

	// matching config
	prefix        string
	suffix        string
	caseSensitive bool
	matcher       *SolanaMatcher
}

// NewSolanaGPUGenerator creates a new Solana GPU-based generator.
func NewSolanaGPUGenerator() (*SolanaGPUGenerator, error) {
	g := &SolanaGPUGenerator{
		caseSensitive: true, // default to case sensitive
	}
	// Note: We don't initialize OpenCL here because we need prefix/suffix first
	// OpenCL will be initialized in Start()
	return g, nil
}

func (g *SolanaGPUGenerator) Name() string {
	return "GPU (Solana SolVanityCL)"
}

func (g *SolanaGPUGenerator) Stats() generator.Stats {
	attempts := atomic.LoadUint64(&g.attempts)
	elapsed := time.Since(g.startTime).Seconds()
	var hashRate float64
	if elapsed > 0 {
		hashRate = float64(attempts) / elapsed
	}
	return generator.Stats{
		Attempts:    attempts,
		HashRate:    hashRate,
		ElapsedSecs: elapsed,
	}
}

func (g *SolanaGPUGenerator) Start(ctx context.Context, config *generator.Config) (<-chan generator.Result, error) {
	resultChan := make(chan generator.Result, 1)
	g.startTime = time.Now()
	atomic.StoreUint64(&g.attempts, 0)

	g.prefix = config.Prefix
	g.suffix = config.Suffix
	g.matcher = NewSolanaMatcher(config.Prefix, config.Suffix)

	// Initialize OpenCL with the specific prefix/suffix
	if err := g.initOpenCL(); err != nil {
		return nil, fmt.Errorf("failed to initialize OpenCL: %w", err)
	}

	go g.runGPU(ctx, resultChan)
	return resultChan, nil
}

func (g *SolanaGPUGenerator) runGPU(ctx context.Context, resultChan chan<- generator.Result) {
	if err := g.createBuffers(); err != nil {
		log.Printf("Buffer creation failed: %v", err)
		return
	}
	defer g.releaseBuffers()

	// Host buffers
	baseSeed := make([]byte, 32)
	hostOutput := make([]byte, 33) // length(1) + seed(32)
	zeros := make([]byte, 33)

	// SolVanityCL uses occupied_bytes to specify how many bytes of the seed
	// are used for the GID offset. With 3 bytes, we can address up to 16M work items.
	var occupiedBytes byte = 3
	var groupOffset byte = 0
	var ret C.cl_int

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// 1. Generate random base seed
			rand.Read(baseSeed)

			// 2. Upload seed
			ret = C.clEnqueueWriteBuffer(g.queue, g.bufSeed, C.CL_TRUE, 0, 32,
				unsafe.Pointer(&baseSeed[0]), 0, nil, nil)
			if ret != C.CL_SUCCESS {
				log.Printf("Failed to write seed: %d", ret)
				return
			}

			// 3. Upload occupied_bytes
			ret = C.clEnqueueWriteBuffer(g.queue, g.bufOccupiedBytes, C.CL_TRUE, 0, 1,
				unsafe.Pointer(&occupiedBytes), 0, nil, nil)
			if ret != C.CL_SUCCESS {
				log.Printf("Failed to write occupied_bytes: %d", ret)
				return
			}

			// 4. Upload group_offset
			ret = C.clEnqueueWriteBuffer(g.queue, g.bufGroupOffset, C.CL_TRUE, 0, 1,
				unsafe.Pointer(&groupOffset), 0, nil, nil)
			if ret != C.CL_SUCCESS {
				log.Printf("Failed to write group_offset: %d", ret)
				return
			}

			// 5. Clear output buffer
			ret = C.clEnqueueWriteBuffer(g.queue, g.bufOutput, C.CL_TRUE, 0, 33,
				unsafe.Pointer(&zeros[0]), 0, nil, nil)
			if ret != C.CL_SUCCESS {
				log.Printf("Failed to clear output: %d", ret)
				return
			}

			// 6. Run kernel
			globalSize := C.size_t(solanaBatchSize)
			localSize := C.size_t(solanaLocalWorkSize)
			ret = C.clEnqueueNDRangeKernel(g.queue, g.kernel, 1, nil,
				&globalSize, &localSize, 0, nil, nil)
			if ret != C.CL_SUCCESS {
				log.Printf("Kernel failed: %d", ret)
				return
			}

			// 7. Read output
			ret = C.clEnqueueReadBuffer(g.queue, g.bufOutput, C.CL_TRUE, 0, 33,
				unsafe.Pointer(&hostOutput[0]), 0, nil, nil)
			if ret != C.CL_SUCCESS {
				log.Printf("Read output failed: %d", ret)
				return
			}

			// 8. Check if found (output[0] = length, non-zero means match found)
			if hostOutput[0] != 0 {
				foundSeed := hostOutput[1:33]

				// Verify with Go's ed25519 implementation
				privKey := ed25519.NewKeyFromSeed(foundSeed)
				pubKey := privKey.Public().(ed25519.PublicKey)
				address := base58.Encode(pubKey)

				// Double-check with matcher
				if g.matcher.Matches(address) {
					resultChan <- generator.Result{
						Network:    generator.Solana,
						Address:    address,
						PrivateKey: base58.Encode(privKey),
					}
					return
				}
				// False positive from kernel, continue
				log.Printf("GPU false positive: address=%s (expected prefix=%s)", address, g.prefix)
			}

			// 9. Update stats
			atomic.AddUint64(&g.attempts, uint64(solanaBatchSize))

			// 10. Increment group offset for next batch
			groupOffset++
			if groupOffset == 0 {
				// Overflow, get new random seed
				groupOffset = 0
			}
		}
	}
}

func (g *SolanaGPUGenerator) initOpenCL() error {
	var ret C.cl_int
	var numPlatforms C.cl_uint
	if C.clGetPlatformIDs(0, nil, &numPlatforms) != C.CL_SUCCESS || numPlatforms == 0 {
		return fmt.Errorf("no OpenCL platforms")
	}
	platforms := make([]C.cl_platform_id, numPlatforms)
	C.clGetPlatformIDs(numPlatforms, &platforms[0], nil)
	g.platform = platforms[0]

	var numDevices C.cl_uint
	if C.clGetDeviceIDs(g.platform, C.CL_DEVICE_TYPE_GPU, 0, nil, &numDevices) != C.CL_SUCCESS || numDevices == 0 {
		return fmt.Errorf("no GPU devices")
	}
	devices := make([]C.cl_device_id, numDevices)
	C.clGetDeviceIDs(g.platform, C.CL_DEVICE_TYPE_GPU, numDevices, &devices[0], nil)
	g.device = devices[0]

	g.clCtx = C.clCreateContext(nil, 1, &g.device, nil, nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("context failed: %d", ret)
	}

	g.queue = C.clCreateCommandQueue(g.clCtx, g.device, 0, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("queue failed: %d", ret)
	}

	// Load generic kernel (prefix/suffix are now runtime parameters)
	kernelSrc, err := LoadSolanaKernel()
	if err != nil {
		return fmt.Errorf("failed to load kernel source: %w", err)
	}

	src := C.CString(kernelSrc)
	defer C.free(unsafe.Pointer(src))

	length := C.size_t(len(kernelSrc))
	g.program = C.clCreateProgramWithSource(g.clCtx, 1, &src, &length, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("program creation failed: %d", ret)
	}

	// Use optimization flags for faster math operations
	// -cl-fast-relaxed-math: Enables fast math optimizations
	// -cl-mad-enable: Enables multiply-add fusion
	buildOptions := C.CString("-cl-fast-relaxed-math -cl-mad-enable")
	defer C.free(unsafe.Pointer(buildOptions))

	ret = C.clBuildProgram(g.program, 1, &g.device, buildOptions, nil, nil)
	if ret != C.CL_SUCCESS {
		var logSize C.size_t
		C.clGetProgramBuildInfo(g.program, g.device, C.CL_PROGRAM_BUILD_LOG, 0, nil, &logSize)
		buildLog := make([]byte, logSize)
		C.clGetProgramBuildInfo(g.program, g.device, C.CL_PROGRAM_BUILD_LOG, logSize, unsafe.Pointer(&buildLog[0]), nil)
		return fmt.Errorf("build failed: %s", string(buildLog))
	}

	kName := C.CString(GetSolanaKernelName())
	defer C.free(unsafe.Pointer(kName))
	g.kernel = C.clCreateKernel(g.program, kName, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("kernel creation failed: %d", ret)
	}

	return nil
}

func (g *SolanaGPUGenerator) createBuffers() error {
	var ret C.cl_int

	// 1. Seed buffer (32 bytes) - constant memory
	g.bufSeed = C.clCreateBuffer(g.clCtx, C.CL_MEM_READ_ONLY, 32, nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufSeed failed: %d", ret)
	}

	// 2. Output buffer (33 bytes: length + seed)
	g.bufOutput = C.clCreateBuffer(g.clCtx, C.CL_MEM_READ_WRITE, 33, nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufOutput failed: %d", ret)
	}

	// 3. Occupied bytes (1 byte)
	g.bufOccupiedBytes = C.clCreateBuffer(g.clCtx, C.CL_MEM_READ_ONLY, 1, nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufOccupiedBytes failed: %d", ret)
	}

	// 4. Group offset (1 byte)
	g.bufGroupOffset = C.clCreateBuffer(g.clCtx, C.CL_MEM_READ_ONLY, 1, nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufGroupOffset failed: %d", ret)
	}

	// 5. Prefix buffer (max 44 bytes for Solana address)
	g.bufPrefix = C.clCreateBuffer(g.clCtx, C.CL_MEM_READ_ONLY, 44, nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufPrefix failed: %d", ret)
	}

	// 6. Suffix buffer (max 44 bytes)
	g.bufSuffix = C.clCreateBuffer(g.clCtx, C.CL_MEM_READ_ONLY, 44, nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufSuffix failed: %d", ret)
	}

	// Upload prefix/suffix data to GPU
	prefixBytes := []byte(g.prefix)
	suffixBytes := []byte(g.suffix)

	if len(prefixBytes) > 0 {
		ret = C.clEnqueueWriteBuffer(g.queue, g.bufPrefix, C.CL_TRUE, 0,
			C.size_t(len(prefixBytes)), unsafe.Pointer(&prefixBytes[0]), 0, nil, nil)
		if ret != C.CL_SUCCESS {
			return fmt.Errorf("failed to write prefix: %d", ret)
		}
	}

	if len(suffixBytes) > 0 {
		ret = C.clEnqueueWriteBuffer(g.queue, g.bufSuffix, C.CL_TRUE, 0,
			C.size_t(len(suffixBytes)), unsafe.Pointer(&suffixBytes[0]), 0, nil, nil)
		if ret != C.CL_SUCCESS {
			return fmt.Errorf("failed to write suffix: %d", ret)
		}
	}

	// Set kernel arguments
	// Kernel signature: generate_pubkey(seed, out, occupied_bytes, group_offset,
	//                                   prefix, suffix, prefix_len, suffix_len, case_sensitive)
	C.clSetKernelArg(g.kernel, 0, C.size_t(unsafe.Sizeof(g.bufSeed)), unsafe.Pointer(&g.bufSeed))
	C.clSetKernelArg(g.kernel, 1, C.size_t(unsafe.Sizeof(g.bufOutput)), unsafe.Pointer(&g.bufOutput))
	C.clSetKernelArg(g.kernel, 2, C.size_t(unsafe.Sizeof(g.bufOccupiedBytes)), unsafe.Pointer(&g.bufOccupiedBytes))
	C.clSetKernelArg(g.kernel, 3, C.size_t(unsafe.Sizeof(g.bufGroupOffset)), unsafe.Pointer(&g.bufGroupOffset))
	C.clSetKernelArg(g.kernel, 4, C.size_t(unsafe.Sizeof(g.bufPrefix)), unsafe.Pointer(&g.bufPrefix))
	C.clSetKernelArg(g.kernel, 5, C.size_t(unsafe.Sizeof(g.bufSuffix)), unsafe.Pointer(&g.bufSuffix))

	// Pass lengths and case sensitivity as values
	prefixLen := C.uint(len(g.prefix))
	suffixLen := C.uint(len(g.suffix))
	caseSensitive := C.uint(1)
	if !g.caseSensitive {
		caseSensitive = C.uint(0)
	}

	C.clSetKernelArg(g.kernel, 6, C.size_t(unsafe.Sizeof(prefixLen)), unsafe.Pointer(&prefixLen))
	C.clSetKernelArg(g.kernel, 7, C.size_t(unsafe.Sizeof(suffixLen)), unsafe.Pointer(&suffixLen))
	C.clSetKernelArg(g.kernel, 8, C.size_t(unsafe.Sizeof(caseSensitive)), unsafe.Pointer(&caseSensitive))

	return nil
}

func (g *SolanaGPUGenerator) releaseBuffers() {
	if g.bufSeed != nil {
		C.clReleaseMemObject(g.bufSeed)
	}
	if g.bufOutput != nil {
		C.clReleaseMemObject(g.bufOutput)
	}
	if g.bufOccupiedBytes != nil {
		C.clReleaseMemObject(g.bufOccupiedBytes)
	}
	if g.bufGroupOffset != nil {
		C.clReleaseMemObject(g.bufGroupOffset)
	}
	if g.bufPrefix != nil {
		C.clReleaseMemObject(g.bufPrefix)
	}
	if g.bufSuffix != nil {
		C.clReleaseMemObject(g.bufSuffix)
	}
}

func (g *SolanaGPUGenerator) Release() {
	if g.kernel != nil {
		C.clReleaseKernel(g.kernel)
	}
	if g.program != nil {
		C.clReleaseProgram(g.program)
	}
	if g.queue != nil {
		C.clReleaseCommandQueue(g.queue)
	}
	if g.clCtx != nil {
		C.clReleaseContext(g.clCtx)
	}
}
