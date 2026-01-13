//go:build !opencl
// +build !opencl

package generator

// GPUTestResult holds the result of GPU verification tests
type GPUTestResult struct {
	TestName     string
	PrivateKey   string
	GPUAddress   string
	CPUAddress   string
	Match        bool
	ErrorMessage string
}

// VerifyGPUImplementation stub for non-OpenCL builds
func VerifyGPUImplementation() (bool, []GPUTestResult) {
	return false, []GPUTestResult{{
		TestName:     "GPU Not Available",
		ErrorMessage: "OpenCL support not compiled. Build with: go build -tags opencl",
	}}
}
