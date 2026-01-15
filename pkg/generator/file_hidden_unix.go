//go:build !windows

package generator

// HideFile does nothing on non-Windows systems
func HideFile(filename string) {
	// No-op for Linux/Mac
}
