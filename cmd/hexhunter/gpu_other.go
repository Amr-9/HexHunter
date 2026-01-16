//go:build !windows

package main

// On non-Windows systems, we don't need GPU preference exports.
// GPU selection is typically handled through environment variables
// like DRI_PRIME=1 on Linux or through system settings on macOS.
