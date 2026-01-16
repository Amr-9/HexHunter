//go:build opencl
// +build opencl

package sui

import (
	"embed"

	"github.com/Amr-9/HexHunter/pkg/generator/common"
)

//go:embed kernels/sui_kernel.cl
var suiKernelFS embed.FS

// LoadSuiKernel loads and combines the Ed25519 core with Sui-specific kernel code.
// This approach reduces code duplication by sharing the Ed25519 implementation.
func LoadSuiKernel() (string, error) {
	// 1. Load shared Ed25519 core
	coreKernel, err := common.LoadEd25519Core()
	if err != nil {
		return "", err
	}

	// 2. Load Sui-specific kernel (Blake2b-256, hex encoding, kernel function)
	suiData, err := suiKernelFS.ReadFile("kernels/sui_kernel.cl")
	if err != nil {
		return "", err
	}

	// 3. Concatenate: core + network-specific
	combined := coreKernel + "\n" + string(suiData)

	// 4. Apply common OpenCL fixes for AMD/Intel compatibility
	return common.ApplyOpenCLFixes(combined), nil
}

// GetSuiKernelName returns the name of the main kernel function
func GetSuiKernelName() string {
	return "generate_sui_address"
}
