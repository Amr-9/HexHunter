package bitcoin

import (
	"crypto/rand"

	"github.com/btcsuite/btcd/btcec/v2"
)

// GenerateKeyPair generates a new random secp256k1 key pair for Bitcoin.
// Returns the private key and compressed public key (33 bytes).
func GenerateKeyPair() (*btcec.PrivateKey, *btcec.PublicKey, error) {
	// Generate 32 bytes of random data for private key
	var privKeyBytes [32]byte
	if _, err := rand.Read(privKeyBytes[:]); err != nil {
		return nil, nil, err
	}

	// Create private key from bytes
	privKey, pubKey := btcec.PrivKeyFromBytes(privKeyBytes[:])
	return privKey, pubKey, nil
}

// PrivateKeyToWIF converts a private key to Wallet Import Format (WIF).
// Uses compressed format (starts with K or L on mainnet).
func PrivateKeyToWIF(privKey *btcec.PrivateKey) string {
	// WIF = Base58Check(0x80 + privKey + 0x01)
	// 0x80 = mainnet prefix
	// 0x01 suffix = compressed public key flag
	data := make([]byte, 34)
	data[0] = 0x80 // Mainnet prefix
	copy(data[1:33], privKey.Serialize())
	data[33] = 0x01 // Compressed flag

	return Base58CheckEncode(data)
}
