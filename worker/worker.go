package worker

import (
    "context"
    "fmt"
    "log"
    "sync"
    "time"
    
    "github.com/gorelay/gorelay/registry"
    "github.com/gorelay/gorelay/storage"
    "github.com/gorelay/gorelay/task"
)

type Worker struct {
    id        string
    store     storage.Storage
    registry  *registry.Registry
    ctx       context.Context
    cancel    context.CancelFunc
    wg        sync.WaitGroup
    running   bool
}

type Pool struct {
    workers []*Worker
    store   storage.Storage
    registry *registry.Registry
    count   int
    ctx     context.Context
    cancel  context.CancelFunc
    wg      sync.WaitGroup
}

func NewPool(store storage.Storage, registry *registry.Registry, count int) *Pool {
    ctx, cancel := context.WithCancel(context.Background())
    
    return &Pool{
        workers:  make([]*Worker, 0, count),
        store:    store,
        registry: registry,
        count:    count,
        ctx:      ctx,
        cancel:   cancel,
    }
}

func (p *Pool) Start() {
    for i := 0; i < p.count; i++ {
        worker := &Worker{
            id:       fmt.Sprintf("worker_%d_%d", i, time.Now().Unix()),
            store:    p.store,
            registry: p.registry,
        }
        
        p.workers = append(p.workers, worker)
        
        p.wg.Add(1)
        go worker.run(p.ctx, &p.wg)
    }
    
    log.Printf("Started %d workers", p.count)
}

func (p *Pool) Stop() {
    p.cancel()
    p.wg.Wait()
    log.Println("All workers stopped")
}

func (w *Worker) run(ctx context.Context, wg *sync.WaitGroup) {
    defer wg.Done()
    
    for {
        select {
        case <-ctx.Done():
            return
        default:
            task, err := w.store.ClaimTask(w.id)
            if err != nil {
                log.Printf("Worker %s error claiming task: %v", w.id, err)
                time.Sleep(time.Second)
                continue
            }
            
            if task == nil {
                time.Sleep(100 * time.Millisecond)
                continue
            }
            
            w.processTask(task)
        }
    }
}

func (w *Worker) processTask(t *task.Task) {
    log.Printf("Worker %s processing task %s (%s)", w.id, t.ID, t.Topic)
    
    handler, ok := w.registry.Get(t.Topic)
    if !ok {
        log.Printf("No handler for topic: %s", t.Topic)
        w.store.FailTask(t.ID, fmt.Sprintf("no handler for topic: %s", t.Topic))
        return
    }
    
    err := handler.Execute(t.Payload)
    
    if err != nil {
        log.Printf("Task %s failed: %v", t.ID, err)
        w.store.FailTask(t.ID, err.Error())
    } else {
        log.Printf("Task %s completed", t.ID)
        w.store.CompleteTask(t.ID)
    }
}