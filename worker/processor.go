package worker

import (
    "log"
    "sync"
    "time"
    
    "github.com/gorelay/gorelay/registry"
    "github.com/gorelay/gorelay/storage"
    "github.com/gorelay/gorelay/task"
)

type Processor struct {
    store     storage.Storage
    registry  *registry.Registry
    workerID  string
    taskChan  <-chan *task.Task
    concurrency int
    wg        sync.WaitGroup
    stopChan  chan struct{}
}

func NewProcessor(store storage.Storage, registry *registry.Registry, 
                  workerID string, taskChan <-chan *task.Task, concurrency int) *Processor {
    return &Processor{
        store:       store,
        registry:    registry,
        workerID:    workerID,
        taskChan:    taskChan,
        concurrency: concurrency,
        stopChan:    make(chan struct{}),
    }
}

func (p *Processor) Start() {
    for i := 0; i < p.concurrency; i++ {
        p.wg.Add(1)
        go p.run(i)
    }
}

func (p *Processor) Stop() {
    close(p.stopChan)
    p.wg.Wait()
}

func (p *Processor) run(workerNum int) {
    defer p.wg.Done()
    
    for {
        select {
        case <-p.stopChan:
            return
        case task, ok := <-p.taskChan:
            if !ok {
                return
            }
            p.processTask(task, workerNum)
        }
    }
}

func (p *Processor) processTask(t *task.Task, workerNum int) {
    start := time.Now()
    log.Printf("Worker %d processing task %s (%s)", workerNum, t.ID, t.Topic)
    
    handler, ok := p.registry.Get(t.Topic)
    if !ok {
        log.Printf("No handler for topic: %s", t.Topic)
        p.store.FailTask(t.ID, "no handler registered")
        return
    }
    
    err := handler.Execute(t.Payload)
    
    duration := time.Since(start)
    
    if err != nil {
        log.Printf("Task %s failed after %v: %v", t.ID, duration, err)
        p.store.FailTask(t.ID, err.Error())
    } else {
        log.Printf("Task %s completed in %v", t.ID, duration)
        p.store.CompleteTask(t.ID)
    }
}