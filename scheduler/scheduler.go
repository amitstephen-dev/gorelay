package scheduler

import (
    "context"
    "log"
    "sync"
    "time"
    
    "github.com/gorelay/gorelay/storage"
    // "github.com/gorelay/gorelay/task"  ← Remove this, it's not used
)

type Scheduler struct {
    store   storage.Storage
    ctx     context.Context
    cancel  context.CancelFunc
    wg      sync.WaitGroup
    ticker  *time.Ticker
}

func New(store storage.Storage) *Scheduler {
    ctx, cancel := context.WithCancel(context.Background())
    
    return &Scheduler{
        store:  store,
        ctx:    ctx,
        cancel: cancel,
    }
}

func (s *Scheduler) Start() {
    s.wg.Add(1)
    go s.run()
    log.Println("Scheduler started")
}

func (s *Scheduler) Stop() {
    s.cancel()
    s.wg.Wait()
    log.Println("Scheduler stopped")
}

func (s *Scheduler) run() {
    defer s.wg.Done()
    
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-s.ctx.Done():
            return
        case <-ticker.C:
            s.processReadyTasks()
        }
    }
}

func (s *Scheduler) processReadyTasks() {
    // Query to get tasks that are ready to execute
    // This depends on your storage interface
    // For now, we'll log that we're checking
    log.Println("Scheduler checking for ready tasks")
    
    // Note: Actual task claiming is done by workers
    // The scheduler just ensures tasks with execute_at <= NOW()
    // are marked as pending (they already are)
    
    // If your storage supports it, you could add:
    // readyTasks, err := s.store.GetTasksReadyToExecute(100)
    // if err != nil {
    //     log.Printf("Scheduler error: %v", err)
    //     return
    // }
    // 
    // for _, task := range readyTasks {
    //     // Tasks are already pending, workers will pick them up
    //     log.Printf("Task %s is ready for execution", task.ID)
    // }
}