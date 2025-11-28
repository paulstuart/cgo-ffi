# Dockerfile for building Vectorscan WASM
# Uses Emscripten toolchain

FROM emscripten/emsdk:3.1.51

# Install build dependencies
RUN apt-get update && apt-get install -y \
    cmake \
    ragel \
    libboost-dev \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

# Install wasmtime for testing
RUN curl https://wasmtime.dev/install.sh -sSf | bash
ENV PATH="/root/.wasmtime/bin:${PATH}"

# Install wasm tools
RUN npm install -g wabt

WORKDIR /workspace

# Default command
CMD ["make", "wasm"]
