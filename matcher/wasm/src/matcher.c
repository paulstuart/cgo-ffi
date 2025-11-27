// WASM wrapper for Vectorscan multi-pattern matching
// Exports minimal API for pattern compilation and matching

#include <stdlib.h>
#include <string.h>
#include "hs.h"

// Global state
static hs_database_t *g_database = NULL;
static hs_scratch_t *g_scratch = NULL;
static int g_pattern_count = 0;

// Match result for callback
static int g_match_id = -1;

// Callback for hs_scan - captures first match and terminates
static int match_handler(unsigned int id, unsigned long long from,
                         unsigned long long to, unsigned int flags, void *ctx) {
    g_match_id = (int)id;
    return 1;  // Non-zero to stop scanning
}

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
    if (patterns_len == 0) return -1;

    int count = 1;
    for (int i = 0; i < patterns_len; i++) {
        if (patterns_data[i] == '\n') count++;
    }

    // Allocate arrays for pattern data
    const char** expressions = malloc(count * sizeof(char*));
    unsigned int* flags = malloc(count * sizeof(unsigned int));
    unsigned int* ids = malloc(count * sizeof(unsigned int));

    if (!expressions || !flags || !ids) {
        free(expressions);
        free(flags);
        free(ids);
        return -2;
    }

    // Parse patterns - split on newlines
    char* data_copy = malloc(patterns_len + 1);
    if (!data_copy) {
        free(expressions);
        free(flags);
        free(ids);
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
    hs_compile_error_t *compile_err = NULL;
    hs_error_t err = hs_compile_multi(expressions, flags, ids, actual_count,
                                       HS_MODE_BLOCK, NULL, &g_database, &compile_err);

    if (err != HS_SUCCESS) {
        if (compile_err) {
            hs_free_compile_error(compile_err);
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
        g_database = NULL;
        free(data_copy);
        free(expressions);
        free(flags);
        free(ids);
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
                             g_scratch, match_handler, NULL);

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

// Close and free resources
__attribute__((export_name("matcher_close")))
void matcher_close(void) {
    if (g_scratch) {
        hs_free_scratch(g_scratch);
        g_scratch = NULL;
    }
    if (g_database) {
        hs_free_database(g_database);
        g_database = NULL;
    }
    g_pattern_count = 0;
}
