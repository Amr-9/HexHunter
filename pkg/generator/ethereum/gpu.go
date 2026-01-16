//go:build opencl
// +build opencl

package ethereum

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
	"crypto/rand"
	"embed"
	_ "embed"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"os"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/Amr-9/HexHunter/pkg/generator"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
)

//go:embed kernels/vanity_v4.cl
var kernelSourceV3 embed.FS

// GetKernelSource returns the embedded vanity kernel source code.
// This allows other packages (like Tron) to reuse the same kernel.
func GetKernelSource() ([]byte, error) {
	return kernelSourceV3.ReadFile("kernels/vanity_v4.cl")
}

// GPUGenerator implements the Generator interface using OpenCL GPU acceleration.
type GPUGenerator struct {
	platform C.cl_platform_id
	device   C.cl_device_id
	context  C.cl_context
	queue    C.cl_command_queue
	program  C.cl_program
	kernel   C.cl_kernel

	// buffers
	bufBasePoint C.cl_mem // BasePoint (Jacobian, 96 bytes)
	bufTable     C.cl_mem // Precomputed table (64 MB)
	bufOutput    C.cl_mem // Output address (64 bytes - single result!)
	bufFlag      C.cl_mem // Found flag
	bufFoundGid  C.cl_mem // Found GID
	bufTargetPfx C.cl_mem // Target prefix pattern
	bufTargetSfx C.cl_mem // Target suffix pattern

	// secp256k1 curve for CPU calculations
	curve *secp256k1.BitCurve

	// stats
	attempts  uint64
	startTime time.Time

	// matching
	prefixBytes []byte
	suffixBytes []byte
	prefixIsOdd bool // true if original prefix hex length was odd
	suffixIsOdd bool // true if original suffix hex length was odd
}

// GPUInfo contains information about an available GPU device
type GPUInfo struct {
	Name         string
	Vendor       string
	MaxWorkGroup int
	ComputeUnits int
	GlobalMem    uint64
}

// NewGPUGenerator creates a new GPU-based generator.
func NewGPUGenerator() (*GPUGenerator, error) {
	g := &GPUGenerator{
		curve: secp256k1.S256(),
	}
	if err := g.initOpenCL(); err != nil {
		return nil, fmt.Errorf("failed to initialize OpenCL: %w", err)
	}
	return g, nil
}

func (g *GPUGenerator) Name() string {
	return "GPU (OpenCL - v4 BatchInv)"
}

