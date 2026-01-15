package solana

import (
	"strings"

	"github.com/mr-tron/base58"
)

// Base58 character constants (using alphabet from matcher_solana.go)
const (
	base58MinChar = '1' // Smallest Base58 character (value 0)
	base58MaxChar = 'z' // Largest Base58 character (value 57)
	solanaAddrLen = 44  // Maximum Solana address length
)

// Base58Range represents the byte range for a Base58 prefix
type Base58Range struct {
	MinBytes  []byte // Lower bound (prefix + "111...1")
	MaxBytes  []byte // Upper bound (prefix + "zzz...z")
	PrefixLen int    // Original prefix length
}

// CalculateBase58Range converts a Base58 prefix into a byte range for GPU comparison.
// This avoids expensive Base58 encoding on GPU by using range checking instead.
//
// Example:
//
//	prefix = "Amr"
//	MinBytes = decode("Amr11111111111111111111111111111111111111111")
//	MaxBytes = decode("Amrzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz")
//
// GPU then checks: MinBytes <= pubKey <= MaxBytes
func CalculateBase58Range(prefix string) (*Base58Range, error) {
	if prefix == "" {
		return &Base58Range{
			MinBytes:  make([]byte, 32),
			MaxBytes:  make([]byte, 32),
			PrefixLen: 0,
		}, nil
	}

	// Validate prefix contains only valid Base58 characters
	for _, c := range prefix {
		if !strings.ContainsRune(base58Alphabet, c) {
			return nil, &InvalidBase58Error{Char: c}
		}
	}

	// Calculate padding needed
	paddingLen := solanaAddrLen - len(prefix)
	if paddingLen < 0 {
		paddingLen = 0
	}

	// Create min string: prefix + "111...1"
	minStr := prefix + strings.Repeat(string(base58MinChar), paddingLen)

	// Create max string: prefix + "zzz...z"
	maxStr := prefix + strings.Repeat(string(base58MaxChar), paddingLen)

	// Decode to bytes
	minBytes, err := base58.Decode(minStr)
	if err != nil {
		return nil, err
	}

	maxBytes, err := base58.Decode(maxStr)
	if err != nil {
		return nil, err
	}

	// Pad to 32 bytes (big-endian - kernel handles endianness)
	minBytes = padTo32(minBytes)
	maxBytes = padTo32(maxBytes)

	return &Base58Range{
		MinBytes:  minBytes,
		MaxBytes:  maxBytes,
		PrefixLen: len(prefix),
	}, nil
}

// CalculateBase58SuffixRange calculates byte range for suffix matching.
// This is trickier because suffix affects the end of the byte array.
func CalculateBase58SuffixRange(suffix string) (*Base58Range, error) {
	if suffix == "" {
		return &Base58Range{
			MinBytes:  make([]byte, 32),
			MaxBytes:  make([]byte, 32),
			PrefixLen: 0,
		}, nil
	}

	// Validate suffix
	for _, c := range suffix {
		if !strings.ContainsRune(base58Alphabet, c) {
			return nil, &InvalidBase58Error{Char: c}
		}
	}

	// For suffix, we pad at the beginning
	paddingLen := solanaAddrLen - len(suffix)
	if paddingLen < 0 {
		paddingLen = 0
	}

	// Min: "111...1" + suffix
	minStr := strings.Repeat(string(base58MinChar), paddingLen) + suffix

	// Max: "zzz...z" + suffix
	maxStr := strings.Repeat(string(base58MaxChar), paddingLen) + suffix

	minBytes, err := base58.Decode(minStr)
	if err != nil {
		return nil, err
	}

	maxBytes, err := base58.Decode(maxStr)
	if err != nil {
		return nil, err
	}

	minBytes = padTo32(minBytes)
	maxBytes = padTo32(maxBytes)

	return &Base58Range{
		MinBytes:  minBytes,
		MaxBytes:  maxBytes,
		PrefixLen: len(suffix),
	}, nil
}

// padTo32 pads or truncates byte slice to exactly 32 bytes (big-endian, left-pad with zeros)
func padTo32(b []byte) []byte {
	if len(b) >= 32 {
		return b[:32]
	}
	result := make([]byte, 32)
	copy(result[32-len(b):], b)
	return result
}

// reverseBytesInPlace reverses a byte slice in place
func reverseBytesInPlace(b []byte) {
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
}

// padAndReverse pads to 32 bytes then reverses for little-endian comparison
// Ed25519 public keys are little-endian, but Base58 decoding produces big-endian
func padAndReverse(b []byte) []byte {
	result := padTo32(b)
	reverseBytesInPlace(result)
	return result
}

// InvalidBase58Error represents an invalid Base58 character error
type InvalidBase58Error struct {
	Char rune
}

func (e *InvalidBase58Error) Error() string {
	return "invalid Base58 character: " + string(e.Char)
}


