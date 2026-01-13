//go:build opencl
// +build opencl

package generator

/*
#cgo CFLAGS: -I${SRCDIR}/../deps/opencl-headers
#cgo windows LDFLAGS: -L${SRCDIR}/../deps/lib -lOpenCL
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

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
)

//go:embed kernels/vanity_v4.cl
var kernelSourceV3 embed.FS

const (
	// Batch size = Table size (2^20 = 1,048,576)
	globalWorkSize = 1 << 20
	localWorkSize  = 256
	outputSize     = globalWorkSize * 20 // 20 bytes per address
	tableSize      = globalWorkSize * 64 // 64 bytes per point (Affine)
)

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
	bufOutput    C.cl_mem // Output addresses (20 MB)
	bufFlag      C.cl_mem // Found flag

	// secp256k1 curve for CPU calculations
	curve *secp256k1.BitCurve

	// stats
	attempts  uint64
	startTime time.Time

	// matching
	prefixBytes []byte
	suffixBytes []byte
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

func (g *GPUGenerator) Stats() Stats {
	attempts := atomic.LoadUint64(&g.attempts)
	elapsed := time.Since(g.startTime).Seconds()
	var hashRate float64
	if elapsed > 0 {
		hashRate = float64(attempts) / elapsed
	}
	return Stats{
		Attempts:    attempts,
		HashRate:    hashRate,
		ElapsedSecs: elapsed,
	}
}

func (g *GPUGenerator) Start(ctx context.Context, config *Config) (<-chan Result, error) {
	resultChan := make(chan Result, 1)
	g.startTime = time.Now()
	atomic.StoreUint64(&g.attempts, 0)

	if config.Prefix != "" {
		g.prefixBytes, _ = hex.DecodeString(padHex(config.Prefix))
	}
	if config.Suffix != "" {
		g.suffixBytes, _ = hex.DecodeString(padHex(config.Suffix))
	}

	go g.runGPU(ctx, resultChan, config)
	return resultChan, nil
}

func (g *GPUGenerator) runGPU(ctx context.Context, resultChan chan<- Result, config *Config) {
	// Create buffers
	if err := g.createBuffers(); err != nil {
		log.Printf("GPU Buffer Error: %v", err)
		return
	}
	defer g.releaseBuffers()

	// Host buffer for reading back results
	hostOutput := make([]byte, outputSize)

	// Generate random starting private key
	baseKeyBytes := make([]byte, 32)
	rand.Read(baseKeyBytes)
	baseInt := new(big.Int).SetBytes(baseKeyBytes)

	batchSizeInt := big.NewInt(globalWorkSize)
	var ret C.cl_int

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// 1. Compute BasePoint = base * G on CPU (in Jacobian form)
			basePointBytes := g.computeBasePointJacobian(baseInt)

			// 2. Upload BasePoint to GPU
			ret = C.clEnqueueWriteBuffer(g.queue, g.bufBasePoint, C.CL_TRUE, 0, 96,
				unsafe.Pointer(&basePointBytes[0]), 0, nil, nil)
			if ret != C.CL_SUCCESS {
				log.Printf("Failed to write base point: %d", ret)
				return
			}

			// 3. Run Kernel
			globalSize := C.size_t(globalWorkSize)
			localSize := C.size_t(localWorkSize)
			ret = C.clEnqueueNDRangeKernel(g.queue, g.kernel, 1, nil, &globalSize, &localSize, 0, nil, nil)
			if ret != C.CL_SUCCESS {
				log.Printf("Kernel execution failed: %d", ret)
				return
			}

			// 4. Read results back
			ret = C.clEnqueueReadBuffer(g.queue, g.bufOutput, C.CL_TRUE, 0, C.size_t(outputSize),
				unsafe.Pointer(&hostOutput[0]), 0, nil, nil)
			if ret != C.CL_SUCCESS {
				log.Printf("Read buffer failed: %d", ret)
				return
			}

			// 5. Check matches on CPU
			if found, res := g.checkBatch(hostOutput, baseInt, config); found {
				resultChan <- res
				return
			}

			// 6. Advance stats and base key
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

// checkBatch verifies addresses on CPU
func (g *GPUGenerator) checkBatch(output []byte, baseInt *big.Int, config *Config) (bool, Result) {
	prefixStr := config.Prefix
	suffixStr := config.Suffix

	for i := 0; i < globalWorkSize; i++ {
		offset := i * 20
		addrBytes := output[offset : offset+20]

		// Skip empty addresses (point at infinity)
		if i == 0 {
			allZero := true
			for _, b := range addrBytes {
				if b != 0 {
					allZero = false
					break
				}
			}
			if allZero {
				continue
			}
		}

		// Check Prefix and Suffix
		addrHex := hex.EncodeToString(addrBytes)
		if len(prefixStr) > 0 && len(addrHex) >= len(prefixStr) {
			if addrHex[:len(prefixStr)] != prefixStr {
				continue
			}
		}
		if len(suffixStr) > 0 && len(addrHex) >= len(suffixStr) {
			if addrHex[len(addrHex)-len(suffixStr):] != suffixStr {
				continue
			}
		}

		// Found! Reconstruct Private Key = base + i
		foundKey := new(big.Int).Add(baseInt, big.NewInt(int64(i)))
		privBytes := pad32(foundKey.Bytes())

		// Verify on CPU
		pk, _ := crypto.ToECDSA(privBytes)
		pub := crypto.PubkeyToAddress(pk.PublicKey)

		return true, Result{
			Address:    pub.Hex(),
			PrivateKey: hex.EncodeToString(privBytes),
		}
	}
	return false, Result{}
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

	// 3. Output (20 MB)
	g.bufOutput = C.clCreateBuffer(g.context, C.CL_MEM_WRITE_ONLY, C.size_t(outputSize), nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufOutput failed: %d", ret)
	}

	// 4. Flag (4 bytes)
	g.bufFlag = C.clCreateBuffer(g.context, C.CL_MEM_READ_WRITE, 4, nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufFlag failed: %d", ret)
	}

	// Set Kernel Args
	C.clSetKernelArg(g.kernel, 0, C.size_t(unsafe.Sizeof(g.bufBasePoint)), unsafe.Pointer(&g.bufBasePoint))
	C.clSetKernelArg(g.kernel, 1, C.size_t(unsafe.Sizeof(g.bufTable)), unsafe.Pointer(&g.bufTable))
	C.clSetKernelArg(g.kernel, 2, C.size_t(unsafe.Sizeof(g.bufOutput)), unsafe.Pointer(&g.bufOutput))
	C.clSetKernelArg(g.kernel, 3, C.size_t(unsafe.Sizeof(g.bufFlag)), unsafe.Pointer(&g.bufFlag))

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
