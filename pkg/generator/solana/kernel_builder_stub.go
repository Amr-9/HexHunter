//go:build !opencl
// +build !opencl

package solana

// LoadSolanaKernel is a stub for non-OpenCL builds
func LoadSolanaKernel() (string, error) {
	return "", nil
}

// GetSolanaKernelName returns the name of the main kernel function
func GetSolanaKernelName() string {
	return "generate_pubkey"
}
