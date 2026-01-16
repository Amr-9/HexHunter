//go:build opencl
// +build opencl

package tron

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
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
)

//go:embed kernels/tron_kernel.cl
var tronKernelSource string

const (
	// Batch size = Table size (2^20 = 1,048,576)
	globalWorkSize = 1 << 20
	localWorkSize  = 256
	tableSize      = globalWorkSize * 64 // 64 bytes per point (Affine)
	// Output: gid(4) + address(34) + null(1) = 39 bytes
	outputBufferSize = 64
)

// TronGPUGenerator implements the Generator interface using OpenCL GPU acceleration.
// Uses optimized kernel with in-kernel Base58Check encoding and pattern matching.
type TronGPUGenerator struct {
	platform C.cl_platform_id
	device   C.cl_device_id
	context  C.cl_context
	queue    C.cl_command_queue
	program  C.cl_program
	kernel   C.cl_kernel

	// buffers
	bufBasePoint C.cl_mem // BasePoint (Jacobian, 96 bytes)
	bufTable     C.cl_mem // Precomputed table (64 MB)
	bufOutput    C.cl_mem // Output buffer (64 bytes)
	bufFlag      C.cl_mem // Found flag (4 bytes)
	bufPrefix    C.cl_mem // Prefix pattern
	bufSuffix    C.cl_mem // Suffix pattern

	// secp256k1 curve for CPU calculations
	curve *secp256k1.BitCurve

	// stats
	attempts  uint64
	startTime time.Time

	// Pattern config
	prefix string
	suffix string
}

// NewTronGPUGenerator creates a new GPU-based generator for Tron.
func NewTronGPUGenerator() (*TronGPUGenerator, error) {
	g := &TronGPUGenerator{
		curve: secp256k1.S256(),
	}
	if err := g.initOpenCL(); err != nil {
		return nil, fmt.Errorf("failed to initialize OpenCL: %w", err)
	}
	return g, nil
}

func (g *TronGPUGenerator) Name() string {
	return "GPU (OpenCL - Tron)"
}