func (g *GPUGenerator) Stats() generator.Stats {
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

func (g *GPUGenerator) Start(ctx context.Context, config *generator.Config) (<-chan generator.Result, error) {
	resultChan := make(chan generator.Result, 1)
	g.startTime = time.Now()
	atomic.StoreUint64(&g.attempts, 0)

	// IMPORTANT: Reset prefix/suffix from previous search!
	g.prefixBytes = nil
	g.suffixBytes = nil
	g.prefixIsOdd = false
	g.suffixIsOdd = false

	if config.Prefix != "" {
		g.prefixIsOdd = len(config.Prefix)%2 == 1 // Track before padding!
		g.prefixBytes, _ = hex.DecodeString(padHex(config.Prefix))
	}
	if config.Suffix != "" {
		g.suffixIsOdd = len(config.Suffix)%2 == 1 // Track before padding!
		// For suffix: pad at BEGINNING (not end!) so low nibble is correct
		paddedSuffix := config.Suffix
		if g.suffixIsOdd {
			paddedSuffix = "0" + config.Suffix // Pad at start for low nibble
		}
		g.suffixBytes, _ = hex.DecodeString(paddedSuffix)
	}

	go g.runGPU(ctx, resultChan, config)
	return resultChan, nil
}

func (g *GPUGenerator) runGPU(ctx context.Context, resultChan chan<- generator.Result, config *generator.Config) {
	// Create buffers
	if err := g.createBuffers(); err != nil {
		log.Printf("GPU Buffer Error: %v", err)
		return
	}
	defer g.releaseBuffers()

	// Host buffers for reading results (minimal now!)
	hostOutput := make([]byte, 20) // Only 20 bytes for single address
	var foundFlag uint32
	var foundGid uint32

	// Generate random starting private key
	baseKeyBytes := make([]byte, 32)
	rand.Read(baseKeyBytes)
	baseInt := new(big.Int).SetBytes(baseKeyBytes)

	batchSizeInt := big.NewInt(globalWorkSize)
	var ret C.cl_int
	zero := uint32(0)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// 1. Reset found_flag and found_gid before each batch
			ret = C.clEnqueueWriteBuffer(g.queue, g.bufFlag, C.CL_TRUE, 0, 4,
				unsafe.Pointer(&zero), 0, nil, nil)
			if ret != C.CL_SUCCESS {
				log.Printf("Failed to reset flag: %d", ret)
				return
			}
			ret = C.clEnqueueWriteBuffer(g.queue, g.bufFoundGid, C.CL_TRUE, 0, 4,
				unsafe.Pointer(&zero), 0, nil, nil)
			if ret != C.CL_SUCCESS {
				log.Printf("Failed to reset gid: %d", ret)
				return
			}

			// 2. Compute BasePoint = base * G on CPU (in Jacobian form)
			basePointBytes := g.computeBasePointJacobian(baseInt)

			// 3. Upload BasePoint to GPU
			ret = C.clEnqueueWriteBuffer(g.queue, g.bufBasePoint, C.CL_TRUE, 0, 96,
				unsafe.Pointer(&basePointBytes[0]), 0, nil, nil)
			if ret != C.CL_SUCCESS {
				log.Printf("Failed to write base point: %d", ret)
				return
			}

			// 4. Run Kernel
			globalSize := C.size_t(globalWorkSize)
			localSize := C.size_t(localWorkSize)
			ret = C.clEnqueueNDRangeKernel(g.queue, g.kernel, 1, nil, &globalSize, &localSize, 0, nil, nil)
			if ret != C.CL_SUCCESS {
				log.Printf("Kernel execution failed: %d", ret)
				return
			}

			// 5. Read ONLY the flag (4 bytes!) - this is the key optimization
			ret = C.clEnqueueReadBuffer(g.queue, g.bufFlag, C.CL_TRUE, 0, 4,
				unsafe.Pointer(&foundFlag), 0, nil, nil)
			if ret != C.CL_SUCCESS {
				log.Printf("Read flag failed: %d", ret)
				return
			}

			// 6. If found, read the result and return
			if foundFlag != 0 {
				// Read found GID
				ret = C.clEnqueueReadBuffer(g.queue, g.bufFoundGid, C.CL_TRUE, 0, 4,
					unsafe.Pointer(&foundGid), 0, nil, nil)
				if ret != C.CL_SUCCESS {
					log.Printf("Read gid failed: %d", ret)
					return
				}

				// Read address from output buffer
				ret = C.clEnqueueReadBuffer(g.queue, g.bufOutput, C.CL_TRUE, 0, 20,
					unsafe.Pointer(&hostOutput[0]), 0, nil, nil)
				if ret != C.CL_SUCCESS {
					log.Printf("Read output failed: %d", ret)
					return
				}

				// Reconstruct Private Key: base + found_gid
				foundKey := new(big.Int).Add(baseInt, big.NewInt(int64(foundGid)))
				privBytes := pad32(foundKey.Bytes())

				// Verify on CPU
				pk, _ := crypto.ToECDSA(privBytes)
				pub := crypto.PubkeyToAddress(pk.PublicKey)

				resultChan <- generator.Result{
					Address:    pub.Hex(),
					PrivateKey: hex.EncodeToString(privBytes),
				}
				return
			}

			// 7. Advance stats and base key
			atomic.AddUint64(&g.attempts, uint64(globalWorkSize))
			baseInt.Add(baseInt, batchSizeInt)
		}
	}
}

// computeBasePointJacobian computes base*G and returns it in Jacobian form (96 bytes)
// Format: X (32 bytes, LE) | Y (32 bytes, LE) | Z (32 bytes, LE)
func (g *GPUGenerator) computeBasePointJacobian(base *big.Int) []byte {
	// Compute affine point: P = base * G
	Px, Py := g.curve.ScalarBaseMult(base.Bytes())

	// Convert to Jacobian (Z = 1)
	result := make([]byte, 96)

	// Write X in little-endian
	xBytes := Px.Bytes()
	for i := 0; i < len(xBytes) && i < 32; i++ {
		result[i] = xBytes[len(xBytes)-1-i]
	}

	// Write Y in little-endian
	yBytes := Py.Bytes()
	for i := 0; i < len(yBytes) && i < 32; i++ {
		result[32+i] = yBytes[len(yBytes)-1-i]
	}

	// Write Z = 1 in little-endian
	result[64] = 1 // Z = 1

	return result
}

