package worker

import (
    "log"
    "sync"
    "time"
    
    "github.com/gorelay/gorelay/storage"
    "github.com/gorelay/gorelay/task"
)

type Claimer struct {
    store     storage.Storage
    workerID  string
    batchSize int
    taskChan  chan *task.Task
    stopChan  chan struct{}
    wg        sync.WaitGroup
}

func NewClaimer(store storage.Storage, workerID string, batchSize int) *Claimer {
    return &Claimer{
        store:     store,
        workerID:  workerID,
        batchSize: batchSize,
        taskChan:  make(chan *task.Task, batchSize*2),
        stopChan:  make(chan struct{}),
    }
}

func (c *Claimer) Start() {
    c.wg.Add(1)
    go c.run()
}

func (c *Claimer) Stop() {
    close(c.stopChan)
    c.wg.Wait()
    close(c.taskChan)
}

func (c *Claimer) Tasks() <-chan *task.Task {
    return c.taskChan
}

func (c *Claimer) run() {
    defer c.wg.Done()
    
    for {
        select {
        case <-c.stopChan:
            return
        default:
            tasks, err := c.store.BatchClaimTasks(c.workerID, c.batchSize)
            if err != nil {
                log.Printf("Claimer error: %v", err)
                time.Sleep(time.Second)
                continue
            }
            
            if len(tasks) == 0 {
                time.Sleep(100 * time.Millisecond)
                continue
            }
            
            for _, task := range tasks {
                select {
                case c.taskChan <- task:
                case <-c.stopChan:
                    return
                }
            }
        }
    }
}