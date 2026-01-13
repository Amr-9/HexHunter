//go:build !windows

package generator

// hideFile does nothing on non-Windows systems
func hideFile(filename string) {
	// No-op for Linux/Mac
}
