//go:build !opencl
// +build !opencl

package aptos

import (
	"context"
	"errors"

	"github.com/Amr-9/HexHunter/pkg/generator"
)

// AptosGPUGenerator is a stub for non-OpenCL builds.
type AptosGPUGenerator struct{}

// NewAptosGPUGenerator returns an error for non-OpenCL builds.
func NewAptosGPUGenerator() (*AptosGPUGenerator, error) {
	return nil, errors.New("GPU support requires OpenCL build tags")
}

// Name returns the implementation name.
func (g *AptosGPUGenerator) Name() string {
	return "GPU (Aptos - Not Available)"
}

// Stats returns empty stats for stub.
func (g *AptosGPUGenerator) Stats() generator.Stats {
	return generator.Stats{}
}

// Start is a stub that returns an error.
func (g *AptosGPUGenerator) Start(ctx context.Context, config *generator.Config) (<-chan generator.Result, error) {
	return nil, errors.New("GPU support requires OpenCL build tags")
}

// GPUInfo represents GPU device information.
type GPUInfo struct {
	Name         string
	ComputeUnits int
}

// GetGPUInfo returns empty GPU info for stub.
func GetGPUInfo() ([]GPUInfo, error) {
	return nil, errors.New("GPU support requires OpenCL build tags")
}
