package bitcoin

import (
	"strings"

	"github.com/Amr-9/HexHunter/pkg/generator"
)

// BitcoinMatcher handles pattern matching for Bitcoin addresses.
// Case sensitivity depends on address type:
// - Bech32/Bech32m (P2TR): Case-insensitive, always compares as lowercase
// - Base58 (P2PKH, P2SH): Case-sensitive
type BitcoinMatcher struct {
	prefix      string
	suffix      string
	addressType generator.AddressType
	isBech32    bool
}

// NewBitcoinMatcher creates a new Bitcoin address matcher.
func NewBitcoinMatcher(prefix, suffix string, addrType generator.AddressType) *BitcoinMatcher {
	isBech32 := IsBech32Type(addrType)

	// For Bech32 addresses, normalize to lowercase
	if isBech32 {
		prefix = strings.ToLower(prefix)
		suffix = strings.ToLower(suffix)
	}

	return &BitcoinMatcher{
		prefix:      prefix,
		suffix:      suffix,
		addressType: addrType,
		isBech32:    isBech32,
	}
}

// Matches checks if a Bitcoin address matches the prefix and suffix criteria.
func (m *BitcoinMatcher) Matches(address string) bool {
	// For Bech32 addresses, compare as lowercase
	if m.isBech32 {
		address = strings.ToLower(address)
	}

	// Check prefix
	if m.prefix != "" && !strings.HasPrefix(address, m.prefix) {
		return false
	}

	// Check suffix
	if m.suffix != "" && !strings.HasSuffix(address, m.suffix) {
		return false
	}

	return true
}

// MatchesAfterPrefix matches after the standard address prefix.
// For Taproot: matches after "bc1p"
// For Legacy: matches after "1"
// For Nested SegWit: matches after "3"
func (m *BitcoinMatcher) MatchesAfterPrefix(address string) bool {
	stdPrefix := AddressPrefix(m.addressType)

	// For Bech32, normalize to lowercase
	if m.isBech32 {
		address = strings.ToLower(address)
	}

	// Skip the standard prefix
	if len(address) <= len(stdPrefix) {
		return false
	}
	addressWithoutPrefix := address[len(stdPrefix):]

	// Check user's custom prefix
	if m.prefix != "" && !strings.HasPrefix(addressWithoutPrefix, m.prefix) {
		return false
	}

	// Check user's custom suffix
	if m.suffix != "" && !strings.HasSuffix(address, m.suffix) {
		return false
	}

	return true
}
