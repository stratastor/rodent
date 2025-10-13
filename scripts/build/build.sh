#!/bin/bash
# Rodent Build Script
# Copyright 2025 The StrataSTOR Authors and Contributors
# SPDX-License-Identifier: Apache-2.0

set -e
set -o pipefail

# Colors
RED='\033[38;2;255;183;178m'
GREEN='\033[38;2;152;251;152m'
YELLOW='\033[38;2;230;230;250m'
BLUE='\033[38;2;176;224;230m'
NC='\033[0m'

# Default values
VERSION="${VERSION:-dev}"
COMMIT_SHA="${COMMIT_SHA:-$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')}"
BUILD_TIME="${BUILD_TIME:-$(date -u '+%Y-%m-%dT%H:%M:%SZ')}"
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-amd64}"
OUTPUT_DIR="${OUTPUT_DIR:-dist}"

print_info() {
    echo -e "${BLUE}ℹ${NC} $@"
}

print_success() {
    echo -e "${GREEN}✓${NC} $@"
}

print_error() {
    echo -e "${RED}✗${NC} $@" >&2
}

print_header() {
    echo -e "${BLUE}================================================${NC}"
    echo -e "${BLUE}  Building Rodent${NC}"
    echo -e "${BLUE}================================================${NC}"
    echo ""
}

# Get project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$PROJECT_ROOT"

print_header

print_info "Build configuration:"
print_info "  Version: $VERSION"
print_info "  Commit: $COMMIT_SHA"
print_info "  Build Time: $BUILD_TIME"
print_info "  OS: $GOOS"
print_info "  Arch: $GOARCH"
print_info "  Output: $OUTPUT_DIR"
echo ""

# Check Go version
print_info "Checking Go version..."
if ! command -v go &> /dev/null; then
    print_error "Go is not installed"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
print_success "Go version: $GO_VERSION"

# Check if version is 1.25.2 or higher
REQUIRED_GO_VERSION="1.25.2"
if ! printf '%s\n%s\n' "$REQUIRED_GO_VERSION" "$GO_VERSION" | sort -V -C; then
    print_error "Go $REQUIRED_GO_VERSION or higher is required. Found: $GO_VERSION"
    exit 1
fi

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Build binary
print_info "Building binary..."

OUTPUT_FILE="$OUTPUT_DIR/rodent-${GOOS}-${GOARCH}"
if [ "$GOOS" = "windows" ]; then
    OUTPUT_FILE="${OUTPUT_FILE}.exe"
fi

# Build flags
LDFLAGS=(
    "-s -w"  # Strip debug info and symbol table
    "-X 'github.com/stratastor/rodent/internal/constants.Version=${VERSION}'"
    "-X 'github.com/stratastor/rodent/internal/constants.CommitSHA=${COMMIT_SHA}'"
    "-X 'github.com/stratastor/rodent/internal/constants.BuildTime=${BUILD_TIME}'"
)

# Build command
CGO_ENABLED=0 \
GOOS="$GOOS" \
GOARCH="$GOARCH" \
go build \
    -trimpath \
    -ldflags="${LDFLAGS[*]}" \
    -o "$OUTPUT_FILE" \
    .

if [ ! -f "$OUTPUT_FILE" ]; then
    print_error "Build failed: output file not found"
    exit 1
fi

# Get file size
FILE_SIZE=$(du -h "$OUTPUT_FILE" | cut -f1)

print_success "Build completed successfully"
print_info "Output: $OUTPUT_FILE"
print_info "Size: $FILE_SIZE"

# Generate checksum
print_info "Generating checksum..."
cd "$OUTPUT_DIR"
sha256sum "$(basename "$OUTPUT_FILE")" > "$(basename "$OUTPUT_FILE").sha256"
print_success "Checksum saved to $(basename "$OUTPUT_FILE").sha256"

# Show checksum
print_info "SHA256: $(cat "$(basename "$OUTPUT_FILE").sha256" | awk '{print $1}')"

echo ""
print_success "Build process completed"
