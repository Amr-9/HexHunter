//go:build windows

package generator

import "syscall"

// hideFile sets the hidden attribute on a file (Windows only)
func hideFile(filename string) {
	filenamePtr, err := syscall.UTF16PtrFromString(filename)
	if err == nil {
		syscall.SetFileAttributes(filenamePtr, syscall.FILE_ATTRIBUTE_HIDDEN)
	}
}
