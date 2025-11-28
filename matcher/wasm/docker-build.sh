#!/bin/bash
# Build Vectorscan WASM using Docker
# Usage: ./docker-build.sh
#
# This builds the matcher.wasm module without requiring local tooling.
# All dependencies (Emscripten, Boost, Ragel, CMake) are in the container.

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DOCKER_DIR="$SCRIPT_DIR/docker"
VECTORSCAN_DIR="$SCRIPT_DIR/vectorscan"
WRAPPER_DIR="$SCRIPT_DIR/src"
OUTPUT_DIR="$SCRIPT_DIR/host"

IMAGE_NAME="vectorscan-wasm-builder"
IMAGE_TAG="latest"

echo "=== Docker WASM Build ==="
echo "Script dir: $SCRIPT_DIR"
echo "Vectorscan: $VECTORSCAN_DIR"
echo "Output: $OUTPUT_DIR"
echo ""

# Check if vectorscan source exists
if [ ! -d "$VECTORSCAN_DIR" ]; then
    echo "Vectorscan source not found. Cloning..."
    git clone https://github.com/VectorCamp/vectorscan.git "$VECTORSCAN_DIR"
    cd "$VECTORSCAN_DIR"
    git submodule update --init --recursive
    cd "$SCRIPT_DIR"
fi

# Build Docker image if needed
if ! docker image inspect "$IMAGE_NAME:$IMAGE_TAG" >/dev/null 2>&1; then
    echo "Building Docker image..."
    docker build -t "$IMAGE_NAME:$IMAGE_TAG" "$DOCKER_DIR"
fi

# Run the build
echo "Running WASM build in Docker..."
docker run --rm \
    -v "$VECTORSCAN_DIR:/src/vectorscan" \
    -v "$WRAPPER_DIR:/src/wrapper:ro" \
    -v "$DOCKER_DIR:/src/patches:ro" \
    -v "$OUTPUT_DIR:/output" \
    "$IMAGE_NAME:$IMAGE_TAG"

# Verify output
if [ -f "$OUTPUT_DIR/matcher.wasm" ]; then
    echo ""
    echo "Build successful!"
    ls -lh "$OUTPUT_DIR/matcher.wasm"
else
    echo "Build failed - no output file"
    exit 1
fi
