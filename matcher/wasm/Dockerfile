# Dockerfile for building Vectorscan WASM
# Contains all tools needed: Emscripten, Binaryen, wasmtime
#
# Usage:
#   docker build -t vectorscan-wasm .
#   docker run -v $(pwd):/workspace vectorscan-wasm ./build.sh nosimd
#
# Or interactively:
#   docker run -it -v $(pwd):/workspace vectorscan-wasm bash

FROM emscripten/emsdk:3.1.51

# Install build dependencies
RUN apt-get update && apt-get install -y \
    cmake \
    ragel \
    libboost-dev \
    pkg-config \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Install Binaryen (wasm-opt) for exception handling transformation
# Required to convert legacy exceptions to exnref format for wasmtime
RUN npm install -g binaryen

# Install wasmtime for testing
RUN curl https://wasmtime.dev/install.sh -sSf | bash
ENV PATH="/root/.wasmtime/bin:${PATH}"

# Install wabt tools (wasm2wat, wasm-objdump)
RUN npm install -g wabt

WORKDIR /workspace

# Verify tools are available
RUN wasm-opt --version && wasmtime --version && wasm2wat --version

# Default: run build script
CMD ["./build.sh", "nosimd"]
