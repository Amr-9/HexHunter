//go:build !opencl
// +build !opencl

package ethereum

import (
	"context"
	"fmt"

	"github.com/Amr-9/HexHunter/pkg/generator"
)

// GPUGenerator is a stub for non-OpenCL builds.
// Build with -tags opencl to enable GPU support.
type GPUGenerator struct{}

// GPUInfo contains information about an available GPU device
type GPUInfo struct {
	Name         string
	Vendor       string
	MaxWorkGroup int
	ComputeUnits int
	GlobalMem    uint64
}

// NewGPUGenerator returns an error when OpenCL is not enabled.
func NewGPUGenerator() (*GPUGenerator, error) {
	return nil, fmt.Errorf("GPU support not compiled. Build with: go build -tags opencl")
}

// Name returns the generator name.
func (g *GPUGenerator) Name() string {
	return "GPU (Disabled)"
}

// Stats returns empty stats.
func (g *GPUGenerator) Stats() generator.Stats {
	return generator.Stats{}
}

// Start returns an error as GPU is not available.
func (g *GPUGenerator) Start(ctx context.Context, config *generator.Config) (<-chan generator.Result, error) {
	return nil, fmt.Errorf("GPU support not compiled")
}

// Release does nothing.
func (g *GPUGenerator) Release() {}

// GetGPUInfo returns an error when OpenCL is not enabled.
func GetGPUInfo() ([]GPUInfo, error) {
	return nil, fmt.Errorf("GPU support not compiled. Build with: go build -tags opencl")
}

// IsGPUAvailable returns false when OpenCL is not compiled.
func IsGPUAvailable() bool {
	return false
}
