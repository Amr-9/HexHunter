//go:build opencl
// +build opencl

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ethvanity/generator"
)

func main() {
	fmt.Println()
	fmt.Println("  ╔═══════════════════════════════════════════════════════════════════╗")
	fmt.Println("  ║              🔬 GPU Verification Test (Phase 1)                   ║")
	fmt.Println("  ╚═══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Println("  🔄 Running GPU vs CPU comparison tests...")
	fmt.Println()

	passed, results := generator.VerifyGPUImplementation()

	// Print results
	for i, r := range results {
		fmt.Printf("  Test %d: %s\n", i+1, r.TestName)
		if r.ErrorMessage != "" {
			fmt.Printf("    ❌ Error: %s\n", r.ErrorMessage)
		} else {
			fmt.Printf("    🔑 Private Key: %s...%s\n", r.PrivateKey[:8], r.PrivateKey[len(r.PrivateKey)-8:])
			fmt.Printf("    💻 CPU Address: %s\n", r.CPUAddress)
			fmt.Printf("    🎮 GPU Address: %s\n", r.GPUAddress)
			if r.Match {
				fmt.Printf("    ✅ MATCH!\n")
			} else {
				fmt.Printf("    ❌ MISMATCH!\n")
			}
		}
		fmt.Println()
	}

	// Summary
	fmt.Println("  ─────────────────────────────────────────────────────────────────")
	if passed {
		fmt.Println("  ✅ ALL TESTS PASSED! GPU implementation is correct.")
		fmt.Println("  ➡️  Ready for Phase 2: Parallelization")
	} else {
		fmt.Println("  ❌ SOME TESTS FAILED! GPU implementation needs fixing.")
		fmt.Println("  Review the mismatches above to debug.")
	}
	fmt.Println()

	if !passed {
		os.Exit(1)
	}
}

func centerPad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	leftPad := (width - len(s)) / 2
	rightPad := width - len(s) - leftPad
	return strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad)
}
