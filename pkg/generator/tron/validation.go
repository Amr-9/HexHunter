package tron

import (
	"strings"
)

// Base58 alphabet for validation (excludes 0, O, I, l)
const validBase58Chars = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// IsValidBase58 checks if a string contains only valid Base58 characters.
// Base58 excludes: 0 (zero), O (uppercase o), I (uppercase i), l (lowercase L)
func IsValidBase58(s string) bool {
	for _, c := range s {
		if !strings.ContainsRune(validBase58Chars, c) {
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
		if !strings.ContainsRune(validBase58Chars, c) {
			invalid = append(invalid, c)
		}
	}
	return invalid
}
