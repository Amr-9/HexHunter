#!/bin/bash
# Build script for Linux/macOS
# Usage: ./build.sh

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${CYAN}Building HexHunter...${NC}"

# Detect OS
OS=$(uname -s)
ARCH=$(uname -m)

case "$OS" in
    Linux)
        GOOS="linux"
        OUTPUT="HexHunter"
        echo -e "${CYAN}Detected: Linux ($ARCH)${NC}"
        ;;
    Darwin)
        GOOS="darwin"
        OUTPUT="HexHunter"
        echo -e "${CYAN}Detected: macOS ($ARCH)${NC}"
        ;;
    *)
        echo -e "${RED}Unsupported OS: $OS${NC}"
        exit 1
        ;;
esac

# Set GOARCH
case "$ARCH" in
    x86_64|amd64)
        GOARCH="amd64"
        ;;
    arm64|aarch64)
        GOARCH="arm64"
        ;;
    *)
        echo -e "${YELLOW}Unknown architecture: $ARCH, defaulting to amd64${NC}"
        GOARCH="amd64"
        ;;
esac

# Download dependencies
echo -e "${YELLOW}Downloading dependencies...${NC}"
go mod download
if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to download dependencies!${NC}"
    exit 1
fi

# Tidy modules
echo -e "${YELLOW}Tidying modules...${NC}"
go mod tidy
if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to tidy modules!${NC}"
    exit 1
fi

# Set environment variables
export GOOS="$GOOS"
export GOARCH="$GOARCH"
export CGO_ENABLED="1"

# Check for OpenCL support
OPENCL_AVAILABLE=false

if [ "$GOOS" = "linux" ]; then
    # Check for OpenCL on Linux
    if [ -f "/usr/lib/libOpenCL.so" ] || [ -f "/usr/lib64/libOpenCL.so" ] || [ -f "/usr/lib/x86_64-linux-gnu/libOpenCL.so" ]; then
        OPENCL_AVAILABLE=true
        echo -e "${GREEN}OpenCL library found${NC}"
    fi
elif [ "$GOOS" = "darwin" ]; then
    # macOS has OpenCL framework built-in (but deprecated on Apple Silicon)
    if [ -d "/System/Library/Frameworks/OpenCL.framework" ]; then
        OPENCL_AVAILABLE=true
        echo -e "${GREEN}OpenCL framework found${NC}"
        if [ "$GOARCH" = "arm64" ]; then
            echo -e "${YELLOW}Warning: OpenCL is deprecated on Apple Silicon. GPU support may be limited.${NC}"
        fi
    fi
fi

# Build the application
echo -e "${YELLOW}Building application ($OUTPUT)...${NC}"

if [ "$OPENCL_AVAILABLE" = true ]; then
    # Try building with OpenCL tags
    go build -tags opencl -trimpath -ldflags="-s -w" -o "$OUTPUT" ./cmd/hexhunter
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}Build successful with GPU support!${NC}"
    else
        echo -e "${YELLOW}OpenCL build failed, falling back to CPU-only...${NC}"
        export CGO_ENABLED="0"
        go build -trimpath -ldflags="-s -w" -o "$OUTPUT" ./cmd/hexhunter
        
        if [ $? -eq 0 ]; then
            echo -e "${GREEN}CPU-only build successful!${NC}"
            echo -e "${YELLOW}Note: No GPU acceleration available${NC}"
        else
            echo -e "${RED}Build failed completely!${NC}"
            exit 1
        fi
    fi
else
    echo -e "${YELLOW}OpenCL not found, building CPU-only version...${NC}"
    export CGO_ENABLED="0"
    go build -trimpath -ldflags="-s -w" -o "$OUTPUT" ./cmd/hexhunter
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}CPU-only build successful!${NC}"
        echo -e "${YELLOW}To enable GPU support, install OpenCL:${NC}"
        if [ "$GOOS" = "linux" ]; then
            echo -e "${YELLOW}  Ubuntu/Debian: sudo apt install ocl-icd-opencl-dev${NC}"
            echo -e "${YELLOW}  Fedora: sudo dnf install ocl-icd-devel${NC}"
            echo -e "${YELLOW}  Arch: sudo pacman -S ocl-icd${NC}"
        fi
    else
        echo -e "${RED}Build failed!${NC}"
        exit 1
    fi
fi

# Show output info
echo -e "${YELLOW}Output: $OUTPUT${NC}"

if [ -f "$OUTPUT" ]; then
    SIZE=$(ls -lh "$OUTPUT" | awk '{print $5}')
    echo -e "${YELLOW}File size: $SIZE${NC}"
    
    # Make executable
    chmod +x "$OUTPUT"
    echo -e "${GREEN}Ready to run: ./$OUTPUT${NC}"
fi
