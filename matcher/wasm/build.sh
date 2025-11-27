#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

VS_DIR="$SCRIPT_DIR/vectorscan"
BUILD_DIR="$VS_DIR/build-wasm"
OUT_DIR="$SCRIPT_DIR/out"

echo "=== Building Vectorscan for WASM ==="

# Clean previous build
rm -rf "$BUILD_DIR" "$OUT_DIR"
mkdir -p "$BUILD_DIR" "$OUT_DIR"

# Configure Vectorscan with Emscripten
cd "$BUILD_DIR"
echo "Configuring with emcmake..."
# Copy boost headers inline if needed (symlink doesn't work well with emscripten)
if [ ! -d "$VS_DIR/include/boost/graph" ]; then
    echo "Copying Boost headers..."
    rm -rf "$VS_DIR/include/boost"
    cp -r /opt/homebrew/opt/boost/include/boost "$VS_DIR/include/"
fi

emcmake cmake .. \
    -DCMAKE_BUILD_TYPE=Release \
    -DFAT_RUNTIME=OFF \
    -DBUILD_SHARED_LIBS=OFF \
    -DBUILD_STATIC_LIBS=ON \
    -DBUILD_AVX2=OFF \
    -DBUILD_AVX512=OFF \
    -DBUILD_AVX512VBMI=OFF \
    -DCMAKE_FIND_ROOT_PATH_MODE_PACKAGE=BOTH \
    -DCMAKE_FIND_ROOT_PATH_MODE_INCLUDE=BOTH

# Build Vectorscan library
echo "Building with emmake..."
emmake make -j$(nproc 2>/dev/null || sysctl -n hw.ncpu) hs

# Build our WASM wrapper
echo "Building WASM wrapper..."
cd "$SCRIPT_DIR"
emcc -O3 \
    -I"$VS_DIR/src" \
    -I"$BUILD_DIR" \
    -L"$BUILD_DIR/lib" \
    src/matcher.c \
    "$BUILD_DIR/lib/libhs.a" \
    -o "$OUT_DIR/matcher.wasm" \
    -s WASM=1 \
    -s STANDALONE_WASM=1 \
    --no-entry \
    -s EXPORTED_FUNCTIONS='["_wasm_alloc","_wasm_free","_matcher_init","_matcher_match","_matcher_pattern_count","_matcher_close","_malloc","_free"]' \
    -s ERROR_ON_UNDEFINED_SYMBOLS=0

echo "=== Build complete ==="
echo "Output: $OUT_DIR/matcher.wasm"
ls -la "$OUT_DIR/"

# Copy to host directory for go:embed
cp "$OUT_DIR/matcher.wasm" "$SCRIPT_DIR/host/"
echo "Copied to host directory for Go embed"
