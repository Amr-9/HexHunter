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
	"bytes"
	"embed"
	"encoding/hex"
	"fmt"
	"math/big"
	"unsafe"

	"github.com/ethereum/go-ethereum/crypto"
)

//go:embed kernels/vanity_v2.cl
var kernelV2Source embed.FS

// GPUTestResult holds the result of GPU verification tests
type GPUTestResult struct {
	TestName     string
	PrivateKey   string
	GPUAddress   string
	CPUAddress   string
	Match        bool
	ErrorMessage string
}

// GPUVerifier tests the GPU implementation against CPU for correctness
type GPUVerifier struct {
	platform C.cl_platform_id
	device   C.cl_device_id
	context  C.cl_context
	queue    C.cl_command_queue
	program  C.cl_program
	kernel   C.cl_kernel
}

// NewGPUVerifier creates a new verifier instance
func NewGPUVerifier() (*GPUVerifier, error) {
	v := &GPUVerifier{}

	if err := v.init(); err != nil {
		return nil, err
	}

	return v, nil
}

// init initializes OpenCL
func (v *GPUVerifier) init() error {
	var ret C.cl_int

	// Get platform
	var numPlatforms C.cl_uint
	ret = C.clGetPlatformIDs(0, nil, &numPlatforms)
	if ret != C.CL_SUCCESS || numPlatforms == 0 {
		return fmt.Errorf("no OpenCL platforms found")
	}

	platforms := make([]C.cl_platform_id, numPlatforms)
	C.clGetPlatformIDs(numPlatforms, &platforms[0], nil)
	v.platform = platforms[0]

	// Get GPU device
	var numDevices C.cl_uint
	ret = C.clGetDeviceIDs(v.platform, C.CL_DEVICE_TYPE_GPU, 0, nil, &numDevices)
	if ret != C.CL_SUCCESS || numDevices == 0 {
		return fmt.Errorf("no OpenCL GPU devices found")
	}

	devices := make([]C.cl_device_id, numDevices)
	C.clGetDeviceIDs(v.platform, C.CL_DEVICE_TYPE_GPU, numDevices, &devices[0], nil)
	v.device = devices[0]

	// Create context
	v.context = C.clCreateContext(nil, 1, &v.device, nil, nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("failed to create OpenCL context")
	}

	// Create command queue
	v.queue = C.clCreateCommandQueue(v.context, v.device, 0, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("failed to create command queue")
	}

	// Load kernel source
	kernelData, err := kernelV2Source.ReadFile("kernels/vanity_v2.cl")
	if err != nil {
		return fmt.Errorf("failed to load kernel source: %w", err)
	}

	source := C.CString(string(kernelData))
	defer C.free(unsafe.Pointer(source))

	sourceLen := C.size_t(len(kernelData))
	v.program = C.clCreateProgramWithSource(v.context, 1, &source, &sourceLen, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("failed to create program")
	}

	// Build program
	ret = C.clBuildProgram(v.program, 1, &v.device, nil, nil, nil)
	if ret != C.CL_SUCCESS {
		// Get build log
		var logSize C.size_t
		C.clGetProgramBuildInfo(v.program, v.device, C.CL_PROGRAM_BUILD_LOG, 0, nil, &logSize)
		log := make([]byte, logSize)
		C.clGetProgramBuildInfo(v.program, v.device, C.CL_PROGRAM_BUILD_LOG, logSize, unsafe.Pointer(&log[0]), nil)
		return fmt.Errorf("failed to build program: %s", string(log))
	}

	// Create kernel
	kernelName := C.CString("compute_address")
	defer C.free(unsafe.Pointer(kernelName))
	v.kernel = C.clCreateKernel(v.program, kernelName, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("failed to create kernel")
	}

	return nil
}

// ComputeBatchGPU computes Ethereum addresses using GPU in batch
func (v *GPUVerifier) ComputeBatchGPU(baseKeyHex string, numThreads int) ([]byte, error) {
	var ret C.cl_int

	// Decode base private key
	baseKey, err := hex.DecodeString(baseKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key hex: %w", err)
	}
	if len(baseKey) != 32 {
		return nil, fmt.Errorf("private key must be 32 bytes")
	}

	// 1. Base Key Buffer (32 bytes input)
	bufBaseKey := C.clCreateBuffer(v.context, C.CL_MEM_READ_ONLY|C.CL_MEM_COPY_HOST_PTR, 32, unsafe.Pointer(&baseKey[0]), &ret)
	if ret != C.CL_SUCCESS {
		return nil, fmt.Errorf("failed to create base key buffer")
	}
	defer C.clReleaseMemObject(bufBaseKey)

	// 2. Output Buffer (numThreads * 20 bytes output)
	outSize := C.size_t(numThreads * 20)
	bufOutput := C.clCreateBuffer(v.context, C.CL_MEM_WRITE_ONLY, outSize, nil, &ret)
	if ret != C.CL_SUCCESS {
		return nil, fmt.Errorf("failed to create output buffer")
	}
	defer C.clReleaseMemObject(bufOutput)

	// 3. Flag Buffer (4 bytes, read/write - unused for now but required by kernel)
	bufFlag := C.clCreateBuffer(v.context, C.CL_MEM_READ_WRITE, 4, nil, &ret)
	if ret != C.CL_SUCCESS {
		return nil, fmt.Errorf("failed to create flag buffer")
	}
	defer C.clReleaseMemObject(bufFlag)

	// Set kernel arguments
	// __kernel void compute_address(__global const uchar *base, __global uchar *out, __global uint *flag)
	if status := C.clSetKernelArg(v.kernel, 0, C.size_t(unsafe.Sizeof(bufBaseKey)), unsafe.Pointer(&bufBaseKey)); status != C.CL_SUCCESS {
		return nil, fmt.Errorf("failed to set arg 0: %d", status)
	}
	if status := C.clSetKernelArg(v.kernel, 1, C.size_t(unsafe.Sizeof(bufOutput)), unsafe.Pointer(&bufOutput)); status != C.CL_SUCCESS {
		return nil, fmt.Errorf("failed to set arg 1: %d", status)
	}
	if status := C.clSetKernelArg(v.kernel, 2, C.size_t(unsafe.Sizeof(bufFlag)), unsafe.Pointer(&bufFlag)); status != C.CL_SUCCESS {
		return nil, fmt.Errorf("failed to set arg 2: %d", status)
	}

	// Execute kernel (Batch!)
	var globalSize C.size_t = C.size_t(numThreads)
	// OpenCL can decide local size, or we can pass NULL
	ret = C.clEnqueueNDRangeKernel(v.queue, v.kernel, 1, nil, &globalSize, nil, 0, nil, nil)
	if ret != C.CL_SUCCESS {
		return nil, fmt.Errorf("failed to enqueue kernel: %d", ret)
	}

	// Wait for completion
	C.clFinish(v.queue)

	// Read result
	output := make([]byte, int(outSize))
	ret = C.clEnqueueReadBuffer(v.queue, bufOutput, C.CL_TRUE, 0, outSize,
		unsafe.Pointer(&output[0]), 0, nil, nil)
	if ret != C.CL_SUCCESS {
		return nil, fmt.Errorf("failed to read output buffer")
	}

	return output, nil
}

