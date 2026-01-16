//go:build windows

package main

// This file forces Windows to use the high-performance GPU instead of integrated graphics.
// It exports symbols that NVIDIA and AMD drivers look for to determine GPU preference.
//
// NvOptimusEnablement = 1 forces NVIDIA discrete GPU
// AmdPowerXpressRequestHighPerformance = 1 forces AMD discrete GPU

/*
#include <stdint.h>

// Force NVIDIA Optimus to use discrete GPU
__declspec(dllexport) uint32_t NvOptimusEnablement = 1;

// Force AMD PowerXpress to use discrete GPU
__declspec(dllexport) uint32_t AmdPowerXpressRequestHighPerformance = 1;
*/
import "C"
