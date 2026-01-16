package bitcoin

import (
	"crypto/sha256"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil/bech32"
	"golang.org/x/crypto/ripemd160"

	"github.com/Amr-9/HexHunter/pkg/generator"
)

// DeriveAddress derives a Bitcoin address from a public key based on the address type.
func DeriveAddress(pubKey *btcec.PublicKey, addrType generator.AddressType) string {
	switch addrType {
	case generator.AddressTypeTaproot:
		return deriveTaprootAddress(pubKey)
	case generator.AddressTypeLegacy:
		return deriveLegacyAddress(pubKey)
	case generator.AddressTypeNestedSegWit:
		return deriveNestedSegWitAddress(pubKey)
	default:
		return deriveTaprootAddress(pubKey) // Default to Taproot
	}
}

// deriveTaprootAddress creates a P2TR (bc1p...) address using Bech32m encoding.
// Taproot address = Bech32m(HRP="bc", version=1, tweaked_pubkey_x)
// BIP-341: The tweaked key is computed as: P + hash(P||m)*G where m is empty for key-path spend
func deriveTaprootAddress(pubKey *btcec.PublicKey) string {
	// BIP-341 key tweaking for key-path spend (no script path)
	// tweak = TaggedHash("TapTweak", pubkey_x)
	// tweaked_pubkey = pubkey + tweak * G

	// Get the x-only public key (32 bytes)
	xOnlyBytes := schnorr.SerializePubKey(pubKey)

	// Compute the tweak: TaggedHash("TapTweak", pubkey_x)
	// For key-path only spend, we don't include a merkle root
	tweak := taprootTweak(xOnlyBytes, nil)

	// Add the tweak to the public key
	// tweaked_pubkey = pubkey + tweak * G
	var tweakScalar btcec.ModNScalar
	tweakScalar.SetBytes((*[32]byte)(tweak))

	// Get the generator point G
	var result btcec.JacobianPoint
	btcec.ScalarBaseMultNonConst(&tweakScalar, &result)

	// Convert pubKey to Jacobian and add
	var pubKeyJacobian btcec.JacobianPoint
	pubKey.AsJacobian(&pubKeyJacobian)

	// result = pubKey + tweak*G
	btcec.AddNonConst(&pubKeyJacobian, &result, &result)

	// Convert back to affine and get x-only
	result.ToAffine()
	tweakedPubKey := btcec.NewPublicKey(&result.X, &result.Y)
	tweakedXOnly := schnorr.SerializePubKey(tweakedPubKey)

	// Convert to 5-bit groups for Bech32m
	data, err := bech32.ConvertBits(tweakedXOnly, 8, 5, true)
	if err != nil {
		return ""
	}

	// Prepend witness version 1 for Taproot
	data = append([]byte{0x01}, data...)

	// Encode using Bech32m (version 1+ uses Bech32m, not Bech32)
	addr, err := bech32.EncodeM("bc", data)
	if err != nil {
		return ""
	}

	return addr
}

// taprootTweak computes the BIP-341 tweak for Taproot.
// TaggedHash("TapTweak", pubkey_x || merkle_root)
// For key-path only, merkle_root is empty.
func taprootTweak(pubKeyX []byte, merkleRoot []byte) []byte {
	// Tagged hash: SHA256(SHA256(tag) || SHA256(tag) || data)
	tagHash := sha256.Sum256([]byte("TapTweak"))

	h := sha256.New()
	h.Write(tagHash[:])
	h.Write(tagHash[:])
	h.Write(pubKeyX)
	if len(merkleRoot) > 0 {
		h.Write(merkleRoot)
	}

	result := h.Sum(nil)
	return result
}

// deriveLegacyAddress creates a P2PKH (1...) address using Base58Check encoding.
// Legacy address = Base58Check(0x00 + HASH160(pubkey))
func deriveLegacyAddress(pubKey *btcec.PublicKey) string {
	// HASH160 = RIPEMD160(SHA256(compressed_pubkey))
	pubKeyBytes := pubKey.SerializeCompressed()
	hash160 := hash160(pubKeyBytes)

	// Version byte 0x00 for mainnet P2PKH
	data := make([]byte, 21)
	data[0] = 0x00
	copy(data[1:], hash160)

	return Base58CheckEncode(data)
}

// deriveNestedSegWitAddress creates a P2SH-P2WPKH (3...) address.
// This wraps a SegWit address inside a P2SH script for compatibility.
// Address = Base58Check(0x05 + HASH160(0x0014 + HASH160(pubkey)))
func deriveNestedSegWitAddress(pubKey *btcec.PublicKey) string {
	// First, get HASH160 of the compressed public key
	pubKeyBytes := pubKey.SerializeCompressed()
	pubKeyHash := hash160(pubKeyBytes)

	// Create the P2WPKH witness program: OP_0 (0x00) + push 20 bytes (0x14) + pubkeyhash
	witnessProgram := make([]byte, 22)
	witnessProgram[0] = 0x00 // Witness version 0
	witnessProgram[1] = 0x14 // Push 20 bytes
	copy(witnessProgram[2:], pubKeyHash)

	// HASH160 of the witness program
	scriptHash := hash160(witnessProgram)

	// Version byte 0x05 for mainnet P2SH
	data := make([]byte, 21)
	data[0] = 0x05
	copy(data[1:], scriptHash)

	return Base58CheckEncode(data)
}

// hash160 computes RIPEMD160(SHA256(data))
func hash160(data []byte) []byte {
	sha := sha256.Sum256(data)
	ripemd := ripemd160.New()
	ripemd.Write(sha[:])
	return ripemd.Sum(nil)
}

// Base58CheckEncode encodes data with a 4-byte checksum in Base58.
func Base58CheckEncode(data []byte) string {
	// Double SHA256 for checksum
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])

	// Append first 4 bytes of second hash as checksum
	full := append(data, second[:4]...)

	return base58Encode(full)
}

// base58Encode encodes bytes to Base58 string.
func base58Encode(data []byte) string {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

	// Count leading zeros
	zeros := 0
	for _, b := range data {
		if b != 0 {
			break
		}
		zeros++
	}

	// Allocate enough space for the result
	size := len(data)*138/100 + 1
	buf := make([]byte, size)
	for _, b := range data {
		carry := int(b)
		for i := size - 1; i >= 0; i-- {
			carry += 256 * int(buf[i])
			buf[i] = byte(carry % 58)
			carry /= 58
		}
	}

	// Skip leading zeros in the buffer
	i := 0
	for i < size && buf[i] == 0 {
		i++
	}

	// Build the result string
	result := make([]byte, zeros+size-i)
	for j := 0; j < zeros; j++ {
		result[j] = '1' // Leading zeros become '1' in Base58
	}
	for j := zeros; i < size; i, j = i+1, j+1 {
		result[j] = alphabet[buf[i]]
	}

	return string(result)
}
