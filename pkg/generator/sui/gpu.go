//go:build opencl
// +build opencl

package sui

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
	"encoding/hex"
	"fmt"
	"log"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/Amr-9/HexHunter/pkg/generator"
)

const (
	// Batch size for Sui GPU kernel
	suiBatchSize     = 1 << 20 // 1,048,576 keys per batch
	suiLocalWorkSize = 256
)

// SuiGPUGenerator implements the Generator interface for Sui using OpenCL.
// Uses Ed25519 for keypair generation and Blake2b-256 for address derivation.
type SuiGPUGenerator struct {
	platform C.cl_platform_id
	device   C.cl_device_id
	clCtx    C.cl_context
	queue    C.cl_command_queue
	program  C.cl_program
	kernel   C.cl_kernel

	// buffers
	bufSeed          C.cl_mem
	bufOutput        C.cl_mem
	bufOccupiedBytes C.cl_mem
	bufGroupOffset   C.cl_mem
	bufPrefix        C.cl_mem
	bufSuffix        C.cl_mem

	// stats
	attempts  uint64
	startTime time.Time

	// matching config
	prefix   string
	suffix   string
	contains string
	matcher  *SuiMatcher
}

// NewSuiGPUGenerator creates a new Sui GPU-based generator.
func NewSuiGPUGenerator() (*SuiGPUGenerator, error) {
	g := &SuiGPUGenerator{}
	return g, nil
}

func (g *SuiGPUGenerator) Name() string {
	return "GPU (Sui Blake2b-256)"
}

