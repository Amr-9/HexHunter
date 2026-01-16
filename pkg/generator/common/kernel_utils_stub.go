//go:build !opencl
// +build !opencl

package common

// LoadEd25519Core stub - OpenCL not available
func LoadEd25519Core() (string, error) {
	return "", nil
}

// ApplyOpenCLFixes stub - OpenCL not available
func ApplyOpenCLFixes(kernelSrc string) string {
	return kernelSrc
}
