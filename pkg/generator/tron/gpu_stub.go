//go:build !opencl
// +build !opencl

package tron

import (
	"context"
	"fmt"

	"github.com/Amr-9/HexHunter/pkg/generator"
)

// TronGPUGenerator is a stub for non-OpenCL builds.
// Build with -tags opencl to enable GPU support.
type TronGPUGenerator struct{}

// NewTronGPUGenerator returns an error when OpenCL is not enabled.
func NewTronGPUGenerator() (*TronGPUGenerator, error) {
	return nil, fmt.Errorf("GPU support not compiled. Build with: go build -tags opencl")
}

// Name returns the generator name.
func (g *TronGPUGenerator) Name() string {
	return "GPU (Disabled)"
}

// Stats returns empty stats.
func (g *TronGPUGenerator) Stats() generator.Stats {
	return generator.Stats{}
}

// Start returns an error as GPU is not available.
func (g *TronGPUGenerator) Start(ctx context.Context, config *generator.Config) (<-chan generator.Result, error) {
	return nil, fmt.Errorf("GPU support not compiled")
}

// Release does nothing.
func (g *TronGPUGenerator) Release() {}