func (g *SuiGPUGenerator) Stats() generator.Stats {
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

func (g *SuiGPUGenerator) Start(ctx context.Context, config *generator.Config) (<-chan generator.Result, error) {
	resultChan := make(chan generator.Result, 1)
	g.startTime = time.Now()
	atomic.StoreUint64(&g.attempts, 0)

	g.prefix = config.Prefix
	g.suffix = config.Suffix
	g.contains = config.Contains
	g.matcher = NewSuiMatcher(config.Prefix, config.Suffix, config.Contains)

	if err := g.initOpenCL(); err != nil {
		return nil, fmt.Errorf("failed to initialize OpenCL: %w", err)
	}

	go g.runGPU(ctx, resultChan)
	return resultChan, nil
}

func (g *SuiGPUGenerator) runGPU(ctx context.Context, resultChan chan<- generator.Result) {
	if err := g.createBuffers(); err != nil {
		log.Printf("Buffer creation failed: %v", err)
		return
	}
	defer g.releaseBuffers()

	baseSeed := make([]byte, 32)
	hostOutput := make([]byte, 33)
	zeros := make([]byte, 33)

	var occupiedBytes byte = 3
	var groupOffset byte = 0
	var ret C.cl_int

	for {
		select {
		case <-ctx.Done():
			return
		default:
			rand.Read(baseSeed)

			ret = C.clEnqueueWriteBuffer(g.queue, g.bufSeed, C.CL_TRUE, 0, 32,
				unsafe.Pointer(&baseSeed[0]), 0, nil, nil)
			if ret != C.CL_SUCCESS {
				log.Printf("Failed to write seed: %d", ret)
				return
			}

			ret = C.clEnqueueWriteBuffer(g.queue, g.bufOccupiedBytes, C.CL_TRUE, 0, 1,
				unsafe.Pointer(&occupiedBytes), 0, nil, nil)
			if ret != C.CL_SUCCESS {
				log.Printf("Failed to write occupied_bytes: %d", ret)
				return
			}

			ret = C.clEnqueueWriteBuffer(g.queue, g.bufGroupOffset, C.CL_TRUE, 0, 1,
				unsafe.Pointer(&groupOffset), 0, nil, nil)
			if ret != C.CL_SUCCESS {
				log.Printf("Failed to write group_offset: %d", ret)
				return
			}

			ret = C.clEnqueueWriteBuffer(g.queue, g.bufOutput, C.CL_TRUE, 0, 33,
				unsafe.Pointer(&zeros[0]), 0, nil, nil)
			if ret != C.CL_SUCCESS {
				log.Printf("Failed to clear output: %d", ret)
				return
			}

			globalSize := C.size_t(suiBatchSize)
			localSize := C.size_t(suiLocalWorkSize)
			ret = C.clEnqueueNDRangeKernel(g.queue, g.kernel, 1, nil,
				&globalSize, &localSize, 0, nil, nil)
			if ret != C.CL_SUCCESS {
				log.Printf("Kernel failed: %d", ret)
				return
			}

			ret = C.clEnqueueReadBuffer(g.queue, g.bufOutput, C.CL_TRUE, 0, 33,
				unsafe.Pointer(&hostOutput[0]), 0, nil, nil)
			if ret != C.CL_SUCCESS {
				log.Printf("Read output failed: %d", ret)
				return
			}

			// Check if kernel found a match
			if hostOutput[0] != 0 {
				foundSeed := hostOutput[1:33]

				// Verify with Go's implementation
				privKey := ed25519.NewKeyFromSeed(foundSeed)
				pubKey := privKey.Public().(ed25519.PublicKey)
				address := DeriveAddress(pubKey)

				if g.matcher.Matches(address) {
					resultChan <- generator.Result{
						Network:    generator.Sui,
						Address:    address,
						PrivateKey: hex.EncodeToString(foundSeed),
					}
					return
				}
				log.Printf("GPU false positive: address=%s", address)
			}

			atomic.AddUint64(&g.attempts, uint64(suiBatchSize))

			groupOffset++
			if groupOffset == 0 {
				groupOffset = 0
			}
		}
	}
}

func (g *SuiGPUGenerator) initOpenCL() error {
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

	// Load Sui kernel with Blake2b-256 for address derivation
	kernelSrc, err := LoadSuiKernel()
	if err != nil {
		return fmt.Errorf("failed to load Sui kernel source: %w", err)
	}

	src := C.CString(kernelSrc)
	defer C.free(unsafe.Pointer(src))

	length := C.size_t(len(kernelSrc))
	g.program = C.clCreateProgramWithSource(g.clCtx, 1, &src, &length, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("program creation failed: %d", ret)
	}

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

	kName := C.CString(GetSuiKernelName())
	defer C.free(unsafe.Pointer(kName))
	g.kernel = C.clCreateKernel(g.program, kName, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("kernel creation failed: %d", ret)
	}

	return nil
}

func (g *SuiGPUGenerator) createBuffers() error {
	var ret C.cl_int

	g.bufSeed = C.clCreateBuffer(g.clCtx, C.CL_MEM_READ_ONLY, 32, nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufSeed failed: %d", ret)
	}

	g.bufOutput = C.clCreateBuffer(g.clCtx, C.CL_MEM_READ_WRITE, 33, nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufOutput failed: %d", ret)
	}

	g.bufOccupiedBytes = C.clCreateBuffer(g.clCtx, C.CL_MEM_READ_ONLY, 1, nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufOccupiedBytes failed: %d", ret)
	}

	g.bufGroupOffset = C.clCreateBuffer(g.clCtx, C.CL_MEM_READ_ONLY, 1, nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufGroupOffset failed: %d", ret)
	}

	g.bufPrefix = C.clCreateBuffer(g.clCtx, C.CL_MEM_READ_ONLY, 64, nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufPrefix failed: %d", ret)
	}

	g.bufSuffix = C.clCreateBuffer(g.clCtx, C.CL_MEM_READ_ONLY, 64, nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufSuffix failed: %d", ret)
	}

	// 7. Contains buffer (64 bytes max for hex address)
	var bufContains C.cl_mem
	bufContains = C.clCreateBuffer(g.clCtx, C.CL_MEM_READ_ONLY, 64, nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufContains failed: %d", ret)
	}

	// Write prefix/suffix/contains to GPU
	prefixBytes := []byte(g.prefix)
	suffixBytes := []byte(g.suffix)
	containsBytes := []byte(g.contains)

	if len(prefixBytes) > 0 {
		C.clEnqueueWriteBuffer(g.queue, g.bufPrefix, C.CL_TRUE, 0,
			C.size_t(len(prefixBytes)), unsafe.Pointer(&prefixBytes[0]), 0, nil, nil)
	}

	if len(suffixBytes) > 0 {
		C.clEnqueueWriteBuffer(g.queue, g.bufSuffix, C.CL_TRUE, 0,
			C.size_t(len(suffixBytes)), unsafe.Pointer(&suffixBytes[0]), 0, nil, nil)
	}

	if len(containsBytes) > 0 {
		C.clEnqueueWriteBuffer(g.queue, bufContains, C.CL_TRUE, 0,
			C.size_t(len(containsBytes)), unsafe.Pointer(&containsBytes[0]), 0, nil, nil)
	}

	C.clSetKernelArg(g.kernel, 0, C.size_t(unsafe.Sizeof(g.bufSeed)), unsafe.Pointer(&g.bufSeed))
	C.clSetKernelArg(g.kernel, 1, C.size_t(unsafe.Sizeof(g.bufOutput)), unsafe.Pointer(&g.bufOutput))
	C.clSetKernelArg(g.kernel, 2, C.size_t(unsafe.Sizeof(g.bufOccupiedBytes)), unsafe.Pointer(&g.bufOccupiedBytes))
	C.clSetKernelArg(g.kernel, 3, C.size_t(unsafe.Sizeof(g.bufGroupOffset)), unsafe.Pointer(&g.bufGroupOffset))
	C.clSetKernelArg(g.kernel, 4, C.size_t(unsafe.Sizeof(g.bufPrefix)), unsafe.Pointer(&g.bufPrefix))
	C.clSetKernelArg(g.kernel, 5, C.size_t(unsafe.Sizeof(g.bufSuffix)), unsafe.Pointer(&g.bufSuffix))
	C.clSetKernelArg(g.kernel, 6, C.size_t(unsafe.Sizeof(bufContains)), unsafe.Pointer(&bufContains))

	prefixLen := C.uint(len(g.prefix))
	suffixLen := C.uint(len(g.suffix))
	containsLen := C.uint(len(g.contains))
	caseSensitive := C.uint(0) // Hex is case-insensitive

	C.clSetKernelArg(g.kernel, 7, C.size_t(unsafe.Sizeof(prefixLen)), unsafe.Pointer(&prefixLen))
	C.clSetKernelArg(g.kernel, 8, C.size_t(unsafe.Sizeof(suffixLen)), unsafe.Pointer(&suffixLen))
	C.clSetKernelArg(g.kernel, 9, C.size_t(unsafe.Sizeof(containsLen)), unsafe.Pointer(&containsLen))
	C.clSetKernelArg(g.kernel, 10, C.size_t(unsafe.Sizeof(caseSensitive)), unsafe.Pointer(&caseSensitive))

	return nil
}

func (g *SuiGPUGenerator) releaseBuffers() {
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

func (g *SuiGPUGenerator) Release() {
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

// GPUInfo represents GPU device information
type GPUInfo struct {
	Name         string
	ComputeUnits int
}

// GetGPUInfo returns information about available GPU devices
func GetGPUInfo() ([]GPUInfo, error) {
	var ret C.cl_int
	var numPlatforms C.cl_uint

	if C.clGetPlatformIDs(0, nil, &numPlatforms) != C.CL_SUCCESS || numPlatforms == 0 {
		return nil, fmt.Errorf("no OpenCL platforms available")
	}

	platforms := make([]C.cl_platform_id, numPlatforms)
	C.clGetPlatformIDs(numPlatforms, &platforms[0], nil)

	var gpus []GPUInfo

	for i := C.cl_uint(0); i < numPlatforms; i++ {
		var numDevices C.cl_uint
		ret = C.clGetDeviceIDs(platforms[i], C.CL_DEVICE_TYPE_GPU, 0, nil, &numDevices)
		if ret != C.CL_SUCCESS || numDevices == 0 {
			continue
		}

		devices := make([]C.cl_device_id, numDevices)
		C.clGetDeviceIDs(platforms[i], C.CL_DEVICE_TYPE_GPU, numDevices, &devices[0], nil)

		for j := C.cl_uint(0); j < numDevices; j++ {
			var nameSize C.size_t
			C.clGetDeviceInfo(devices[j], C.CL_DEVICE_NAME, 0, nil, &nameSize)
			nameBuf := make([]byte, nameSize)
			C.clGetDeviceInfo(devices[j], C.CL_DEVICE_NAME, nameSize, unsafe.Pointer(&nameBuf[0]), nil)

			var computeUnits C.cl_uint
			C.clGetDeviceInfo(devices[j], C.CL_DEVICE_MAX_COMPUTE_UNITS, C.size_t(unsafe.Sizeof(computeUnits)), unsafe.Pointer(&computeUnits), nil)

			gpus = append(gpus, GPUInfo{
				Name:         string(nameBuf[:len(nameBuf)-1]),
				ComputeUnits: int(computeUnits),
			})
		}
	}

	if len(gpus) == 0 {
		return nil, fmt.Errorf("no GPU devices found")
	}

	return gpus, nil
}
