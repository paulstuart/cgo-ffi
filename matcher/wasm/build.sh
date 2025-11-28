#!/bin/bash
# Build Vectorscan for WASM with multiple variants
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

VS_DIR="$SCRIPT_DIR/vectorscan"
OUT_DIR="$SCRIPT_DIR/out"

# Default variant to build (all, simd, nosimd)
VARIANT="${1:-all}"

echo "=== Building Vectorscan for WASM (variant: $VARIANT) ==="

# Copy boost headers inline if needed
if [ ! -d "$VS_DIR/include/boost/graph" ]; then
    echo "Copying Boost headers..."
    rm -rf "$VS_DIR/include/boost"
    if [ -d "/opt/homebrew/opt/boost/include/boost" ]; then
        cp -r /opt/homebrew/opt/boost/include/boost "$VS_DIR/include/"
    elif [ -d "/usr/include/boost" ]; then
        cp -r /usr/include/boost "$VS_DIR/include/"
    fi
fi

# Function to build a variant
build_variant() {
    local name="$1"
    local opt_level="$2"
    local simd_flag="$3"
    local extra_flags="$4"

    local build_dir="$VS_DIR/build-$name"

    echo ""
    echo "=== Building variant: $name ==="
    echo "  Optimization: $opt_level"
    echo "  SIMD: $simd_flag"
    echo ""

    # Clean and create build directory
    rm -rf "$build_dir"
    mkdir -p "$build_dir"

    # Configure Vectorscan with Emscripten
    cd "$build_dir"

    # Use -fwasm-exceptions for native WASM exception handling
    local cmake_cflags="$opt_level $simd_flag -fwasm-exceptions -Wno-error=pass-failed -Wno-pass-failed $extra_flags"

    emcmake cmake .. \
        -DCMAKE_BUILD_TYPE=Release \
        -DFAT_RUNTIME=OFF \
        -DBUILD_SHARED_LIBS=OFF \
        -DBUILD_STATIC_LIBS=ON \
        -DBUILD_AVX2=OFF \
        -DBUILD_AVX512=OFF \
        -DBUILD_AVX512VBMI=OFF \
        -DCMAKE_C_FLAGS="$cmake_cflags" \
        -DCMAKE_CXX_FLAGS="$cmake_cflags" \
        -DCMAKE_FIND_ROOT_PATH_MODE_PACKAGE=BOTH \
        -DCMAKE_FIND_ROOT_PATH_MODE_INCLUDE=BOTH

    # Build Vectorscan library
    emmake make -j$(nproc 2>/dev/null || sysctl -n hw.ncpu) hs 2>&1 | tail -10

    # Build WASM wrapper
    cd "$SCRIPT_DIR"
    mkdir -p "$OUT_DIR"

    local wasm_file="$OUT_DIR/matcher-${name}.wasm"

    emcc $opt_level $simd_flag \
        -I"$VS_DIR/src" \
        -I"$build_dir" \
        -fwasm-exceptions \
        src/matcher.cpp \
        "$build_dir/lib/libhs.a" \
        -o "$wasm_file" \
        -s WASM=1 \
        -s STANDALONE_WASM=1 \
        --no-entry \
        -s EXPORTED_FUNCTIONS='["_wasm_alloc","_wasm_free","_matcher_init","_matcher_match","_matcher_pattern_count","_matcher_close","_matcher_get_error","_matcher_check_platform","_malloc","_free"]' \
        -s ERROR_ON_UNDEFINED_SYMBOLS=0 \
        -s TOTAL_MEMORY=67108864 \
        -s ALLOW_MEMORY_GROWTH=1

    # Transform legacy exceptions to new exnref format (required by wasmtime v39+)
    echo "  Transforming to exnref format..."
    wasm-opt --all-features \
        --translate-to-exnref --emit-exnref \
        -o "$wasm_file.new" "$wasm_file"
    mv "$wasm_file.new" "$wasm_file"

    local size=$(ls -lh "$wasm_file" | awk '{print $5}')
    echo "  Output: $wasm_file ($size)"
}

# Create output directory
mkdir -p "$OUT_DIR"

# Build variants based on selection
case "$VARIANT" in
    simd)
        # With WASM SIMD (-msimd128)
        build_variant "simd-O3" "-O3" "-msimd128" ""
        cp "$OUT_DIR/matcher-simd-O3.wasm" "$SCRIPT_DIR/host/matcher.wasm"
        ;;
    nosimd)
        # Without SIMD (pure scalar fallback)
        build_variant "nosimd-O3" "-O3" "" ""
        cp "$OUT_DIR/matcher-nosimd-O3.wasm" "$SCRIPT_DIR/host/matcher.wasm"
        ;;
    all)
        # Build all variants
        build_variant "simd-O3" "-O3" "-msimd128" ""
        build_variant "simd-O2" "-O2" "-msimd128" ""
        build_variant "nosimd-O3" "-O3" "" ""
        build_variant "nosimd-O2" "-O2" "" ""

        # Default to nosimd-O3 for now (most compatible)
        cp "$OUT_DIR/matcher-nosimd-O3.wasm" "$SCRIPT_DIR/host/matcher.wasm"
        ;;
    *)
        echo "Usage: $0 [simd|nosimd|all]"
        exit 1
        ;;
esac

echo ""
echo "=== Build complete ==="
ls -la "$OUT_DIR/"
echo ""
echo "Active variant in host/matcher.wasm: $(ls -lh $SCRIPT_DIR/host/matcher.wasm | awk '{print $5}')"
