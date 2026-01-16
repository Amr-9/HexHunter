package tron

import (
	"crypto/sha256"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mr-tron/base58"
)

// TronMainnetPrefix is the address prefix for Tron mainnet (0x41)
const TronMainnetPrefix = 0x41

// DeriveAddress derives a Tron address from a public key.
// Tron address = Base58Check(0x41 + last 20 bytes of Keccak256(pubKey[1:]))
// All Tron addresses start with 'T'.
func DeriveAddress(pubKeyBytes []byte) string {
	// Skip the 0x04 prefix for uncompressed public key, take last 64 bytes
	// Then compute Keccak256 hash
	hash := crypto.Keccak256(pubKeyBytes[1:])

	// Take last 20 bytes of the hash
	addressBytes := hash[len(hash)-20:]

	// Prepend Tron mainnet prefix (0x41)
	data := make([]byte, 21)
	data[0] = TronMainnetPrefix
	copy(data[1:], addressBytes)

	// Encode using Base58Check
	return Base58CheckEncode(data)
}

// Base58CheckEncode encodes data with a 4-byte checksum in Base58.
// This is the same encoding used by Bitcoin.
func Base58CheckEncode(data []byte) string {
	// Double SHA256 for checksum
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])

	// Append first 4 bytes of second hash as checksum
	full := append(data, second[:4]...)

	// Use the mr-tron/base58 library for correct encoding
	return base58.Encode(full)
}

// PrivateKeyToHex converts a private key to hex string.
func PrivateKeyToHex(privKeyBytes []byte) string {
	const hextable = "0123456789abcdef"
	result := make([]byte, len(privKeyBytes)*2)
	for i, v := range privKeyBytes {
		result[i*2] = hextable[v>>4]
		result[i*2+1] = hextable[v&0x0f]
	}
	return string(result)
}
