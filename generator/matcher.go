package generator

import (
	"encoding/hex"
	"strings"
)

// Matcher provides optimized prefix/suffix matching for Ethereum addresses.
// It pre-processes the search patterns to avoid string allocations in the hot loop.
type Matcher struct {
	prefixBytes []byte // Pre-converted prefix as lowercase hex bytes
	suffixBytes []byte // Pre-converted suffix as lowercase hex bytes
	hasPrefix   bool
	hasSuffix   bool
}

// NewMatcher creates a new Matcher with the given prefix and suffix.
// Both are converted to lowercase bytes once, avoiding per-iteration allocations.
func NewMatcher(prefix, suffix string) *Matcher {
	m := &Matcher{}

	// Pre-process prefix (remove 0x if present, convert to lowercase hex bytes)
	prefix = strings.TrimPrefix(strings.ToLower(prefix), "0x")
	if prefix != "" {
		m.prefixBytes = []byte(prefix)
		m.hasPrefix = true
	}

	// Pre-process suffix (convert to lowercase hex bytes)
	suffix = strings.ToLower(suffix)
	if suffix != "" {
		m.suffixBytes = []byte(suffix)
		m.hasSuffix = true
	}

	return m
}

// Matches checks if the given address bytes match the prefix and/or suffix.
// The address should be the raw 20-byte Ethereum address (not hex encoded).
// This method is optimized to avoid any memory allocations.
func (m *Matcher) Matches(addressBytes []byte) bool {
	if !m.hasPrefix && !m.hasSuffix {
		return true // No criteria specified
	}

	// Convert address to lowercase hex string (40 chars) without allocation
	// We use a fixed buffer to avoid heap allocations
	var hexBuf [40]byte
	hexEncode(hexBuf[:], addressBytes)

	// Check prefix
	if m.hasPrefix {
		if len(m.prefixBytes) > 40 {
			return false
		}
		for i, b := range m.prefixBytes {
			if hexBuf[i] != b {
				return false
			}
		}
	}

	// Check suffix
	if m.hasSuffix {
		if len(m.suffixBytes) > 40 {
			return false
		}
		start := 40 - len(m.suffixBytes)
		for i, b := range m.suffixBytes {
			if hexBuf[start+i] != b {
				return false
			}
		}
	}

	return true
}

// hexEncode encodes src into dst as lowercase hexadecimal.
// dst must be at least len(src)*2 bytes.
// This is a simplified version that avoids the overhead of hex.Encode.
func hexEncode(dst, src []byte) {
	const hextable = "0123456789abcdef"
	for i, v := range src {
		dst[i*2] = hextable[v>>4]
		dst[i*2+1] = hextable[v&0x0f]
	}
}

// AddressToHex converts raw address bytes to a 0x-prefixed hex string.
// Used for the final result output only (not in the hot loop).
func AddressToHex(addressBytes []byte) string {
	return "0x" + hex.EncodeToString(addressBytes)
}
