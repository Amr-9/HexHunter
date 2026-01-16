//go:build !opencl
// +build !opencl

package solana

import (
	"context"
	"fmt"

	"github.com/Amr-9/HexHunter/pkg/generator"
)

// SolanaGPUGenerator is a stub for non-OpenCL builds.
// Build with -tags opencl to enable GPU support.
type SolanaGPUGenerator struct{}

// NewSolanaGPUGenerator returns an error when OpenCL is not enabled.
func NewSolanaGPUGenerator() (*SolanaGPUGenerator, error) {
	return nil, fmt.Errorf("Solana GPU support not compiled. Build with: go build -tags opencl")
}

// Name returns the generator name.
func (g *SolanaGPUGenerator) Name() string {
	return "GPU Solana (Disabled)"
}

// Stats returns empty stats.
func (g *SolanaGPUGenerator) Stats() generator.Stats {
	return generator.Stats{}
}

// Start returns an error as GPU is not available.
func (g *SolanaGPUGenerator) Start(ctx context.Context, config *generator.Config) (<-chan generator.Result, error) {
	return nil, fmt.Errorf("Solana GPU support not compiled")
}

// Release does nothing.
func (g *SolanaGPUGenerator) Release() {}
