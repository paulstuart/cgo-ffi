# Makefile for Vectorscan WASM build
# Supports both local (with Emscripten) and Docker builds

SCRIPT_DIR := $(shell pwd)
VS_DIR := $(SCRIPT_DIR)/vectorscan
OUT_DIR := $(SCRIPT_DIR)/out
HOST_DIR := $(SCRIPT_DIR)/host

# Docker image name
DOCKER_IMAGE := vectorscan-wasm-builder

# Build variants
VARIANT ?= nosimd
OPT_LEVEL ?= -O3
SIMD_FLAG := $(if $(filter simd,$(VARIANT)),-msimd128,)

# Emscripten flags
CMAKE_FLAGS := \
	-DCMAKE_BUILD_TYPE=Release \
	-DFAT_RUNTIME=OFF \
	-DBUILD_SHARED_LIBS=OFF \
	-DBUILD_STATIC_LIBS=ON \
	-DBUILD_AVX2=OFF \
	-DBUILD_AVX512=OFF \
	-DBUILD_AVX512VBMI=OFF

# C/C++ flags - use fexceptions for proper exception handling
CFLAGS := $(OPT_LEVEL) $(SIMD_FLAG) -fexceptions -Wno-error=pass-failed -Wno-pass-failed

# Emcc linker flags
EMCC_FLAGS := \
	-s WASM=1 \
	-s STANDALONE_WASM=1 \
	--no-entry \
	-s EXPORTED_FUNCTIONS='["_wasm_alloc","_wasm_free","_matcher_init","_matcher_match","_matcher_pattern_count","_matcher_close","_matcher_get_error","_matcher_check_platform","_malloc","_free"]' \
	-s ERROR_ON_UNDEFINED_SYMBOLS=0 \
	-s TOTAL_MEMORY=67108864 \
	-s ALLOW_MEMORY_GROWTH=1 \
	-s DISABLE_EXCEPTION_CATCHING=0

.PHONY: all clean wasm wasm-simd wasm-nosimd docker-build docker-wasm test test-wasmtime help

help:
	@echo "Vectorscan WASM Build System"
	@echo ""
	@echo "Targets:"
	@echo "  wasm          - Build WASM module (default: nosimd)"
	@echo "  wasm-simd     - Build with WASM SIMD support"
	@echo "  wasm-nosimd   - Build without SIMD (scalar fallback)"
	@echo "  docker-build  - Build the Docker image"
	@echo "  docker-wasm   - Build WASM inside Docker container"
	@echo "  test          - Run Go tests"
	@echo "  test-wasmtime - Test with wasmtime CLI"
	@echo "  inspect       - Inspect WASM module imports/exports"
	@echo "  clean         - Remove build artifacts"
	@echo ""
	@echo "Variables:"
	@echo "  VARIANT=simd|nosimd  - Build variant (default: nosimd)"
	@echo "  OPT_LEVEL=-O0|-O2|-O3 - Optimization level (default: -O3)"

all: wasm

# Ensure output directory exists
$(OUT_DIR):
	mkdir -p $(OUT_DIR)

# Copy boost headers if needed
$(VS_DIR)/include/boost/graph:
	@echo "Copying Boost headers..."
	@if [ -d "/opt/homebrew/opt/boost/include/boost" ]; then \
		cp -r /opt/homebrew/opt/boost/include/boost $(VS_DIR)/include/; \
	elif [ -d "/usr/include/boost" ]; then \
		cp -r /usr/include/boost $(VS_DIR)/include/; \
	else \
		echo "ERROR: Boost headers not found"; exit 1; \
	fi

# Build Vectorscan library
$(VS_DIR)/build-$(VARIANT)/lib/libhs.a: $(VS_DIR)/include/boost/graph
	@echo "=== Building Vectorscan library ($(VARIANT)) ==="
	rm -rf $(VS_DIR)/build-$(VARIANT)
	mkdir -p $(VS_DIR)/build-$(VARIANT)
	cd $(VS_DIR)/build-$(VARIANT) && \
		emcmake cmake .. $(CMAKE_FLAGS) \
			-DCMAKE_C_FLAGS="$(CFLAGS)" \
			-DCMAKE_CXX_FLAGS="$(CFLAGS)" \
			-DCMAKE_FIND_ROOT_PATH_MODE_PACKAGE=BOTH \
			-DCMAKE_FIND_ROOT_PATH_MODE_INCLUDE=BOTH
	cd $(VS_DIR)/build-$(VARIANT) && emmake make -j$$(nproc 2>/dev/null || sysctl -n hw.ncpu) hs

# Build WASM module
$(OUT_DIR)/matcher-$(VARIANT).wasm: $(VS_DIR)/build-$(VARIANT)/lib/libhs.a src/matcher.cpp | $(OUT_DIR)
	@echo "=== Building WASM wrapper ==="
	emcc $(OPT_LEVEL) $(SIMD_FLAG) \
		-I$(VS_DIR)/src \
		-I$(VS_DIR)/build-$(VARIANT) \
		-fexceptions \
		src/matcher.cpp \
		$(VS_DIR)/build-$(VARIANT)/lib/libhs.a \
		-o $@ \
		$(EMCC_FLAGS)
	@echo "Output: $@ ($$(ls -lh $@ | awk '{print $$5}'))"

wasm: $(OUT_DIR)/matcher-$(VARIANT).wasm
	cp $(OUT_DIR)/matcher-$(VARIANT).wasm $(HOST_DIR)/matcher.wasm
	@echo "Copied to $(HOST_DIR)/matcher.wasm"

wasm-simd:
	$(MAKE) wasm VARIANT=simd

wasm-nosimd:
	$(MAKE) wasm VARIANT=nosimd

# Docker build
docker-build:
	docker build -t $(DOCKER_IMAGE) -f Dockerfile .

docker-wasm: docker-build
	docker run --rm -v $(SCRIPT_DIR):/workspace $(DOCKER_IMAGE) make wasm VARIANT=$(VARIANT)

# Testing
test:
	cd $(HOST_DIR) && go test -v ./...

test-wasmtime: $(OUT_DIR)/matcher-$(VARIANT).wasm
	@echo "=== Testing with wasmtime ==="
	@echo "Inspecting module..."
	wasmtime explore $(OUT_DIR)/matcher-$(VARIANT).wasm || true
	@echo ""
	@echo "Module exports:"
	wasm-objdump -x $(OUT_DIR)/matcher-$(VARIANT).wasm | grep -A 50 "Export\[" | head -30

inspect: $(OUT_DIR)/matcher-$(VARIANT).wasm
	@echo "=== WASM Module Info ==="
	@echo "Size: $$(ls -lh $(OUT_DIR)/matcher-$(VARIANT).wasm | awk '{print $$5}')"
	@echo ""
	@echo "Imports:"
	wasm-objdump -x $(OUT_DIR)/matcher-$(VARIANT).wasm | grep -E "func\[.*<" | head -20
	@echo ""
	@echo "Exports:"
	wasm-objdump -x $(OUT_DIR)/matcher-$(VARIANT).wasm | grep -A 30 "Export\[" | head -35

clean:
	rm -rf $(OUT_DIR)
	rm -rf $(VS_DIR)/build-*
	rm -f $(HOST_DIR)/matcher.wasm
