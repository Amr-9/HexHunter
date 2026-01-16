//go:build opencl
// +build opencl

package ethereum

import (
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/Amr-9/HexHunter/pkg/generator"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
)

// GenerateTable generates the precomputed table for GPU acceleration
// It writes to "tables.bin" and returns the data
func GenerateTable() ([]byte, error) {
	fmt.Printf("=== First Time Setup ===\n")
	fmt.Printf("Generating optimization tables (this only happens once)...\n")

	startTime := time.Now()

	// Create buffer
	buf := make([]byte, tableSize)

	curve := secp256k1.S256()
	Gx := curve.Gx
	Gy := curve.Gy

	var currentX, currentY *big.Int

	// Print progress every 10%
	progressStep := globalWorkSize / 10

	for i := 0; i < globalWorkSize; i++ {
		if i == 0 {
			// Point at infinity is all zeros
			// Already zero-initialized
		} else if i == 1 {
			currentX = new(big.Int).Set(Gx)
			currentY = new(big.Int).Set(Gy)
			writeAffinePoint(buf, i, currentX, currentY)
		} else {
			currentX, currentY = curve.Add(currentX, currentY, Gx, Gy)
			writeAffinePoint(buf, i, currentX, currentY)
		}

		if i > 0 && i%progressStep == 0 {
			fmt.Printf("Generating tables: %d%%\r", i*100/globalWorkSize)
		}
	}
	fmt.Printf("Generating tables: 100%%\n")

	// Save to file
	err := os.WriteFile("tables.bin", buf, 0644)
	if err != nil {
		fmt.Printf("Warning: Could not save tables.bin: %v\n", err)
	} else {
		// HideFile is now implemented in file_hidden_*.go
		generator.HideFile("tables.bin")
	}

	fmt.Printf("✓ Setup complete in %v\n\n", time.Since(startTime))
	return buf, nil
}

func writeAffinePoint(buf []byte, index int, x, y *big.Int) {
	offset := index * 64

	xBytes := x.Bytes()
	yBytes := y.Bytes()

	// Write X (Little Endian)
	for i := 0; i < len(xBytes) && i < 32; i++ {
		buf[offset+i] = xBytes[len(xBytes)-1-i]
	}

	// Write Y (Little Endian)
	for i := 0; i < len(yBytes) && i < 32; i++ {
		buf[offset+32+i] = yBytes[len(yBytes)-1-i]
	}
}
