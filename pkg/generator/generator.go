// Package generator defines the interface for vanity address generation.
// This design allows easy swapping between CPU and GPU implementations,
// and supports multiple blockchain networks (Ethereum, Solana, Bitcoin).
package generator

import (
	"context"
)

// Network represents the blockchain network for address generation.
type Network int

const (
	Ethereum Network = iota // Ethereum (secp256k1, Keccak-256, Hex)
	Solana                  // Solana (Ed25519, Base58)
	Aptos                   // Aptos (Ed25519, SHA3-256, Hex)
	Sui                     // Sui (Ed25519, Blake2b-256, Hex)
	Bitcoin                 // Bitcoin (secp256k1, SHA256+RIPEMD160, Base58/Bech32)
	Tron                    // Tron (secp256k1, Keccak-256, Base58Check)
)

// String returns the network name.
func (n Network) String() string {
	switch n {
	case Ethereum:
		return "Ethereum"
	case Solana:
		return "Solana"
	case Aptos:
		return "Aptos"
	case Sui:
		return "Sui"
	case Bitcoin:
		return "Bitcoin"
	case Tron:
		return "Tron"
	default:
		return "Unknown"
	}
}

// AddressType represents the Bitcoin address format.
type AddressType int

const (
	AddressTypeDefault      AddressType = iota // Default for network (P2TR for Bitcoin)
	AddressTypeTaproot                         // P2TR - Taproot (bc1p...) - Recommended
	AddressTypeLegacy                          // P2PKH - Legacy (1...)
	AddressTypeNestedSegWit                    // P2SH-P2WPKH - Nested SegWit (3...)
)

// String returns the address type name.
func (a AddressType) String() string {
	switch a {
	case AddressTypeTaproot:
		return "Taproot (P2TR)"
	case AddressTypeLegacy:
		return "Legacy (P2PKH)"
	case AddressTypeNestedSegWit:
		return "Nested SegWit (P2SH)"
	default:
		return "Default"
	}
}

// Config holds the configuration for vanity address generation.
type Config struct {
	Network     Network     // Target network (Ethereum, Solana, Bitcoin)
	AddressType AddressType // Address type (for Bitcoin: P2TR, P2PKH, P2SH)
	Prefix      string      // Desired address prefix
	Suffix      string      // Desired address suffix
	Workers     int         // Number of concurrent workers
}

// Result contains a successfully found vanity address and its private key.
type Result struct {
	Network    Network // Network the address belongs to
	Address    string  // Formatted address (0x... for ETH, Base58 for SOL)
	PrivateKey string  // Private key (Hex for ETH, Base58 for SOL)
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
