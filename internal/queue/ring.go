package queue

import (
    "sync/atomic"
    "unsafe"
    
    "github.com/gorelay/gorelay/task"
)

type RingBuffer struct {
    buffer []unsafe.Pointer
    size   uint64
    head   uint64
    tail   uint64
}

func NewRingBuffer(size int) *RingBuffer {
    // Size must be power of two
    if size&(size-1) != 0 {
        panic("ring buffer size must be power of two")
    }
    
    return &RingBuffer{
        buffer: make([]unsafe.Pointer, size),
        size:   uint64(size),
    }
}

func (r *RingBuffer) Push(t *task.Task) bool {
    for {
        tail := atomic.LoadUint64(&r.tail)
        head := atomic.LoadUint64(&r.head)
        
        if tail-head >= r.size {
            return false // Full
        }
        
        if atomic.CompareAndSwapUint64(&r.tail, tail, tail+1) {
            idx := tail & (r.size - 1)
            atomic.StorePointer(&r.buffer[idx], unsafe.Pointer(t))
            return true
        }
    }
}

func (r *RingBuffer) Pop() *task.Task {
    for {
        head := atomic.LoadUint64(&r.head)
        tail := atomic.LoadUint64(&r.tail)
        
        if head >= tail {
            return nil // Empty
        }
        
        if atomic.CompareAndSwapUint64(&r.head, head, head+1) {
            idx := head & (r.size - 1)
            ptr := atomic.SwapPointer(&r.buffer[idx], nil)
            if ptr != nil {
                return (*task.Task)(ptr)
            }
        }
    }
}

func (r *RingBuffer) Len() uint64 {
    return atomic.LoadUint64(&r.tail) - atomic.LoadUint64(&r.head)
}

func (r *RingBuffer) Cap() uint64 {
    return r.size
}