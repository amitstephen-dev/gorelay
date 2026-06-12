package pool

import (
    "sync"
    
    "github.com/gorelay/gorelay/task"
)

var TaskPool = sync.Pool{
    New: func() interface{} {
        return &task.Task{
            Payload: make([]byte, 0, 512),
        }
    },
}

var BufferPool = sync.Pool{
    New: func() interface{} {
        b := make([]byte, 0, 4096)
        return &b
    },
}

func AcquireTask() *task.Task {
    t := TaskPool.Get().(*task.Task)
    t.Reset()
    return t
}

func ReleaseTask(t *task.Task) {
    if t != nil {
        t.Reset()
        TaskPool.Put(t)
    }
}

func AcquireBuffer() *[]byte {
    return BufferPool.Get().(*[]byte)
}

func ReleaseBuffer(buf *[]byte) {
    *buf = (*buf)[:0]
    BufferPool.Put(buf)
}