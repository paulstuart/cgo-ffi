// Package wasmvs provides CGO helper for enabling WASM exceptions
package wasmvs

/*
#cgo CFLAGS:-I${SRCDIR}/../../../../../../../go/pkg/mod/github.com/bytecodealliance/wasmtime-go/v39@v39.0.1/build/include
#cgo !windows LDFLAGS:-lwasmtime -lm -ldl -pthread
#cgo darwin,amd64 LDFLAGS:-L${SRCDIR}/../../../../../../../go/pkg/mod/github.com/bytecodealliance/wasmtime-go/v39@v39.0.1/build/macos-x86_64
#cgo darwin,arm64 LDFLAGS:-L${SRCDIR}/../../../../../../../go/pkg/mod/github.com/bytecodealliance/wasmtime-go/v39@v39.0.1/build/macos-aarch64
#cgo linux,amd64 LDFLAGS:-L${SRCDIR}/../../../../../../../go/pkg/mod/github.com/bytecodealliance/wasmtime-go/v39@v39.0.1/build/linux-x86_64
#cgo linux,arm64 LDFLAGS:-L${SRCDIR}/../../../../../../../go/pkg/mod/github.com/bytecodealliance/wasmtime-go/v39@v39.0.1/build/linux-aarch64

#include <wasm.h>
#include <wasmtime.h>

// Enable WASM exception handling (not exposed in wasmtime-go v39)
static void enable_exceptions_on_config(wasm_config_t* cfg) {
    wasmtime_config_wasm_exceptions_set(cfg, true);
    wasmtime_config_wasm_gc_set(cfg, true);  // Exceptions require GC
}
*/
import "C"

import (
	"reflect"
	"unsafe"

	"github.com/bytecodealliance/wasmtime-go/v39"
)

// enableExceptions enables WASM exception handling on a Config via reflection
func enableExceptions(cfg *wasmtime.Config) {
	// Access the private _ptr field via reflection
	v := reflect.ValueOf(cfg).Elem()
	ptrField := v.FieldByName("_ptr")
	if !ptrField.IsValid() {
		return
	}
	// Get the unsafe pointer and convert to C type
	ptr := unsafe.Pointer(ptrField.Pointer())
	C.enable_exceptions_on_config((*C.wasm_config_t)(ptr))
}
