//go:build !opencl
// +build !opencl

package sui

// LoadSuiKernel is a stub for non-OpenCL builds
func LoadSuiKernel() (string, error) {
	return "", nil
}

// GetSuiKernelName returns the name of the main kernel function
func GetSuiKernelName() string {
	return "generate_sui_address"
}
