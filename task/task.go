package task

import (
    "encoding/json"
    "time"
)

// Remove Status definition from here (it's in status.go)
// Remove Priority definition from here (it's in priority.go)

type Task struct {
    ID          string          `json:"id"`
    Topic       string          `json:"topic"`
    Payload     json.RawMessage `json:"payload"`
    Status      Status          `json:"status"`      // Now uses Status from status.go
    Priority    Priority        `json:"priority"`    // Now uses Priority from priority.go
    RetryCount  int             `json:"retry_count"`
    MaxRetries  int             `json:"max_retries"`
    ExecuteAt   time.Time       `json:"execute_at"`
    LastError   string          `json:"last_error,omitempty"`
    WorkerID    string          `json:"worker_id,omitempty"`
    ClaimedAt   time.Time       `json:"claimed_at,omitempty"`
    CreatedAt   time.Time       `json:"created_at"`
    UpdatedAt   time.Time       `json:"updated_at"`
    CompletedAt time.Time       `json:"completed_at,omitempty"`
}

func NewTask(topic string, payload json.RawMessage) *Task {
    now := time.Now()
    return &Task{
        ID:         GenerateID(),
        Topic:      topic,
        Payload:    payload,
        Status:     StatusPending,
        Priority:   PriorityNormal,
        MaxRetries: 3,
        ExecuteAt:  now,
        CreatedAt:  now,
        UpdatedAt:  now,
    }
}

func (t *Task) CanRetry() bool {
    return t.RetryCount < t.MaxRetries
}

func (t *Task) Reset() {
    t.ID = ""
    t.Topic = ""
    t.Payload = nil
    t.Status = ""
    t.Priority = 0
    t.RetryCount = 0
    t.MaxRetries = 0
    t.LastError = ""
    t.WorkerID = ""
}

func GenerateID() string {
    return "task_" + time.Now().Format("20060102150405") + "_" + randomString(8)
}

func randomString(n int) string {
    const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
    b := make([]byte, n)
    for i := range b {
        b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
    }
    return string(b)
}
