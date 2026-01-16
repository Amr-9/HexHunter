//go:build opencl
// +build opencl

package common

import (
	"embed"
	"strings"
)

//go:embed kernels/ed25519_core.cl
var ed25519CoreKernel embed.FS

// LoadEd25519Core loads the shared Ed25519 core implementation
func LoadEd25519Core() (string, error) {
	kernelData, err := ed25519CoreKernel.ReadFile("kernels/ed25519_core.cl")
	if err != nil {
		return "", err
	}
	return string(kernelData), nil
}

// ApplyOpenCLFixes applies common OpenCL compatibility fixes for AMD/Intel GPUs.
// The original kernel was designed for NVIDIA which is more lenient with address space qualifiers.
func ApplyOpenCLFixes(kernelSrc string) string {
	// 1. Remove the entire "#define __generic" line first
	kernelSrc = strings.ReplaceAll(kernelSrc, "#define __generic\r\n", "")
	kernelSrc = strings.ReplaceAll(kernelSrc, "#define __generic\n", "")

	// 2. Remove all remaining __generic qualifiers
	kernelSrc = strings.ReplaceAll(kernelSrc, "__generic ", "")
	kernelSrc = strings.ReplaceAll(kernelSrc, " __generic", "")

	// 3. Replace function signatures to use int* instead of fe (int[10])
	// This allows passing struct members without address space mismatch
	feReplacements := map[string]string{
		"void fe_0(fe h)":                                             "void fe_0(int* h)",
		"void fe_1(fe h)":                                             "void fe_1(int* h)",
		"void fe_copy(fe h, const fe f)":                              "void fe_copy(int* h, const int* f)",
		"void fe_add(fe h, const fe f, const fe g)":                   "void fe_add(int* h, const int* f, const int* g)",
		"void fe_sub(fe h, const fe f, const fe g)":                   "void fe_sub(int* h, const int* f, const int* g)",
		"void fe_mul(fe h, const fe f, const fe g)":                   "void fe_mul(int* h, const int* f, const int* g)",
		"void fe_sq(fe h, const fe f)":                                "void fe_sq(int* h, const int* f)",
		"void fe_sq2(fe h, const fe f)":                               "void fe_sq2(int* h, const int* f)",
		"void fe_invert(fe out, const fe z)":                          "void fe_invert(int* out, const int* z)",
		"void fe_neg(fe h, const fe f)":                               "void fe_neg(int* h, const int* f)",
		"void fe_cmov(fe f, const fe g, unsigned int b)":              "void fe_cmov(int* f, const int* g, unsigned int b)",
		"void fe_cmov__constant(fe f, constant fe g, unsigned int b)": "void fe_cmov__constant(int* f, constant int* g, unsigned int b)",
		"void fe_tobytes(unsigned char *s, const fe h)":               "void fe_tobytes(unsigned char *s, const int* h)",
		"int fe_isnegative(const fe f)":                               "int fe_isnegative(const int* f)",
		"unsigned int fe_isnegative(const fe f)":                      "unsigned int fe_isnegative(const int* f)",
		"void fe_pow22523(fe out, const fe z)":                        "void fe_pow22523(int* out, const int* z)",
	}

	for old, new := range feReplacements {
		kernelSrc = strings.ReplaceAll(kernelSrc, old, new)
	}

	return kernelSrc
}
