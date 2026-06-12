package storage

import (
    "time"
    "github.com/gorelay/gorelay/task"
)

type Storage interface {
    // Core operations
    SaveTask(t *task.Task) error
    ClaimTask(workerID string) (*task.Task, error)
    CompleteTask(id string) error
    FailTask(id string, errMsg string) error
    
    // Read operations
    GetTask(id string) (*task.Task, error)
    GetTasks(filter TaskFilter) ([]*task.Task, error)
    GetStats() (*Stats, error)
    
    // History
    RecordHistory(taskID, event, message string) error
    GetTaskHistory(taskID string) ([]*HistoryEntry, error)
    
    // Batch operations
    BatchSaveTasks(tasks []*task.Task) error
    BatchClaimTasks(workerID string, limit int) ([]*task.Task, error)
    
    // Cleanup
    Cleanup(completedRetention, failedRetention time.Duration) error
    
    // Health
    Ping() error
    Close() error
}

type TaskFilter struct {
    Topic    string
    Status   task.Status
    Limit    int
    Offset   int
    OrderBy  string
}

type Stats struct {
    Pending   int64 `json:"pending"`
    Running   int64 `json:"running"`
    Completed int64 `json:"completed"`
    Failed    int64 `json:"failed"`
    Dead      int64 `json:"dead"`
    Total     int64 `json:"total"`
}

type HistoryEntry struct {
    ID        int64     `json:"id"`
    TaskID    string    `json:"task_id"`
    Event     string    `json:"event"`
    Message   string    `json:"message"`
    CreatedAt time.Time `json:"created_at"`
}