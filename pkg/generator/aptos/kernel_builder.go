//go:build opencl
// +build opencl

package aptos

import (
	"embed"

	"github.com/Amr-9/HexHunter/pkg/generator/common"
)

//go:embed kernels/aptos_kernel.cl
var aptosKernelFS embed.FS

// LoadAptosKernel loads and combines the Ed25519 core with Aptos-specific kernel code.
// This approach reduces code duplication by sharing the Ed25519 implementation.
func LoadAptosKernel() (string, error) {
	// 1. Load shared Ed25519 core
	coreKernel, err := common.LoadEd25519Core()
	if err != nil {
		return "", err
	}

	// 2. Load Aptos-specific kernel (SHA3-256, hex encoding, kernel function)
	aptosData, err := aptosKernelFS.ReadFile("kernels/aptos_kernel.cl")
	if err != nil {
		return "", err
	}

	// 3. Concatenate: core + network-specific
	combined := coreKernel + "\n" + string(aptosData)

	// 4. Apply common OpenCL fixes for AMD/Intel compatibility
	return common.ApplyOpenCLFixes(combined), nil
}

// GetAptosKernelName returns the name of the main kernel function
func GetAptosKernelName() string {
	return "generate_aptos_address"
}
