package tron

import (
	"strings"
)

// TronMatcher handles pattern matching for Tron addresses.
// Tron addresses are Base58-encoded and case-sensitive, starting with 'T'.
type TronMatcher struct {
	prefix   string
	suffix   string
	contains string
}

// NewTronMatcher creates a new Tron address matcher.
// Patterns are case-sensitive for Tron (Base58).
func NewTronMatcher(prefix, suffix, contains string) *TronMatcher {
	return &TronMatcher{
		prefix:   prefix,
		suffix:   suffix,
		contains: contains,
	}
}

// Matches checks if a Tron address matches the prefix, suffix, and contains criteria.
// This is case-sensitive matching.
func (m *TronMatcher) Matches(address string) bool {
	// Tron addresses always start with 'T', so we match after the T
	// Check prefix (case-sensitive) - after the T
	if m.prefix != "" {
		searchAddr := address[1:] // Skip the 'T'
		if !strings.HasPrefix(searchAddr, m.prefix) {
			return false
		}
	}

	// Check suffix (case-sensitive)
	if m.suffix != "" && !strings.HasSuffix(address, m.suffix) {
		return false
	}

	// Check contains in the middle section
	if m.contains != "" {
		// Middle section is after 'T' + prefix and before suffix
		startIdx := 1 + len(m.prefix) // Skip 'T' + prefix
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
