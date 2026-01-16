//go:build !opencl
// +build !opencl

package sui

import (
	"context"
	"fmt"

	"github.com/Amr-9/HexHunter/pkg/generator"
)

// SuiGPUGenerator is a stub for non-OpenCL builds
type SuiGPUGenerator struct{}

// NewSuiGPUGenerator returns an error on non-OpenCL builds
func NewSuiGPUGenerator() (*SuiGPUGenerator, error) {
	return nil, fmt.Errorf("GPU support not available: build with -tags opencl")
}

func (g *SuiGPUGenerator) Name() string {
	return "GPU (Sui Blake2b-256)"
}

func (g *SuiGPUGenerator) Stats() generator.Stats {
	return generator.Stats{}
}

func (g *SuiGPUGenerator) Start(ctx context.Context, config *generator.Config) (<-chan generator.Result, error) {
	return nil, fmt.Errorf("GPU support not available: build with -tags opencl")
}

func (g *SuiGPUGenerator) Release() {}

// GPUInfo represents GPU device information
type GPUInfo struct {
	Name         string
	ComputeUnits int
}

// GetGPUInfo returns an error on non-OpenCL builds
func GetGPUInfo() ([]GPUInfo, error) {
	return nil, fmt.Errorf("GPU support not available: build with -tags opencl")
}
