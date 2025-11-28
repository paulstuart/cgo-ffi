# Dockerfile for building Vectorscan to WebAssembly
# Contains: Emscripten, CMake, Boost headers, Ragel, and build dependencies

FROM emscripten/emsdk:3.1.50

# TODO: perhaps mount host cwd as container cwd read only, and a second dir as writeable for the build artifact(s)
#.      although just cwd:cwd  normal make the interface  as simple as can be

# Install build dependencies
RUN apt-get update && apt-get install -y \
    cmake \
    ragel \
    python3 \
    pkg-config \
    patch \
    git \
    && rm -rf /var/lib/apt/lists/*

# Download and install Boost headers (header-only, no build needed)
ARG BOOST_VERSION=1.83.0
ARG BOOST_VERSION_UNDERSCORE=1_83_0
RUN curl -L https://boostorg.jfrog.io/artifactory/main/release/${BOOST_VERSION}/source/boost_${BOOST_VERSION_UNDERSCORE}.tar.gz \
    | tar xz -C /opt \
    && mv /opt/boost_${BOOST_VERSION_UNDERSCORE} /opt/boost

# Set up Boost include path
ENV BOOST_ROOT=/opt/boost
ENV BOOST_INCLUDEDIR=/opt/boost

WORKDIR /build

# Copy the build script
COPY build-wasm.sh /usr/local/bin/build-wasm.sh
RUN chmod +x /usr/local/bin/build-wasm.sh

# Default command runs the build
ENTRYPOINT ["/usr/local/bin/build-wasm.sh"]
