# Build script for Windows (PowerShell)
# Usage: .\build.ps1

Write-Host "Building for Windows (amd64)..." -ForegroundColor Cyan

# Download dependencies first
Write-Host "Downloading dependencies..." -ForegroundColor Yellow
go mod download
if ($LASTEXITCODE -ne 0) {
    Write-Host "Failed to download dependencies!" -ForegroundColor Red
    exit 1
}

# Tidy modules
Write-Host "Tidying modules..." -ForegroundColor Yellow
go mod tidy
if ($LASTEXITCODE -ne 0) {
    Write-Host "Failed to tidy modules!" -ForegroundColor Red
    exit 1
}

# Set environment variables for GPU build (OpenCL)
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "1"
$env:CC = "gcc"

# Compile Windows resources (manifest, icon, version info)
Write-Host "Compiling Windows resources..." -ForegroundColor Yellow
$resourceDir = "cmd\hexhunter"
$sysoFile = "$resourceDir\resource.syso"

# Check if windres is available
$windresPath = Get-Command windres -ErrorAction SilentlyContinue
if ($windresPath) {
    Push-Location $resourceDir
    windres -o resource.syso resource.rc 2>$null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Resources compiled successfully!" -ForegroundColor Green
    } else {
        Write-Host "Resource compilation failed, building without manifest..." -ForegroundColor Yellow
    }
    Pop-Location
} else {
    Write-Host "windres not found, building without manifest embedding..." -ForegroundColor Yellow
    Write-Host "To embed manifest, install MinGW-w64 and add to PATH" -ForegroundColor Gray
}

# Build the application
Write-Host "Building application (HexHunter.exe)..." -ForegroundColor Yellow

# Try building with OpenCL tags
go build -tags opencl -trimpath -ldflags="-s -w" -o HexHunter.exe ./cmd/hexhunter

if ($LASTEXITCODE -eq 0) {
    Write-Host "Build successful!" -ForegroundColor Green
    Write-Host "Output: HexHunter.exe" -ForegroundColor Yellow
    
    # Show file info
    if (Test-Path "HexHunter.exe") {
        $fileInfo = Get-Item "HexHunter.exe"
        $sizeMB = [math]::Round($fileInfo.Length / 1MB, 2)
        Write-Host "File size: $sizeMB MB" -ForegroundColor Yellow
    }
} else {
    Write-Host "Build failed! Checking if GCC/OpenCL is missing..." -ForegroundColor Red
    Write-Host "Attempting CPU-only build..." -ForegroundColor Yellow
    
    # Fallback to CPU only
    $env:CGO_ENABLED = "0"
    go build -trimpath -ldflags="-s -w" -o HexHunter.exe ./cmd/hexhunter
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "CPU-only Build successful!" -ForegroundColor Green
        Write-Host "Output: HexHunter.exe (No GPU support)" -ForegroundColor Yellow
    } else {
        Write-Host "Build failed completely!" -ForegroundColor Red
        exit 1
    }
}