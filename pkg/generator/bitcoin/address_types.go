// Package bitcoin provides Bitcoin vanity address generation support.
// Supports P2TR (Taproot), P2PKH (Legacy), and P2SH-P2WPKH (Nested SegWit).
package bitcoin

import (
	"github.com/Amr-9/HexHunter/pkg/generator"
)

// AddressPrefix returns the expected prefix for a Bitcoin address type.
func AddressPrefix(addrType generator.AddressType) string {
	switch addrType {
	case generator.AddressTypeTaproot:
		return "bc1p"
	case generator.AddressTypeLegacy:
		return "1"
	case generator.AddressTypeNestedSegWit:
		return "3"
	default:
		return "bc1p" // Default to Taproot
	}
}

// AddressDescription returns a human-readable description of an address type.
func AddressDescription(addrType generator.AddressType) string {
	switch addrType {
	case generator.AddressTypeTaproot:
		return "Taproot (bc1p...)"
	case generator.AddressTypeLegacy:
		return "Legacy (1...)"
	case generator.AddressTypeNestedSegWit:
		return "Nested SegWit (3...)"
	default:
		return "Unknown"
	}
}

// IsBech32Type returns true if the address type uses Bech32/Bech32m encoding.
func IsBech32Type(addrType generator.AddressType) bool {
	return addrType == generator.AddressTypeTaproot || addrType == generator.AddressTypeDefault
}

// IsBase58Type returns true if the address type uses Base58Check encoding.
func IsBase58Type(addrType generator.AddressType) bool {
	return addrType == generator.AddressTypeLegacy || addrType == generator.AddressTypeNestedSegWit
}