// ComputeAddressCPU computes Ethereum address using CPU (go-ethereum)
func ComputeAddressCPU(privateKeyHex string) (string, error) {
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return "", fmt.Errorf("invalid private key: %w", err)
	}

	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	return address.Hex(), nil
}

// RunVerificationTests runs Phase 2 Batch Tests
func (v *GPUVerifier) RunVerificationTests() []GPUTestResult {
	// Base Key: ...0001
	baseKeyHex := "0000000000000000000000000000000000000000000000000000000000000001"
	numThreads := 1024

	fmt.Printf("\n🚀 Phase 2: Running Batch Verification (BaseKey=...01, Threads=%d)...\n", numThreads)

	// Run Batch on GPU
	gpuOutput, err := v.ComputeBatchGPU(baseKeyHex, numThreads)
	if err != nil {
		return []GPUTestResult{{TestName: "Batch Run", ErrorMessage: err.Error()}}
	}

	// Helper to get key at offset
	getKeyAt := func(offset int64) string {
		baseInt := new(big.Int)
		baseInt.SetString(baseKeyHex, 16)
		baseInt.Add(baseInt, big.NewInt(offset))
		k := hex.EncodeToString(baseInt.Bytes())
		// Pad to 64 chars
		for len(k) < 64 {
			k = "0" + k
		}
		return k
	}

	// Verify specific indices
	indicesToCheck := []struct {
		offset int
		name   string
	}{
		{0, "Thread 0 (Base + 0)"},
		{1, "Thread 1 (Base + 1)"},
		{2, "Thread 2 (Base + 2)"},
		{1000, "Thread 1000 (Base + 1000)"}, // Jump
		{1023, "Thread 1023 (Last)"},
	}

	var results []GPUTestResult

	for _, tc := range indicesToCheck {
		res := GPUTestResult{TestName: tc.name}

		// 1. Get Expected CPU Address
		privKey := getKeyAt(int64(tc.offset))
		res.PrivateKey = privKey
		cpuAddr, err := ComputeAddressCPU(privKey)
		if err != nil {
			res.ErrorMessage = fmt.Sprintf("CPU Error: %v", err)
			results = append(results, res)
			continue
		}
		res.CPUAddress = cpuAddr

		// 2. Get GPU Address from buffer
		start := tc.offset * 20
		end := start + 20
		if start >= len(gpuOutput) {
			res.ErrorMessage = "GPU Output Index Out of Bounds"
			results = append(results, res)
			continue
		}
		gpuAddrBytes := gpuOutput[start:end]
		gpuAddr := "0x" + hex.EncodeToString(gpuAddrBytes)
		res.GPUAddress = gpuAddr

		// 3. Compare
		if bytes.EqualFold([]byte(cpuAddr), []byte(gpuAddr)) {
			res.Match = true
		} else {
			res.Match = false
		}
		results = append(results, res)
	}

	return results
}

// Release cleans up resources
func (v *GPUVerifier) Release() {
	if v.kernel != nil {
		C.clReleaseKernel(v.kernel)
	}
	if v.program != nil {
		C.clReleaseProgram(v.program)
	}
	if v.queue != nil {
		C.clReleaseCommandQueue(v.queue)
	}
	if v.context != nil {
		C.clReleaseContext(v.context)
	}
}

// VerifyGPUImplementation is the main entry point for testing
func VerifyGPUImplementation() (bool, []GPUTestResult) {
	verifier, err := NewGPUVerifier()
	if err != nil {
		return false, []GPUTestResult{{
			TestName:     "Initialization",
			ErrorMessage: err.Error(),
		}}
	}
	defer verifier.Release()

	results := verifier.RunVerificationTests()

	allPassed := true
	for _, r := range results {
		if !r.Match || r.ErrorMessage != "" {
			allPassed = false
			break
		}
	}

	return allPassed, results
}
