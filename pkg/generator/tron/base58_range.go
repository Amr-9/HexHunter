package tron

import (
	"strings"

	"github.com/mr-tron/base58"
)

// Tron address constants
const (
	base58MinChar  = '1' // Smallest Base58 character (value 0)
	base58MaxChar  = 'z' // Largest Base58 character (value 57)
	tronAddrLen    = 34  // Tron addresses are 34 characters (T + 33 chars)
	tronDataLen    = 25  // 21 bytes data + 4 bytes checksum = 25 bytes
	base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
)

// Base58Range represents the byte range for a Base58 prefix/suffix
type Base58Range struct {
	MinBytes   []byte // Lower bound (prefix + "111...1")
	MaxBytes   []byte // Upper bound (prefix + "zzz...z")
	PatternLen int    // Original pattern length
	Valid      bool   // Whether the range is valid for matching
}

// CalculateTronPrefixRange converts a Base58 prefix (after T) into a byte range for GPU comparison.
// Tron addresses format: T + 33 Base58 characters = 34 total
// Data: 0x41 (1 byte) + address (20 bytes) + checksum (4 bytes) = 25 bytes
//
// For prefix matching, we calculate the byte range of the raw 20-byte address
// that would produce the desired Base58 prefix when encoded.
func CalculateTronPrefixRange(prefix string) *Base58Range {
	if prefix == "" {
		return &Base58Range{
			MinBytes:   make([]byte, 20),
			MaxBytes:   make([]byte, 20),
			PatternLen: 0,
			Valid:      true,
		}
	}

	// Validate prefix contains only valid Base58 characters
	for _, c := range prefix {
		if !strings.ContainsRune(base58Alphabet, c) {
			return &Base58Range{Valid: false}
		}
	}

	// Full Tron address starts with 'T', we're matching after T
	// So full address is: T + prefix + padding
	fullPrefix := "T" + prefix

	// Calculate padding needed
	paddingLen := tronAddrLen - len(fullPrefix)
	if paddingLen < 0 {
		paddingLen = 0
	}

	// Create min string: T + prefix + "111...1"
	minStr := fullPrefix + strings.Repeat(string(base58MinChar), paddingLen)

	// Create max string: T + prefix + "zzz...z"
	maxStr := fullPrefix + strings.Repeat(string(base58MaxChar), paddingLen)

	// Decode from Base58 to get the raw bytes
	// This includes the 0x41 prefix and checksum
	minBytes, err := base58.Decode(minStr)
	if err != nil {
		return &Base58Range{Valid: false}
	}

	maxBytes, err := base58.Decode(maxStr)
	if err != nil {
		return &Base58Range{Valid: false}
	}

	// Extract just the 20-byte address portion (skip 0x41 prefix, ignore checksum)
	// minBytes format: [0x41][20 address bytes][4 checksum bytes]
	if len(minBytes) >= 21 {
		minBytes = minBytes[1:21] // Skip prefix byte, take 20 address bytes
	}
	if len(maxBytes) >= 21 {
		maxBytes = maxBytes[1:21]
	}

	// Pad to exactly 20 bytes
	minBytes = padTo20(minBytes)
	maxBytes = padTo20(maxBytes)

	return &Base58Range{
		MinBytes:   minBytes,
		MaxBytes:   maxBytes,
		PatternLen: len(prefix),
		Valid:      true,
	}
}

// CalculateTronSuffixRange calculates byte range for suffix matching.
// For suffix, we pad at the beginning: "111...1" + suffix up to 34 chars
func CalculateTronSuffixRange(suffix string) *Base58Range {
	if suffix == "" {
		return &Base58Range{
			MinBytes:   make([]byte, 20),
			MaxBytes:   make([]byte, 20),
			PatternLen: 0,
			Valid:      true,
		}
	}

	// Validate suffix
	for _, c := range suffix {
		if !strings.ContainsRune(base58Alphabet, c) {
			return &Base58Range{Valid: false}
		}
	}

	// For suffix, we create: T + padding + suffix
	paddingLen := tronAddrLen - 1 - len(suffix) // -1 for T
	if paddingLen < 0 {
		paddingLen = 0
	}

	// Min: T + "111...1" + suffix
	minStr := "T" + strings.Repeat(string(base58MinChar), paddingLen) + suffix

	// Max: T + "zzz...z" + suffix
	maxStr := "T" + strings.Repeat(string(base58MaxChar), paddingLen) + suffix

	minBytes, err := base58.Decode(minStr)
	if err != nil {
		return &Base58Range{Valid: false}
	}

	maxBytes, err := base58.Decode(maxStr)
	if err != nil {
		return &Base58Range{Valid: false}
	}

	// Extract 20-byte address portion
	if len(minBytes) >= 21 {
		minBytes = minBytes[1:21]
	}
	if len(maxBytes) >= 21 {
		maxBytes = maxBytes[1:21]
	}

	minBytes = padTo20(minBytes)
	maxBytes = padTo20(maxBytes)

	return &Base58Range{
		MinBytes:   minBytes,
		MaxBytes:   maxBytes,
		PatternLen: len(suffix),
		Valid:      true,
	}
}

// padTo20 pads or truncates byte slice to exactly 20 bytes (left-pad with zeros)
func padTo20(b []byte) []byte {
	if len(b) >= 20 {
		return b[:20]
	}
	result := make([]byte, 20)
	copy(result[20-len(b):], b)
	return result
}
