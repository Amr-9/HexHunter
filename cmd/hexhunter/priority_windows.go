//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// Windows priority constants
const (
	REALTIME_PRIORITY_CLASS     = 0x00000100
	HIGH_PRIORITY_CLASS         = 0x00000080
	ABOVE_NORMAL_PRIORITY_CLASS = 0x00008000
	NORMAL_PRIORITY_CLASS       = 0x00000020
)

var (
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	procGetCurrentProcess = kernel32.NewProc("GetCurrentProcess")
	procSetPriorityClass  = kernel32.NewProc("SetPriorityClass")
)

// SetHighPriority sets the current process to high priority
// This gives the process more CPU time compared to normal priority processes
func SetHighPriority() error {
	handle, _, _ := procGetCurrentProcess.Call()

	// Use HIGH_PRIORITY_CLASS (not REALTIME which can freeze the system)
	ret, _, err := procSetPriorityClass.Call(handle, HIGH_PRIORITY_CLASS)
	if ret == 0 {
		return err
	}
	return nil
}

// SetAboveNormalPriority sets the current process to above normal priority
// A safer option that still gives some priority boost
func SetAboveNormalPriority() error {
	handle, _, _ := procGetCurrentProcess.Call()

	ret, _, err := procSetPriorityClass.Call(handle, ABOVE_NORMAL_PRIORITY_CLASS)
	if ret == 0 {
		return err
	}
	return nil
}

// SetProcessAffinity sets which CPU cores the process can use
// Use all cores by default (mask = all 1s)
func SetProcessAffinity(mask uintptr) error {
	handle, _, _ := procGetCurrentProcess.Call()

	procSetProcessAffinityMask := kernel32.NewProc("SetProcessAffinityMask")
	ret, _, err := procSetProcessAffinityMask.Call(handle, mask)
	if ret == 0 {
		return err
	}
	return nil
}

// DisableProcessorPowerThrottling disables power throttling for the process
// Available on Windows 10 1709+ and Windows 11
func DisableProcessorPowerThrottling() error {
	procSetProcessInformation := kernel32.NewProc("SetProcessInformation")

	handle, _, _ := procGetCurrentProcess.Call()

	// ProcessPowerThrottling = 4
	const ProcessPowerThrottling = 4

	// PROCESS_POWER_THROTTLING_STATE structure
	type PROCESS_POWER_THROTTLING_STATE struct {
		Version     uint32
		ControlMask uint32
		StateMask   uint32
	}

	const PROCESS_POWER_THROTTLING_EXECUTION_SPEED = 0x1

	state := PROCESS_POWER_THROTTLING_STATE{
		Version:     1,
		ControlMask: PROCESS_POWER_THROTTLING_EXECUTION_SPEED,
		StateMask:   0, // 0 = disable throttling
	}

	ret, _, err := procSetProcessInformation.Call(
		handle,
		ProcessPowerThrottling,
		uintptr(unsafe.Pointer(&state)),
		unsafe.Sizeof(state),
	)

	if ret == 0 {
		return err
	}
	return nil
}

// init automatically sets high priority and disables power throttling when the program starts
func init() {
	// Try to set high priority, fall back to above normal if it fails
	if err := SetHighPriority(); err != nil {
		_ = SetAboveNormalPriority()
	}

	// Disable Windows power throttling (Efficiency Mode)
	_ = DisableProcessorPowerThrottling()
}
