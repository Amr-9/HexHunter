//go:build opencl
// +build opencl

package solana

import (
	"embed"

	"github.com/Amr-9/HexHunter/pkg/generator/common"
)

//go:embed kernels/solana_kernel.cl
var solanaKernelFS embed.FS

// LoadSolanaKernel loads and combines the Ed25519 core with Solana-specific kernel code.
// This approach reduces code duplication by sharing the Ed25519 implementation.
func LoadSolanaKernel() (string, error) {
	// 1. Load shared Ed25519 core
	coreKernel, err := common.LoadEd25519Core()
	if err != nil {
		return "", err
	}

	// 2. Load Solana-specific kernel
	solanaData, err := solanaKernelFS.ReadFile("kernels/solana_kernel.cl")
	if err != nil {
		return "", err
	}

	// 3. Concatenate: core + network-specific
	combined := coreKernel + "\n" + string(solanaData)

	// 4. Apply common OpenCL fixes for AMD/Intel compatibility
	return common.ApplyOpenCLFixes(combined), nil
}

// GetSolanaKernelName returns the name of the main kernel function
func GetSolanaKernelName() string {
	return "generate_pubkey"
}
