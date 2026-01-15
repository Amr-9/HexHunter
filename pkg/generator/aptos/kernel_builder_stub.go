//go:build !opencl
// +build !opencl

package aptos

// LoadAptosKernel is a stub for non-OpenCL builds.
func LoadAptosKernel() (string, error) {
	return "", nil
}

// GetAptosKernelName is a stub for non-OpenCL builds.
func GetAptosKernelName() string {
	return ""
}
