//go:build !windows

package main

// On non-Windows systems, process priority is typically set via:
// - nice/renice commands on Linux/macOS
// - Running as: nice -n -20 ./HexHunter (highest priority)
//
// For now, we don't automatically set priority on non-Windows systems.
// Users who need maximum performance can use the above commands.
