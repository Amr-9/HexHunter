//go:build opencl
// +build opencl

package solana

import (
	"embed"
	"strings"
)

//go:embed kernels/SolVanityCL.cl
var solVanityCLKernel embed.FS

// LoadSolanaKernel loads the SolVanityCL kernel source.
// The kernel now uses runtime parameters for prefix/suffix,
// so no compile-time string replacement is needed.
func LoadSolanaKernel() (string, error) {
	kernelData, err := solVanityCLKernel.ReadFile("kernels/SolVanityCL.cl")
	if err != nil {
		return "", err
	}

	kernelSrc := string(kernelData)

	// Fix OpenCL address space qualifier issues for AMD/Intel GPUs
	// The original kernel was designed for NVIDIA which is more lenient

	// 1. Remove the entire "#define __generic" line first
	kernelSrc = strings.ReplaceAll(kernelSrc, "#define __generic\r\n", "")
	kernelSrc = strings.ReplaceAll(kernelSrc, "#define __generic\n", "")

	// 2. Remove all remaining __generic qualifiers
	kernelSrc = strings.ReplaceAll(kernelSrc, "__generic ", "")
	kernelSrc = strings.ReplaceAll(kernelSrc, " __generic", "")

	// 3. Replace function signatures to use int* instead of fe (int[10])
	// This allows passing struct members without address space mismatch
	kernelSrc = strings.ReplaceAll(kernelSrc, "void fe_0(fe h)", "void fe_0(int* h)")
	kernelSrc = strings.ReplaceAll(kernelSrc, "void fe_1(fe h)", "void fe_1(int* h)")
	kernelSrc = strings.ReplaceAll(kernelSrc, "void fe_copy(fe h, const fe f)", "void fe_copy(int* h, const int* f)")
	kernelSrc = strings.ReplaceAll(kernelSrc, "void fe_add(fe h, const fe f, const fe g)", "void fe_add(int* h, const int* f, const int* g)")
	kernelSrc = strings.ReplaceAll(kernelSrc, "void fe_sub(fe h, const fe f, const fe g)", "void fe_sub(int* h, const int* f, const int* g)")
	kernelSrc = strings.ReplaceAll(kernelSrc, "void fe_mul(fe h, const fe f, const fe g)", "void fe_mul(int* h, const int* f, const int* g)")
	kernelSrc = strings.ReplaceAll(kernelSrc, "void fe_sq(fe h, const fe f)", "void fe_sq(int* h, const int* f)")
	kernelSrc = strings.ReplaceAll(kernelSrc, "void fe_sq2(fe h, const fe f)", "void fe_sq2(int* h, const int* f)")
	kernelSrc = strings.ReplaceAll(kernelSrc, "void fe_invert(fe out, const fe z)", "void fe_invert(int* out, const int* z)")
	kernelSrc = strings.ReplaceAll(kernelSrc, "void fe_neg(fe h, const fe f)", "void fe_neg(int* h, const int* f)")
	kernelSrc = strings.ReplaceAll(kernelSrc, "void fe_cmov(fe f, const fe g, unsigned int b)", "void fe_cmov(int* f, const int* g, unsigned int b)")
	kernelSrc = strings.ReplaceAll(kernelSrc, "void fe_cmov__constant(fe f, constant fe g, unsigned int b)", "void fe_cmov__constant(int* f, constant int* g, unsigned int b)")
	kernelSrc = strings.ReplaceAll(kernelSrc, "void fe_tobytes(unsigned char *s, const fe h)", "void fe_tobytes(unsigned char *s, const int* h)")
	kernelSrc = strings.ReplaceAll(kernelSrc, "unsigned int fe_isnegative(const fe f)", "unsigned int fe_isnegative(const int* f)")
	kernelSrc = strings.ReplaceAll(kernelSrc, "void fe_pow22523(fe out, const fe z)", "void fe_pow22523(int* out, const int* z)")

	return kernelSrc, nil
}

// GetSolanaKernelName returns the name of the main kernel function
func GetSolanaKernelName() string {
	return "generate_pubkey"
}
