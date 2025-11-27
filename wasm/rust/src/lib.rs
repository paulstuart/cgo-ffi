// Rust implementation of vector operations for WASM (Core Module)
//
// Uses pre-allocated static buffers to eliminate per-call allocation.
// The host copies data into these buffers at known offsets.
//
// This version uses direct #[no_mangle] exports for compatibility with
// wasmtime's core module API (not Component Model).

#![no_std]

use core::cell::UnsafeCell;
use core::ptr::addr_of;

// Pre-allocated buffer capacity (100K f64 elements = 800KB per buffer)
const CAPACITY: usize = 100_000;

// Wrapper for static mutable buffers - safe in single-threaded WASM
#[repr(transparent)]
struct StaticBuffer(UnsafeCell<[f64; CAPACITY]>);

// SAFETY: WASM is single-threaded, so this is safe
unsafe impl Sync for StaticBuffer {}

impl StaticBuffer {
    const fn new() -> Self {
        StaticBuffer(UnsafeCell::new([0.0; CAPACITY]))
    }

    #[inline]
    fn as_ptr(&self) -> *const f64 {
        self.0.get() as *const f64
    }

    #[inline]
    fn as_mut_ptr(&self) -> *mut f64 {
        self.0.get() as *mut f64
    }

    #[inline]
    unsafe fn get(&self, i: usize) -> f64 {
        *self.as_ptr().add(i)
    }

    #[inline]
    unsafe fn set(&self, i: usize, val: f64) {
        *self.as_mut_ptr().add(i) = val;
    }
}

// Static buffers - allocated once, stable addresses
static BUFFER_A: StaticBuffer = StaticBuffer::new();
static BUFFER_B: StaticBuffer = StaticBuffer::new();
static RESULT: StaticBuffer = StaticBuffer::new();

#[no_mangle]
pub extern "C" fn sum(len: u32) -> f64 {
    let len = (len as usize).min(CAPACITY);
    let mut s = 0.0;
    unsafe {
        for i in 0..len {
            s += BUFFER_A.get(i);
        }
    }
    s
}

#[no_mangle]
pub extern "C" fn dot(len: u32) -> f64 {
    let len = (len as usize).min(CAPACITY);
    let mut d = 0.0;
    unsafe {
        for i in 0..len {
            d += BUFFER_A.get(i) * BUFFER_B.get(i);
        }
    }
    d
}

#[no_mangle]
pub extern "C" fn mul(len: u32) {
    let len = (len as usize).min(CAPACITY);
    unsafe {
        for i in 0..len {
            RESULT.set(i, BUFFER_A.get(i) * BUFFER_B.get(i));
        }
    }
}

#[no_mangle]
pub extern "C" fn scale(scalar: f64, len: u32) {
    let len = (len as usize).min(CAPACITY);
    unsafe {
        for i in 0..len {
            BUFFER_A.set(i, BUFFER_A.get(i) * scalar);
        }
    }
}

#[no_mangle]
pub extern "C" fn sum_simd(len: u32) -> f64 {
    let len = (len as usize).min(CAPACITY);
    // 4-way unrolling for better auto-vectorization
    let mut sum0 = 0.0;
    let mut sum1 = 0.0;
    let mut sum2 = 0.0;
    let mut sum3 = 0.0;

    unsafe {
        let mut i = 0;
        while i + 3 < len {
            sum0 += BUFFER_A.get(i);
            sum1 += BUFFER_A.get(i + 1);
            sum2 += BUFFER_A.get(i + 2);
            sum3 += BUFFER_A.get(i + 3);
            i += 4;
        }
        while i < len {
            sum0 += BUFFER_A.get(i);
            i += 1;
        }
    }
    sum0 + sum1 + sum2 + sum3
}

#[no_mangle]
pub extern "C" fn get_buffer_a_offset() -> u32 {
    addr_of!(BUFFER_A) as u32
}

#[no_mangle]
pub extern "C" fn get_buffer_b_offset() -> u32 {
    addr_of!(BUFFER_B) as u32
}

#[no_mangle]
pub extern "C" fn get_result_offset() -> u32 {
    addr_of!(RESULT) as u32
}

#[no_mangle]
pub extern "C" fn get_capacity() -> u32 {
    CAPACITY as u32
}

// Panic handler for no_std
#[panic_handler]
fn panic(_info: &core::panic::PanicInfo) -> ! {
    loop {}
}
