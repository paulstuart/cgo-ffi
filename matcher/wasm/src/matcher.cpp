// WASM wrapper for Vectorscan multi-pattern matching
// Exports minimal API for pattern compilation and matching
// Compiled without C++ exceptions - errors handled via return codes

#include <cstdlib>
#include <cstring>
#include <cstdio>
#include <cstdarg>

#include "hs.h"

// Debug output buffer for WASM
static char g_error_msg[512] = {0};

// Global state
static hs_database_t *g_database = nullptr;
static hs_scratch_t *g_scratch = nullptr;
static int g_pattern_count = 0;

// Match result for callback
static int g_match_id = -1;

// Callback for hs_scan - captures first match and terminates
static int match_handler(unsigned int id, unsigned long long from,
                         unsigned long long to, unsigned int flags, void *ctx) {
    g_match_id = static_cast<int>(id);
    return 1;  // Non-zero to stop scanning
}

// Helper to set error message
static void set_error(const char* msg) {
    snprintf(g_error_msg, sizeof(g_error_msg), "%s", msg);
}

static void set_error_fmt(const char* fmt, ...) {
    va_list args;
    va_start(args, fmt);
    vsnprintf(g_error_msg, sizeof(g_error_msg), fmt, args);
    va_end(args);
}

extern "C" {

// Memory allocation exports for WASM host
__attribute__((export_name("wasm_alloc")))
void* wasm_alloc(int size) {
    return malloc(size);
}

__attribute__((export_name("wasm_free")))
void wasm_free(void* ptr) {
    free(ptr);
}

// Initialize matcher with patterns (newline-separated)
// Returns 0 on success, negative on error
__attribute__((export_name("matcher_init")))
int matcher_init(const char* patterns_data, int patterns_len) {
    // Count patterns (number of newlines + 1, or 0 if empty)
    if (patterns_len == 0) {
        set_error("No patterns provided");
        return -1;
    }

    int count = 1;
    for (int i = 0; i < patterns_len; i++) {
        if (patterns_data[i] == '\n') count++;
    }

    // Allocate arrays for pattern data
    const char** expressions = static_cast<const char**>(malloc(count * sizeof(char*)));
    unsigned int* flags = static_cast<unsigned int*>(malloc(count * sizeof(unsigned int)));
    unsigned int* ids = static_cast<unsigned int*>(malloc(count * sizeof(unsigned int)));

    if (!expressions || !flags || !ids) {
        free(expressions);
        free(flags);
        free(ids);
        set_error("Memory allocation failed for pattern arrays");
        return -2;
    }

    // Parse patterns - split on newlines
    char* data_copy = static_cast<char*>(malloc(patterns_len + 1));
    if (!data_copy) {
        free(expressions);
        free(flags);
        free(ids);
        set_error("Memory allocation failed for pattern data");
        return -3;
    }
    memcpy(data_copy, patterns_data, patterns_len);
    data_copy[patterns_len] = '\0';

    int idx = 0;
    char* line = data_copy;
    for (int i = 0; i <= patterns_len && idx < count; i++) {
        if (i == patterns_len || data_copy[i] == '\n') {
            if (i < patterns_len) data_copy[i] = '\0';
            if (line[0] != '\0') {  // Skip empty lines
                expressions[idx] = line;
                flags[idx] = HS_FLAG_CASELESS | HS_FLAG_SINGLEMATCH;
                ids[idx] = idx;
                idx++;
            }
            line = &data_copy[i + 1];
        }
    }

    int actual_count = idx;

    // Compile patterns into database
    hs_compile_error_t *compile_err = nullptr;
    hs_error_t err = hs_compile_multi(expressions, flags, ids, actual_count,
                                      HS_MODE_BLOCK, nullptr, &g_database, &compile_err);

    if (err != HS_SUCCESS) {
        if (compile_err) {
            set_error_fmt("Compile error at pattern %d: %s",
                          compile_err->expression,
                          compile_err->message ? compile_err->message : "unknown");
            hs_free_compile_error(compile_err);
        } else {
            set_error_fmt("hs_compile_multi failed with code %d", err);
        }
        free(data_copy);
        free(expressions);
        free(flags);
        free(ids);
        return -4;
    }

    // Allocate scratch space
    err = hs_alloc_scratch(g_database, &g_scratch);
    if (err != HS_SUCCESS) {
        hs_free_database(g_database);
        g_database = nullptr;
        free(data_copy);
        free(expressions);
        free(flags);
        free(ids);
        set_error_fmt("hs_alloc_scratch failed with code %d", err);
        return -5;
    }

    g_pattern_count = actual_count;

    // Note: we keep data_copy allocated since expressions point into it
    // This is a memory leak but acceptable for our benchmark use case
    free(expressions);
    free(flags);
    free(ids);

    return 0;
}

// Match input against all patterns
// Returns first matching pattern ID, or -1 if no match
__attribute__((export_name("matcher_match")))
int matcher_match(const char* input, int input_len) {
    if (!g_database || !g_scratch) return -1;

    g_match_id = -1;

    hs_error_t err = hs_scan(g_database, input, input_len, 0,
                             g_scratch, match_handler, nullptr);

    // HS_SCAN_TERMINATED means we found a match and stopped early
    if (err != HS_SUCCESS && err != HS_SCAN_TERMINATED) {
        return -1;
    }

    return g_match_id;
}

// Get pattern count
__attribute__((export_name("matcher_pattern_count")))
int matcher_pattern_count(void) {
    return g_pattern_count;
}

// Get last error message
__attribute__((export_name("matcher_get_error")))
const char* matcher_get_error(void) {
    return g_error_msg;
}

// Check if platform is valid for Hyperscan
__attribute__((export_name("matcher_check_platform")))
int matcher_check_platform(void) {
    return hs_valid_platform();
}

// Close and free resources
__attribute__((export_name("matcher_close")))
void matcher_close(void) {
    if (g_scratch) {
        hs_free_scratch(g_scratch);
        g_scratch = nullptr;
    }
    if (g_database) {
        hs_free_database(g_database);
        g_database = nullptr;
    }
    g_pattern_count = 0;
}

} // extern "C"
