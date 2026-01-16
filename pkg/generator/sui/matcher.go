package sui

import (
	"encoding/hex"
	"strings"

	"golang.org/x/crypto/blake2b"
)

// SuiMatcher handles Sui address matching with hex validation.
// Sui addresses are 32 bytes (64 hex chars) with 0x prefix.
type SuiMatcher struct {
	prefix   string
	suffix   string
	contains string
}

// NewSuiMatcher creates a new Sui address matcher.
// Patterns should be lowercase hex characters without the 0x prefix.
func NewSuiMatcher(prefix, suffix, contains string) *SuiMatcher {
	return &SuiMatcher{
		prefix:   strings.ToLower(prefix),
		suffix:   strings.ToLower(suffix),
		contains: strings.ToLower(contains),
	}
}

// Matches checks if the address matches the prefix, suffix, and contains patterns.
// The address should be in the format 0x... (64 hex chars).
func (m *SuiMatcher) Matches(address string) bool {
	// Remove 0x prefix for matching
	addr := strings.TrimPrefix(address, "0x")
	addr = strings.ToLower(addr)

	// Check prefix
	if m.prefix != "" && !strings.HasPrefix(addr, m.prefix) {
		return false
	}

	// Check suffix
	if m.suffix != "" && !strings.HasSuffix(addr, m.suffix) {
		return false
	}

	// Check contains in the middle section
	if m.contains != "" {
		// Calculate middle section (between prefix and suffix)
		startIdx := len(m.prefix)
		endIdx := len(addr) - len(m.suffix)

		if startIdx >= endIdx || endIdx-startIdx < len(m.contains) {
			return false
		}

		// Search for contains in middle section (case-insensitive, already lowercased)
		middleSection := addr[startIdx:endIdx]
		if !strings.Contains(middleSection, m.contains) {
			return false
		}
	}

	return true
}

// DeriveAddress computes the Sui address from an Ed25519 public key.
// Sui address = Blake2b-256(0x00 || pubkey) where 0x00 is Ed25519 signature scheme flag.
func DeriveAddress(pubKey []byte) string {
	// Prepend 0x00 (Ed25519 signature scheme flag)
	data := make([]byte, len(pubKey)+1)
	data[0] = 0x00
	copy(data[1:], pubKey)

	// Compute Blake2b-256
	hash := blake2b.Sum256(data)
	return "0x" + hex.EncodeToString(hash[:])
}

// IsValidHex checks if a string contains only valid hex characters.
func IsValidHex(s string) bool {
	s = strings.ToLower(s)
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
