// Package generator defines the interface for Ethereum vanity address generation.
// This design allows easy swapping between CPU and GPU implementations.
package generator

import (
	"context"
)

// Config holds the configuration for vanity address generation.
type Config struct {
	Prefix  string // Desired address prefix (without 0x)
	Suffix  string // Desired address suffix
	Workers int    // Number of concurrent workers
}

// Result contains a successfully found vanity address and its private key.
type Result struct {
	Address    string // Ethereum address (with 0x prefix)
	PrivateKey string // Private key in hex format
}

// Stats holds real-time performance statistics.
type Stats struct {
	Attempts    uint64  // Total number of addresses generated
	HashRate    float64 // Current hashes per second
	ElapsedSecs float64 // Time elapsed since start
}

// Generator defines the contract for address generation backends.
// Implementations can be CPU-based (goroutines) or GPU-based (CUDA/OpenCL).
type Generator interface {
	// Start begins the vanity address search with the given configuration.
	// It returns a channel that will receive the result when found.
	// The search can be cancelled via the context.
	Start(ctx context.Context, config *Config) (<-chan Result, error)

	// Stats returns the current performance statistics.
	// This method is safe to call concurrently from any goroutine.
	Stats() Stats

	// Name returns the implementation name (e.g., "CPU", "CUDA", "OpenCL").
	Name() string
}