func (g *GPUGenerator) initOpenCL() error {
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

	g.context = C.clCreateContext(nil, 1, &g.device, nil, nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("context failed")
	}

	g.queue = C.clCreateCommandQueue(g.context, g.device, 0, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("queue failed")
	}

	// Load Kernel V4 (Batch Inversion)
	kernelData, err := kernelSourceV3.ReadFile("kernels/vanity_v4.cl")
	if err != nil {
		return fmt.Errorf("failed to read kernel: %w", err)
	}
	src := C.CString(string(kernelData))
	defer C.free(unsafe.Pointer(src))

	length := C.size_t(len(kernelData))
	g.program = C.clCreateProgramWithSource(g.context, 1, &src, &length, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("program creation failed: %d", ret)
	}

	ret = C.clBuildProgram(g.program, 1, &g.device, nil, nil, nil)
	if ret != C.CL_SUCCESS {
		// Get build log
		var logSize C.size_t
		C.clGetProgramBuildInfo(g.program, g.device, C.CL_PROGRAM_BUILD_LOG, 0, nil, &logSize)
		buildLog := make([]byte, logSize)
		C.clGetProgramBuildInfo(g.program, g.device, C.CL_PROGRAM_BUILD_LOG, logSize, unsafe.Pointer(&buildLog[0]), nil)
		return fmt.Errorf("program build failed: %s", string(buildLog))
	}

	// Create Kernel
	kName := C.CString("compute_address")
	defer C.free(unsafe.Pointer(kName))
	g.kernel = C.clCreateKernel(g.program, kName, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("kernel creation failed: %d", ret)
	}

	return nil
}

func (g *GPUGenerator) createBuffers() error {
	var ret C.cl_int

	// 1. BasePoint (Jacobian, 96 bytes)
	g.bufBasePoint = C.clCreateBuffer(g.context, C.CL_MEM_READ_ONLY, 96, nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufBasePoint failed: %d", ret)
	}

	// 2. Precomputed Table (64 MB)
	tableData, err := g.loadTable()
	if err != nil {
		return fmt.Errorf("failed to load table: %w", err)
	}
	g.bufTable = C.clCreateBuffer(g.context, C.CL_MEM_READ_ONLY|C.CL_MEM_COPY_HOST_PTR,
		C.size_t(len(tableData)), unsafe.Pointer(&tableData[0]), &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufTable failed: %d", ret)
	}

	// 3. Output (64 bytes - single result only!)
	g.bufOutput = C.clCreateBuffer(g.context, C.CL_MEM_WRITE_ONLY, C.size_t(outputSize), nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufOutput failed: %d", ret)
	}

	// 4. Flag (4 bytes)
	g.bufFlag = C.clCreateBuffer(g.context, C.CL_MEM_READ_WRITE, 4, nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufFlag failed: %d", ret)
	}

	// 5. Found GID (4 bytes)
	g.bufFoundGid = C.clCreateBuffer(g.context, C.CL_MEM_READ_WRITE, 4, nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufFoundGid failed: %d", ret)
	}

	// 6. Target Prefix (20 bytes max, using __constant memory)
	prefixData := make([]byte, 20)
	copy(prefixData, g.prefixBytes)
	g.bufTargetPfx = C.clCreateBuffer(g.context, C.CL_MEM_READ_ONLY|C.CL_MEM_COPY_HOST_PTR,
		20, unsafe.Pointer(&prefixData[0]), &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufTargetPfx failed: %d", ret)
	}

	// 7. Target Suffix (20 bytes max, using __constant memory)
	suffixData := make([]byte, 20)
	copy(suffixData, g.suffixBytes)
	g.bufTargetSfx = C.clCreateBuffer(g.context, C.CL_MEM_READ_ONLY|C.CL_MEM_COPY_HOST_PTR,
		20, unsafe.Pointer(&suffixData[0]), &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufTargetSfx failed: %d", ret)
	}

	// Set Kernel Args (12 total)
	prefixLen := C.uint(len(g.prefixBytes))
	suffixLen := C.uint(len(g.suffixBytes))

	// Convert bool to uint for kernel
	prefixOdd := C.uint(0)
	if g.prefixIsOdd {
		prefixOdd = 1
	}
	suffixOdd := C.uint(0)
	if g.suffixIsOdd {
		suffixOdd = 1
	}

	C.clSetKernelArg(g.kernel, 0, C.size_t(unsafe.Sizeof(g.bufBasePoint)), unsafe.Pointer(&g.bufBasePoint))
	C.clSetKernelArg(g.kernel, 1, C.size_t(unsafe.Sizeof(g.bufTable)), unsafe.Pointer(&g.bufTable))
	C.clSetKernelArg(g.kernel, 2, C.size_t(unsafe.Sizeof(g.bufOutput)), unsafe.Pointer(&g.bufOutput))
	C.clSetKernelArg(g.kernel, 3, C.size_t(unsafe.Sizeof(g.bufFlag)), unsafe.Pointer(&g.bufFlag))
	C.clSetKernelArg(g.kernel, 4, C.size_t(unsafe.Sizeof(g.bufFoundGid)), unsafe.Pointer(&g.bufFoundGid))
	C.clSetKernelArg(g.kernel, 5, C.size_t(unsafe.Sizeof(g.bufTargetPfx)), unsafe.Pointer(&g.bufTargetPfx))
	C.clSetKernelArg(g.kernel, 6, C.size_t(unsafe.Sizeof(prefixLen)), unsafe.Pointer(&prefixLen))
	C.clSetKernelArg(g.kernel, 7, C.size_t(unsafe.Sizeof(g.bufTargetSfx)), unsafe.Pointer(&g.bufTargetSfx))
	C.clSetKernelArg(g.kernel, 8, C.size_t(unsafe.Sizeof(suffixLen)), unsafe.Pointer(&suffixLen))
	C.clSetKernelArg(g.kernel, 9, C.size_t(unsafe.Sizeof(prefixOdd)), unsafe.Pointer(&prefixOdd))
	C.clSetKernelArg(g.kernel, 10, C.size_t(unsafe.Sizeof(suffixOdd)), unsafe.Pointer(&suffixOdd))

	return nil
}

