#!/bin/bash
# Build script for compiling Vectorscan to WebAssembly
# Runs inside Docker container with Emscripten toolchain

set -e

# Configuration
VS_SOURCE="${VS_SOURCE:-/src/vectorscan}"
WRAPPER_SOURCE="${WRAPPER_SOURCE:-/src/wrapper}"
OUTPUT_DIR="${OUTPUT_DIR:-/output}"
PATCH_FILE="${PATCH_FILE:-/src/patches/vectorscan-wasm.patch}"
BUILD_DIR="/build/vectorscan-wasm"

echo "=== Vectorscan WASM Build ==="
echo "Source: ${VS_SOURCE}"
echo "Wrapper: ${WRAPPER_SOURCE}"
echo "Output: ${OUTPUT_DIR}"
echo ""

# Verify source exists
if [ ! -d "$VS_SOURCE" ]; then
    echo "ERROR: Vectorscan source not found at $VS_SOURCE"
    exit 1
fi

# Apply WASM patch if needed
if [ -f "$PATCH_FILE" ]; then
    if ! grep -q "ARCH_WASM32" "$VS_SOURCE/CMakeLists.txt"; then
        echo "Applying WASM patch to CMakeLists.txt..."
        cd "$VS_SOURCE"
        patch -p1 < "$PATCH_FILE" || echo "Patch may already be applied"
        cd /build
    else
        echo "WASM patches already present"
    fi
fi

# Ensure SIMDe submodule is present
if [ ! -f "$VS_SOURCE/include/simde/simde/simde-common.h" ]; then
    echo "Initializing SIMDe submodule..."
    cd "$VS_SOURCE"
    git submodule update --init --recursive include/simde 2>/dev/null || true
fi

# Create build directory
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"
cd "$BUILD_DIR"

echo "=== Configuring with CMake ==="

# Run CMake with Emscripten toolchain
emcmake cmake "$VS_SOURCE" \
    -DCMAKE_BUILD_TYPE=Release \
    -DBUILD_SHARED_LIBS=OFF \
    -DFAT_RUNTIME=OFF \
    -DBUILD_AVX2=OFF \
    -DBUILD_AVX512=OFF \
    -DBUILD_AVX512VBMI=OFF \
    -DCMAKE_C_FLAGS="-O3 -Wno-error=pass-failed -Wno-pass-failed" \
    -DCMAKE_CXX_FLAGS="-O3 -Wno-error=pass-failed -Wno-pass-failed" \
    -DBOOST_ROOT=/opt/boost \
    -DBoost_INCLUDE_DIR=/opt/boost

echo ""
echo "=== Building Vectorscan ==="

# Build the library
emmake make -j$(nproc) hs 2>&1 | tail -20

# Verify library was built
if [ ! -f "$BUILD_DIR/lib/libhs.a" ]; then
    echo "ERROR: libhs.a not found after build"
    exit 1
fi

echo ""
echo "=== Building WASM module ==="

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Build the WASM module with wrapper
if [ -f "$WRAPPER_SOURCE/matcher.c" ]; then
    emcc -O3 \
        -I"$VS_SOURCE/src" \
        -I"$BUILD_DIR" \
        "$WRAPPER_SOURCE/matcher.c" \
        "$BUILD_DIR/lib/libhs.a" \
        -o "$OUTPUT_DIR/matcher.wasm" \
        -s WASM=1 \
        -s STANDALONE_WASM=1 \
        --no-entry \
        -s EXPORTED_FUNCTIONS='["_wasm_alloc","_wasm_free","_matcher_init","_matcher_match","_matcher_pattern_count","_matcher_close","_malloc","_free"]' \
        -s ERROR_ON_UNDEFINED_SYMBOLS=0 \
        -s TOTAL_MEMORY=67108864 \
        -s ALLOW_MEMORY_GROWTH=1

    if [ -f "$OUTPUT_DIR/matcher.wasm" ]; then
        SIZE=$(ls -lh "$OUTPUT_DIR/matcher.wasm" | awk '{print $5}')
        echo ""
        echo "=== Build Complete ==="
        echo "Output: $OUTPUT_DIR/matcher.wasm ($SIZE)"
    else
        echo "ERROR: Failed to create matcher.wasm"
        exit 1
    fi
else
    echo "WARNING: No wrapper source found at $WRAPPER_SOURCE/matcher.c"
    echo "Only built libhs.a library"
    # Copy the library for inspection
    cp "$BUILD_DIR/lib/libhs.a" "$OUTPUT_DIR/"
    echo "Output: $OUTPUT_DIR/libhs.a"
fi