func (g *TronGPUGenerator) Stats() generator.Stats {
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

func (g *TronGPUGenerator) Start(ctx context.Context, config *generator.Config) (<-chan generator.Result, error) {
	resultChan := make(chan generator.Result, 1)
	g.startTime = time.Now()
	atomic.StoreUint64(&g.attempts, 0)

	g.prefix = config.Prefix
	g.suffix = config.Suffix

	go g.runGPU(ctx, resultChan, config)
	return resultChan, nil
}

func (g *TronGPUGenerator) runGPU(ctx context.Context, resultChan chan<- generator.Result, config *generator.Config) {
	// Create buffers
	if err := g.createBuffers(); err != nil {
		log.Printf("GPU Buffer Error: %v", err)
		return
	}
	defer g.releaseBuffers()

	// Host buffer for reading result
	hostOutput := make([]byte, outputBufferSize)
	var foundFlag uint32

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
			// 1. Reset found_flag
			ret = C.clEnqueueWriteBuffer(g.queue, g.bufFlag, C.CL_TRUE, 0, 4,
				unsafe.Pointer(&zero), 0, nil, nil)
			if ret != C.CL_SUCCESS {
				log.Printf("Failed to reset flag: %d", ret)
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

			// 4. Run Kernel (computes addresses, does Base58, matches pattern)
			globalSize := C.size_t(globalWorkSize)
			localSize := C.size_t(localWorkSize)
			ret = C.clEnqueueNDRangeKernel(g.queue, g.kernel, 1, nil, &globalSize, &localSize, 0, nil, nil)
			if ret != C.CL_SUCCESS {
				log.Printf("Kernel execution failed: %d", ret)
				return
			}

			// 5. Read found flag
			ret = C.clEnqueueReadBuffer(g.queue, g.bufFlag, C.CL_TRUE, 0, 4,
				unsafe.Pointer(&foundFlag), 0, nil, nil)
			if ret != C.CL_SUCCESS {
				log.Printf("Read flag failed: %d", ret)
				return
			}

			// 6. If found, read result
			if foundFlag != 0 {
				ret = C.clEnqueueReadBuffer(g.queue, g.bufOutput, C.CL_TRUE, 0, C.size_t(outputBufferSize),
					unsafe.Pointer(&hostOutput[0]), 0, nil, nil)
				if ret != C.CL_SUCCESS {
					log.Printf("Read output failed: %d", ret)
					return
				}

				// Parse GID from output (big-endian)
				foundGid := uint32(hostOutput[0])<<24 | uint32(hostOutput[1])<<16 |
					uint32(hostOutput[2])<<8 | uint32(hostOutput[3])

				// Get address string (null-terminated)
				addrEnd := 4
				for addrEnd < outputBufferSize && hostOutput[addrEnd] != 0 {
					addrEnd++
				}
				foundAddress := string(hostOutput[4:addrEnd])

				// Reconstruct private key: base + gid
				privKey := new(big.Int).Add(baseInt, big.NewInt(int64(foundGid)))
				privBytes := pad32(privKey.Bytes())

				resultChan <- generator.Result{
					Address:    foundAddress,
					PrivateKey: hex.EncodeToString(privBytes),
					Network:    generator.Tron,
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
func (g *TronGPUGenerator) computeBasePointJacobian(base *big.Int) []byte {
	Px, Py := g.curve.ScalarBaseMult(base.Bytes())
	result := make([]byte, 96)

	xBytes := Px.Bytes()
	for i := 0; i < len(xBytes) && i < 32; i++ {
		result[i] = xBytes[len(xBytes)-1-i]
	}

	yBytes := Py.Bytes()
	for i := 0; i < len(yBytes) && i < 32; i++ {
		result[32+i] = yBytes[len(yBytes)-1-i]
	}

	result[64] = 1
	return result
}

func (g *TronGPUGenerator) initOpenCL() error {
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

	// Load Tron-specific kernel
	src := C.CString(tronKernelSource)
	defer C.free(unsafe.Pointer(src))

	length := C.size_t(len(tronKernelSource))
	g.program = C.clCreateProgramWithSource(g.context, 1, &src, &length, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("program creation failed: %d", ret)
	}

	ret = C.clBuildProgram(g.program, 1, &g.device, nil, nil, nil)
	if ret != C.CL_SUCCESS {
		var logSize C.size_t
		C.clGetProgramBuildInfo(g.program, g.device, C.CL_PROGRAM_BUILD_LOG, 0, nil, &logSize)
		buildLog := make([]byte, logSize)
		C.clGetProgramBuildInfo(g.program, g.device, C.CL_PROGRAM_BUILD_LOG, logSize, unsafe.Pointer(&buildLog[0]), nil)
		return fmt.Errorf("program build failed: %s", string(buildLog))
	}

	kName := C.CString("tron_generate_address")
	defer C.free(unsafe.Pointer(kName))
	g.kernel = C.clCreateKernel(g.program, kName, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("kernel creation failed: %d", ret)
	}

	return nil
}

func (g *TronGPUGenerator) createBuffers() error {
	var ret C.cl_int

	// 1. BasePoint (Jacobian, 96 bytes)
	g.bufBasePoint = C.clCreateBuffer(g.context, C.CL_MEM_READ_ONLY, 96, nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufBasePoint failed: %d", ret)
	}

	// 2. Precomputed Table
	tableData, err := g.loadTable()
	if err != nil {
		return fmt.Errorf("failed to load table: %w", err)
	}
	g.bufTable = C.clCreateBuffer(g.context, C.CL_MEM_READ_ONLY|C.CL_MEM_COPY_HOST_PTR,
		C.size_t(len(tableData)), unsafe.Pointer(&tableData[0]), &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufTable failed: %d", ret)
	}

	// 3. Output buffer
	g.bufOutput = C.clCreateBuffer(g.context, C.CL_MEM_WRITE_ONLY, C.size_t(outputBufferSize), nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufOutput failed: %d", ret)
	}

	// 4. Found flag
	g.bufFlag = C.clCreateBuffer(g.context, C.CL_MEM_READ_WRITE, 4, nil, &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufFlag failed: %d", ret)
	}

	// 5. Prefix pattern (pad to 44 bytes max)
	prefixData := make([]byte, 44)
	copy(prefixData, g.prefix)
	g.bufPrefix = C.clCreateBuffer(g.context, C.CL_MEM_READ_ONLY|C.CL_MEM_COPY_HOST_PTR,
		44, unsafe.Pointer(&prefixData[0]), &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufPrefix failed: %d", ret)
	}

	// 6. Suffix pattern
	suffixData := make([]byte, 44)
	copy(suffixData, g.suffix)
	g.bufSuffix = C.clCreateBuffer(g.context, C.CL_MEM_READ_ONLY|C.CL_MEM_COPY_HOST_PTR,
		44, unsafe.Pointer(&suffixData[0]), &ret)
	if ret != C.CL_SUCCESS {
		return fmt.Errorf("bufSuffix failed: %d", ret)
	}

	// Set Kernel Args
	prefixLen := C.uint(len(g.prefix))
	suffixLen := C.uint(len(g.suffix))

	C.clSetKernelArg(g.kernel, 0, C.size_t(unsafe.Sizeof(g.bufBasePoint)), unsafe.Pointer(&g.bufBasePoint))
	C.clSetKernelArg(g.kernel, 1, C.size_t(unsafe.Sizeof(g.bufTable)), unsafe.Pointer(&g.bufTable))
	C.clSetKernelArg(g.kernel, 2, C.size_t(unsafe.Sizeof(g.bufOutput)), unsafe.Pointer(&g.bufOutput))
	C.clSetKernelArg(g.kernel, 3, C.size_t(unsafe.Sizeof(g.bufFlag)), unsafe.Pointer(&g.bufFlag))
	C.clSetKernelArg(g.kernel, 4, C.size_t(unsafe.Sizeof(g.bufPrefix)), unsafe.Pointer(&g.bufPrefix))
	C.clSetKernelArg(g.kernel, 5, C.size_t(unsafe.Sizeof(g.bufSuffix)), unsafe.Pointer(&g.bufSuffix))
	C.clSetKernelArg(g.kernel, 6, C.size_t(unsafe.Sizeof(prefixLen)), unsafe.Pointer(&prefixLen))
	C.clSetKernelArg(g.kernel, 7, C.size_t(unsafe.Sizeof(suffixLen)), unsafe.Pointer(&suffixLen))

	return nil
}

func (g *TronGPUGenerator) loadTable() ([]byte, error) {
	data, err := os.ReadFile("tables.bin")
	if err != nil {
		if os.IsNotExist(err) {
			return generateTable()
		}
		return nil, fmt.Errorf("error reading tables.bin: %w", err)
	}
	if len(data) != tableSize {
		return nil, fmt.Errorf("invalid table size: got %d, expected %d", len(data), tableSize)
	}
	return data, nil
}

func generateTable() ([]byte, error) {
	curve := secp256k1.S256()
	table := make([]byte, tableSize)

	for i := 0; i < globalWorkSize; i++ {
		scalar := big.NewInt(int64(i + 1))
		px, py := curve.ScalarBaseMult(scalar.Bytes())

		offset := i * 64
		xBytes := px.Bytes()
		yBytes := py.Bytes()

		for j := 0; j < len(xBytes) && j < 32; j++ {
			table[offset+j] = xBytes[len(xBytes)-1-j]
		}
		for j := 0; j < len(yBytes) && j < 32; j++ {
			table[offset+32+j] = yBytes[len(yBytes)-1-j]
		}
	}

	if err := os.WriteFile("tables.bin", table, 0644); err != nil {
		return nil, fmt.Errorf("failed to save tables.bin: %w", err)
	}
	return table, nil
}

func (g *TronGPUGenerator) releaseBuffers() {
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
	if g.bufPrefix != nil {
		C.clReleaseMemObject(g.bufPrefix)
	}
	if g.bufSuffix != nil {
		C.clReleaseMemObject(g.bufSuffix)
	}
}

func (g *TronGPUGenerator) Release() {
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

func pad32(b []byte) []byte {
	if len(b) >= 32 {
		return b
	}
	res := make([]byte, 32)
	copy(res[32-len(b):], b)
	return res
}