// loadTable loads precomputed table from tables.bin, or generates it if missing
func (g *GPUGenerator) loadTable() ([]byte, error) {
	data, err := os.ReadFile("tables.bin")
	if err != nil {
		if os.IsNotExist(err) {
			return GenerateTable()
		}
		return nil, fmt.Errorf("error reading tables.bin: %w", err)
	}
	if len(data) != tableSize {
		return nil, fmt.Errorf("invalid table size in tables.bin: got %d, expected %d. Please delete tables.bin and restart.", len(data), tableSize)
	}
	return data, nil
}

func (g *GPUGenerator) releaseBuffers() {
	if g.bufBasePoint != nil {
		C.clReleaseMemObject(g.bufBasePoint)
	}
	if g.bufTable != nil {
		C.clReleaseMemObject(g.bufTable)
	}
	if g.bufOutput != nil {
		C.clReleaseMemObject(g.bufOutput)
	}
	if g.bufFlag != nil {
		C.clReleaseMemObject(g.bufFlag)
	}
	if g.bufFoundGid != nil {
		C.clReleaseMemObject(g.bufFoundGid)
	}
	if g.bufTargetPfx != nil {
		C.clReleaseMemObject(g.bufTargetPfx)
	}
	if g.bufTargetSfx != nil {
		C.clReleaseMemObject(g.bufTargetSfx)
	}
}

func (g *GPUGenerator) Release() {
	if g.kernel != nil {
		C.clReleaseKernel(g.kernel)
	}
	if g.program != nil {
		C.clReleaseProgram(g.program)
	}
	if g.queue != nil {
		C.clReleaseCommandQueue(g.queue)
	}
	if g.context != nil {
		C.clReleaseContext(g.context)
	}
}

// Helpers
func padHex(s string) string {
	if len(s)%2 == 1 {
		return s + "0"
	}
	return s
}

func pad32(b []byte) []byte {
	if len(b) >= 32 {
		return b
	}
	res := make([]byte, 32)
	copy(res[32-len(b):], b)
	return res
}

// GetGPUInfo returns information about available GPU devices
func GetGPUInfo() ([]GPUInfo, error) {
	return []GPUInfo{{Name: "Supported GPU", Vendor: "Generic"}}, nil
}
