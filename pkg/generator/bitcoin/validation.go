package bitcoin

import (
	"strings"

	"github.com/Amr-9/HexHunter/pkg/generator"
)

// Bech32 charset (excludes 1, b, i, o to prevent ambiguity)
// Note: 'b' is excluded because it's used in the 'bc' prefix
const bech32Charset = "023456789acdefghjklmnpqrstuvwxyz"

// Base58 charset (excludes 0, O, I, l)
const base58Charset = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// IsValidBech32Char checks if a character is valid in Bech32/Bech32m encoding.
// Used for P2TR (Taproot) address patterns.
func IsValidBech32Char(c rune) bool {
	return strings.ContainsRune(bech32Charset, c) ||
		strings.ContainsRune(strings.ToLower(bech32Charset), c)
}

// IsValidBase58Char checks if a character is valid in Base58 encoding.
// Used for P2PKH (Legacy) and P2SH (Nested SegWit) address patterns.
func IsValidBase58Char(c rune) bool {
	return strings.ContainsRune(base58Charset, c)
}

// IsValidPattern checks if a pattern is valid for the given address type.
func IsValidPattern(pattern string, addrType generator.AddressType) bool {
	if pattern == "" {
		return true
	}

	if IsBech32Type(addrType) {
		// Bech32 patterns should be lowercase
		for _, c := range strings.ToLower(pattern) {
			if !IsValidBech32Char(c) {
				return false
			}
		}
	} else {
		// Base58 patterns are case-sensitive
		for _, c := range pattern {
			if !IsValidBase58Char(c) {
				return false
			}
		}
	}

	return true
}

// InvalidChars returns any invalid characters in the pattern for the given address type.
func InvalidChars(pattern string, addrType generator.AddressType) []rune {
	var invalid []rune

	if IsBech32Type(addrType) {
		for _, c := range strings.ToLower(pattern) {
			if !IsValidBech32Char(c) {
				invalid = append(invalid, c)
			}
		}
	} else {
		for _, c := range pattern {
			if !IsValidBase58Char(c) {
				invalid = append(invalid, c)
			}
		}
	}

	return invalid
}

// InvalidBech32Chars returns invalid Bech32 characters in the input.
func InvalidBech32Chars(s string) []rune {
	var invalid []rune
	for _, c := range strings.ToLower(s) {
		if !IsValidBech32Char(c) {
			invalid = append(invalid, c)
		}
	}
	return invalid
}

// InvalidBase58Chars returns invalid Base58 characters in the input.
func InvalidBase58Chars(s string) []rune {
	var invalid []rune
	for _, c := range s {
		if !IsValidBase58Char(c) {
			invalid = append(invalid, c)
		}
	}
	return invalid
}
