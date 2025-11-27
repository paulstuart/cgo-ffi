#!/bin/bash
# Build all WASM modules
#
# Prerequisites:
# - Rust with wasm32-unknown-unknown target: rustup target add wasm32-unknown-unknown
# - TinyGo: https://tinygo.org/getting-started/install/
# - wasi-sdk or Emscripten for C: https://github.com/WebAssembly/wasi-sdk
#
# Usage: ./build.sh [rust|tinygo|c|all]

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Track build results for 'all' mode
BUILT=""
SKIPPED=""

build_rust() {
    echo "=== Building Rust WASM ==="

    if ! command -v rustup &> /dev/null; then
        echo "SKIP: Rust/rustup not found. Install from https://rustup.rs/"
        return 1
    fi

    cd rust

    # Ensure target is installed
    if ! rustup target list --installed | grep -q wasm32-unknown-unknown; then
        echo "Installing wasm32-unknown-unknown target..."
        rustup target add wasm32-unknown-unknown
    fi

    # Enable SIMD and other optimizations via RUSTFLAGS
    export RUSTFLAGS="-C target-feature=+simd128"

    if ! cargo build --release --target wasm32-unknown-unknown; then
        echo "ERROR: Rust build failed"
        cd ..
        return 1
    fi

    # Copy to expected location
    cp target/wasm32-unknown-unknown/release/vector_wasm.wasm vector.wasm

    # Run wasm-opt if available for additional optimization
    if command -v wasm-opt &> /dev/null; then
        echo "Running wasm-opt..."
        wasm-opt -O3 --enable-simd vector.wasm -o vector.wasm
    fi

    echo "Rust WASM built: rust/vector.wasm ($(stat -f%z vector.wasm 2>/dev/null || stat -c%s vector.wasm 2>/dev/null) bytes)"
    cd ..
    return 0
}

build_tinygo() {
    echo "=== Building TinyGo WASM ==="

    if ! command -v tinygo &> /dev/null; then
        echo "SKIP: TinyGo not found. Install from https://tinygo.org/getting-started/install/"
        return 1
    fi

    cd tinygo

    # Optimization flags:
    # -opt=2        : Maximum optimization
    # -no-debug     : No debug info
    # -scheduler=none : No goroutine scheduler (we don't use goroutines)
    # -gc=leaking   : No GC (we never free memory)
    if ! tinygo build -o vector.wasm -target=wasi -opt=2 -no-debug -scheduler=none -gc=leaking main.go; then
        echo "ERROR: TinyGo build failed"
        cd ..
        return 1
    fi

    # Run wasm-opt if available for additional optimization
    if command -v wasm-opt &> /dev/null; then
        echo "Running wasm-opt..."
        wasm-opt -O3 vector.wasm -o vector.wasm
    fi

    echo "TinyGo WASM built: tinygo/vector.wasm ($(stat -f%z vector.wasm 2>/dev/null || stat -c%s vector.wasm 2>/dev/null) bytes)"
    cd ..
    return 0
}

build_c() {
    echo "=== Building C WASM ==="

    # Try wasi-sdk first, then Emscripten, then clang with wasm target
    if [ -n "$WASI_SDK_PATH" ] && [ -f "$WASI_SDK_PATH/bin/clang" ]; then
        echo "Using wasi-sdk..."
        cd c
        if "$WASI_SDK_PATH/bin/clang" \
            --target=wasm32-wasi \
            -O3 \
            -nostartfiles \
            -Wl,--no-entry \
            -Wl,--export=sum \
            -Wl,--export=dot \
            -Wl,--export=mul \
            -Wl,--export=scale \
            -Wl,--export=sum_simd \
            -Wl,--export=get_buffer_a_offset \
            -Wl,--export=get_buffer_b_offset \
            -Wl,--export=get_result_offset \
            -Wl,--export=get_capacity \
            -Wl,--export=memory \
            -o vector.wasm \
            vector_wasm.c; then
            echo "C WASM built: c/vector.wasm"
            cd ..
            return 0
        else
            echo "ERROR: wasi-sdk build failed"
            cd ..
            return 1
        fi
    elif command -v emcc &> /dev/null; then
        echo "Using Emscripten..."
        cd c
        if emcc -O3 \
            -s STANDALONE_WASM=1 \
            -s EXPORTED_FUNCTIONS='["_sum","_dot","_mul","_scale","_sum_simd","_get_buffer_a_offset","_get_buffer_b_offset","_get_result_offset","_get_capacity"]' \
            --no-entry \
            -o vector.wasm \
            vector_wasm.c; then
            echo "C WASM built: c/vector.wasm"
            cd ..
            return 0
        else
            echo "ERROR: Emscripten build failed"
            cd ..
            return 1
        fi
    elif command -v clang &> /dev/null && clang --print-targets 2>/dev/null | grep -q wasm32; then
        echo "Using clang with wasm32 target..."
        cd c
        if clang \
            --target=wasm32 \
            -O3 \
            -nostdlib \
            -Wl,--no-entry \
            -Wl,--export-all \
            -o vector.wasm \
            vector_wasm.c; then
            echo "C WASM built: c/vector.wasm"
            cd ..
            return 0
        else
            echo "ERROR: clang build failed"
            cd ..
            return 1
        fi
    else
        echo "SKIP: No WASM C compiler found. Install one of:"
        echo "  wasi-sdk: https://github.com/WebAssembly/wasi-sdk"
        echo "  Emscripten: https://emscripten.org/docs/getting_started/downloads.html"
        return 1
    fi
}

case "${1:-all}" in
    rust)
        build_rust || exit 1
        ;;
    tinygo)
        build_tinygo || exit 1
        ;;
    c)
        build_c || exit 1
        ;;
    all)
        # Build all available toolchains, don't fail on missing ones
        if build_rust; then
            BUILT="$BUILT rust"
        else
            SKIPPED="$SKIPPED rust"
        fi
        echo ""

        if build_tinygo; then
            BUILT="$BUILT tinygo"
        else
            SKIPPED="$SKIPPED tinygo"
        fi
        echo ""

        if build_c; then
            BUILT="$BUILT c"
        else
            SKIPPED="$SKIPPED c"
        fi
        echo ""

        echo "=== Build Summary ==="
        if [ -n "$BUILT" ]; then
            echo "Built:$BUILT"
        fi
        if [ -n "$SKIPPED" ]; then
            echo "Skipped:$SKIPPED"
        fi
        if [ -z "$BUILT" ]; then
            echo "ERROR: No WASM modules were built"
            exit 1
        fi
        ;;
    *)
        echo "Usage: $0 [rust|tinygo|c|all]"
        exit 1
        ;;
esac
