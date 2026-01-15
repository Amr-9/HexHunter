package aptos

import (
	"encoding/hex"
	"strings"

	"golang.org/x/crypto/sha3"
)

// AptosMatcher handles pattern matching for Aptos addresses.
// Aptos addresses are 64-character hex strings with 0x prefix.
// Address = SHA3-256(pubkey || 0x00)
type AptosMatcher struct {
	prefix string
	suffix string
}

// NewAptosMatcher creates a new Aptos address matcher.
// Patterns are case-insensitive for Aptos (Hex).
func NewAptosMatcher(prefix, suffix string) *AptosMatcher {
	return &AptosMatcher{
		prefix: strings.ToLower(prefix),
		suffix: strings.ToLower(suffix),
	}
}

// Matches checks if an Aptos address matches the prefix and suffix criteria.
// This is case-insensitive matching (hex addresses).
func (m *AptosMatcher) Matches(address string) bool {
	// Remove 0x prefix for matching
	addr := strings.ToLower(strings.TrimPrefix(address, "0x"))

	// Check prefix (case-insensitive)
	if m.prefix != "" && !strings.HasPrefix(addr, m.prefix) {
		return false
	}

	// Check suffix (case-insensitive)
	if m.suffix != "" && !strings.HasSuffix(addr, m.suffix) {
		return false
	}

	return true
}

// DeriveAddress derives an Aptos address from an Ed25519 public key.
// Formula: SHA3-256(pubkey || 0x00)
// The 0x00 is the single-signature scheme identifier.
func DeriveAddress(pubKey []byte) string {
	// Aptos: SHA3-256(pubkey || 0x00)
	data := make([]byte, len(pubKey)+1)
	copy(data, pubKey)
	data[len(pubKey)] = 0x00 // Single-signature scheme identifier

	hash := sha3.Sum256(data)
	return "0x" + hex.EncodeToString(hash[:])
}

// IsValidHex checks if a string contains only valid hex characters.
func IsValidHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
