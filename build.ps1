# build.ps1 - Build script for Ethereum Vanity Address Generator
# Creates a single executable with GPU support using TDM-GCC
# ./build.ps1
param(
    [switch]$CPU,       # Build CPU-only version
    [switch]$Debug,     # Include debug symbols
    [switch]$Clean      # Clean build cache first
)

$ErrorActionPreference = "Stop"

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Ethereum zz Address Generator" -ForegroundColor Cyan  
Write-Host "  Build Script v1.0" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Verify Go is installed
try {
    $goVersion = go version
    Write-Host "[OK] $goVersion" -ForegroundColor Green
} catch {
    Write-Host "[ERROR] Go is not installed or not in PATH" -ForegroundColor Red
    exit 1
}

# Verify GCC is installed (for GPU build)
if (-not $CPU) {
    try {
        $gccVersion = gcc --version | Select-Object -First 1
        Write-Host "[OK] $gccVersion" -ForegroundColor Green
    } catch {
        Write-Host "[ERROR] GCC (TDM-GCC) is not installed or not in PATH" -ForegroundColor Red
        Write-Host "       GPU build requires TDM-GCC. Use -CPU flag for CPU-only build." -ForegroundColor Yellow
        exit 1
    }
}

# Clean if requested  
if ($Clean) {
    Write-Host ""
    Write-Host "Cleaning build cache..." -ForegroundColor Yellow
    go clean -cache
}

# Set environment variables
$env:CGO_ENABLED = "1"
$env:CC = "gcc"
$env:GOOS = "windows"
$env:GOARCH = "amd64"

# Build flags
$ldflags = "-s -w"  # Strip debug info for smaller binary

if ($Debug) {
    $ldflags = ""
    Write-Host "[DEBUG] Debug build enabled" -ForegroundColor Yellow
}

# Determine build type
if ($CPU) {
    Write-Host ""
    Write-Host "Building CPU-only version..." -ForegroundColor Yellow
    Write-Host ""
    
    $outputName = "zz-cpu.exe"
    go build -ldflags "$ldflags" -o $outputName .
    
} else {
    Write-Host ""
    Write-Host "Building with GPU (OpenCL) support..." -ForegroundColor Yellow
    Write-Host ""
    
    # Check if deps files exist
    $headerPath = "deps\opencl-headers\CL\cl.h"
    $libPath = "deps\lib\libOpenCL.a"
    
    if (-not (Test-Path $headerPath)) {
        Write-Host "[ERROR] OpenCL headers not found at $headerPath" -ForegroundColor Red
        Write-Host "        Please ensure deps/opencl-headers/CL/ contains the OpenCL headers" -ForegroundColor Yellow
        exit 1
    }
    
    if (-not (Test-Path $libPath)) {
        Write-Host "[ERROR] libOpenCL.a not found at $libPath" -ForegroundColor Red
        Write-Host "        Please ensure deps/lib/ contains libOpenCL.a" -ForegroundColor Yellow
        exit 1
    }
    
    Write-Host "[OK] OpenCL headers found" -ForegroundColor Green
    Write-Host "[OK] libOpenCL.a found" -ForegroundColor Green
    Write-Host ""
    
    $outputName = "zz1.exe"
    go build -tags opencl -ldflags "$ldflags" -o $outputName .
}

# Check if build succeeded
if ($LASTEXITCODE -eq 0) {
    $fileInfo = Get-Item $outputName
    $sizeMB = [math]::Round($fileInfo.Length / 1MB, 2)
    
    Write-Host ""
    Write-Host "========================================" -ForegroundColor Green
    Write-Host "  BUILD SUCCESSFUL!" -ForegroundColor Green
    Write-Host "========================================" -ForegroundColor Green
    Write-Host ""
    Write-Host "  Output: $outputName" -ForegroundColor White
    Write-Host "  Size:   $sizeMB MB" -ForegroundColor White
    Write-Host ""
    
    if (-not $CPU) {
        Write-Host "  Note: Users need GPU drivers installed for GPU mode." -ForegroundColor Yellow
        Write-Host "        CPU mode is always available as fallback." -ForegroundColor Yellow
    }
    Write-Host ""
} else {
    Write-Host ""
    Write-Host "[ERROR] Build failed!" -ForegroundColor Red
    exit 1
}
    