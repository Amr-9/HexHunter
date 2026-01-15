//go:build windows

package generator

import "syscall"

// HideFile sets the hidden attribute on a file (Windows only)
func HideFile(filename string) {
	filenamePtr, err := syscall.UTF16PtrFromString(filename)
	if err == nil {
		syscall.SetFileAttributes(filenamePtr, syscall.FILE_ATTRIBUTE_HIDDEN)
	}
}
