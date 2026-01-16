package solana

import (
	"strings"
)

// Base58 alphabet (Bitcoin/Solana style - excludes 0, O, I, l)
const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// SolanaMatcher handles pattern matching for Solana addresses.
// Solana addresses are Base58-encoded and case-sensitive.
type SolanaMatcher struct {
	prefix   string
	suffix   string
	contains string
}

// NewSolanaMatcher creates a new Solana address matcher.
// Patterns are case-sensitive for Solana (Base58).
func NewSolanaMatcher(prefix, suffix, contains string) *SolanaMatcher {
	return &SolanaMatcher{
		prefix:   prefix,
		suffix:   suffix,
		contains: contains,
	}
}

// Matches checks if a Solana address matches the prefix, suffix, and contains criteria.
// This is case-sensitive matching.
func (m *SolanaMatcher) Matches(address string) bool {
	// Check prefix (case-sensitive)
	if m.prefix != "" && !strings.HasPrefix(address, m.prefix) {
		return false
	}

	// Check suffix (case-sensitive)
	if m.suffix != "" && !strings.HasSuffix(address, m.suffix) {
		return false
	}

	// Check contains in the middle section
	if m.contains != "" {
		// Calculate middle section (between prefix and suffix)
		startIdx := len(m.prefix)
		endIdx := len(address) - len(m.suffix)

		if startIdx >= endIdx || endIdx-startIdx < len(m.contains) {
			return false
		}

		// Search for contains in middle section (case-sensitive)
		middleSection := address[startIdx:endIdx]
		if !strings.Contains(middleSection, m.contains) {
			return false
		}
	}

	return true
}

// IsValidBase58 checks if a string contains only valid Base58 characters.
// Base58 excludes: 0 (zero), O (uppercase o), I (uppercase i), l (lowercase L)
func IsValidBase58(s string) bool {
	for _, c := range s {
		if !strings.ContainsRune(base58Alphabet, c) {
			return false
		}
	}
	return true
}

// InvalidBase58Chars returns any invalid Base58 characters in the input.
// Useful for providing helpful error messages to users.
func InvalidBase58Chars(s string) []rune {
	var invalid []rune
	for _, c := range s {
		if !strings.ContainsRune(base58Alphabet, c) {
			invalid = append(invalid, c)
		}
	}
	return invalid
}
