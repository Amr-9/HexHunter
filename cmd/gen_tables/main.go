// gen_tables generates precomputed elliptic curve points for GPU acceleration.
// Output: tables.bin containing 2^20 affine points (i*G for i=0 to 2^20-1)
package main

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/crypto/secp256k1"
)

const (
	TableSize = 1 << 20 // 2^20 = 1,048,576 points
	PointSize = 64      // 32 bytes X + 32 bytes Y (Affine)
)

func main() {
	fmt.Printf("=== Phase 3: Precomputed Table Generator ===\n")
	fmt.Printf("Generating %d points (%.2f MB)...\n", TableSize, float64(TableSize*PointSize)/(1024*1024))

	startTime := time.Now()

	// Open output file
	file, err := os.Create("tables.bin")
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Get the secp256k1 curve
	curve := secp256k1.S256()

	// Generator point G
	Gx := curve.Gx
	Gy := curve.Gy

	// Current point (starts at infinity, represented as 0,0)
	// We'll compute i*G iteratively: point[i] = point[i-1] + G
	var currentX, currentY *big.Int

	// Buffer for writing points
	buf := make([]byte, PointSize)

	for i := 0; i < TableSize; i++ {
		if i == 0 {
			// Point at infinity (0*G) - represented as zero bytes
			// This is a special case - the GPU will need to handle this
			for j := range buf {
				buf[j] = 0
			}
		} else if i == 1 {
			// First point is G itself
			currentX = new(big.Int).Set(Gx)
			currentY = new(big.Int).Set(Gy)
			writeAffinePoint(buf, currentX, currentY)
		} else {
			// Add G to current point: point[i] = point[i-1] + G
			currentX, currentY = curve.Add(currentX, currentY, Gx, Gy)
			writeAffinePoint(buf, currentX, currentY)
		}

		// Write to file
		_, err := file.Write(buf)
		if err != nil {
			fmt.Printf("Error writing point %d: %v\n", i, err)
			os.Exit(1)
		}

		// Progress indicator
		if i > 0 && i%(TableSize/10) == 0 {
			progress := float64(i) / float64(TableSize) * 100
			fmt.Printf("  Progress: %.0f%%\n", progress)
		}
	}

	elapsed := time.Since(startTime)
	fmt.Printf("\n✓ Generated %d points in %v\n", TableSize, elapsed)
	fmt.Printf("✓ Output file: tables.bin (%.2f MB)\n", float64(TableSize*PointSize)/(1024*1024))

	// Verify file size
	fileInfo, _ := file.Stat()
	expectedSize := int64(TableSize * PointSize)
	if fileInfo.Size() != expectedSize {
		fmt.Printf("⚠ Warning: File size mismatch! Expected %d, got %d\n", expectedSize, fileInfo.Size())
	} else {
		fmt.Printf("✓ File size verified: %d bytes\n", fileInfo.Size())
	}

	// Print sample points for verification
	fmt.Printf("\n=== Sample Points (for verification) ===\n")
	verifyPoints(curve)
}

// writeAffinePoint writes X and Y coordinates to buffer in little-endian format
// Each coordinate is 32 bytes, padded with leading zeros if needed
func writeAffinePoint(buf []byte, x, y *big.Int) {
	xBytes := x.Bytes()
	yBytes := y.Bytes()

	// Clear buffer
	for i := range buf {
		buf[i] = 0
	}

	// Write X in little-endian (reverse byte order)
	for i := 0; i < len(xBytes) && i < 32; i++ {
		buf[i] = xBytes[len(xBytes)-1-i]
	}

	// Write Y in little-endian (reverse byte order)
	for i := 0; i < len(yBytes) && i < 32; i++ {
		buf[32+i] = yBytes[len(yBytes)-1-i]
	}
}

// verifyPoints reads back and verifies some sample points
func verifyPoints(curve *secp256k1.BitCurve) {
	file, err := os.Open("tables.bin")
	if err != nil {
		fmt.Printf("Cannot open file for verification: %v\n", err)
		return
	}
	defer file.Close()

	// Read and verify point[1] = G
	buf := make([]byte, PointSize)
	file.Seek(1*PointSize, 0)
	file.Read(buf)

	x := readLittleEndian(buf[0:32])
	y := readLittleEndian(buf[32:64])

	fmt.Printf("Table[1] (should be G):\n")
	fmt.Printf("  X: %s\n", x.Text(16))
	fmt.Printf("  Gx: %s\n", curve.Gx.Text(16))
	fmt.Printf("  Y: %s\n", y.Text(16))
	fmt.Printf("  Gy: %s\n", curve.Gy.Text(16))
	fmt.Printf("  Match: %v\n", x.Cmp(curve.Gx) == 0 && y.Cmp(curve.Gy) == 0)

	// Read and verify point[2] = 2*G
	file.Seek(2*PointSize, 0)
	file.Read(buf)

	x2 := readLittleEndian(buf[0:32])
	y2 := readLittleEndian(buf[32:64])

	// Compute 2*G
	expected2Gx, expected2Gy := curve.Double(curve.Gx, curve.Gy)

	fmt.Printf("\nTable[2] (should be 2*G):\n")
	fmt.Printf("  X: %s\n", x2.Text(16))
	fmt.Printf("  Expected: %s\n", expected2Gx.Text(16))
	fmt.Printf("  Match: %v\n", x2.Cmp(expected2Gx) == 0 && y2.Cmp(expected2Gy) == 0)
}

// readLittleEndian converts little-endian bytes to big.Int
func readLittleEndian(buf []byte) *big.Int {
	// Reverse bytes to big-endian
	reversed := make([]byte, len(buf))
	for i := 0; i < len(buf); i++ {
		reversed[i] = buf[len(buf)-1-i]
	}
	return new(big.Int).SetBytes(reversed)
}

// For compatibility - not used but matches binary package import
var _ = binary.LittleEndian
