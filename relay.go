package gorelay

import (
    "context"
    "encoding/json"
    "time"
    
    "github.com/amitstephen-dev/gorelay/registry"
    "github.com/amitstephen-dev/gorelay/storage"
    "github.com/amitstephen-dev/gorelay/storage/sqlite"
    "github.com/amitstephen-dev/gorelay/task"
    "github.com/amitstephen-dev/gorelay/worker"
    "github.com/amitstephen-dev/gorelay/scheduler"
    "github.com/amitstephen-dev/gorelay/dashboard"
)

type Relay struct {
    registry  *registry.Registry
    storage   storage.Storage
    worker    *worker.Pool
    scheduler *scheduler.Scheduler
    dashboard *dashboard.Dashboard
    config    *Config
}

// New creates a new Relay instance
func New(opts ...Option) *Relay {
    config := DefaultConfig()
    
    for _, opt := range opts {
        opt(config)
    }
    
    // Default SQLite storage
    store, err := sqlite.New(config.StorageDSN)
    if err != nil {
        panic(err)
    }
    
    r := &Relay{
        registry: registry.New(),
        storage:  store,
        config:   config,
    }
    
    r.worker = worker.NewPool(store, r.registry, config.WorkerCount)
    r.scheduler = scheduler.New(store)
    
    return r
}

// Register registers a handler for a topic
func (r *Relay) Register(topic string, fn registry.HandlerFunc, payloadType interface{}) error {
    return r.registry.Register(topic, fn, payloadType)
}

// Enqueue adds a task to the queue
func (r *Relay) Enqueue(topic string, payload interface{}) (string, error) {
    payloadBytes, err := json.Marshal(payload)
    if err != nil {
        return "", err
    }
    
    t := task.NewTask(topic, payloadBytes)
    t.MaxRetries = r.config.MaxRetries
    
    if err := r.storage.SaveTask(t); err != nil {
        return "", err
    }
    
    return t.ID, nil
}

// Schedule adds a task to be executed at a specific time
func (r *Relay) Schedule(executeAt time.Time, topic string, payload interface{}) (string, error) {
    payloadBytes, err := json.Marshal(payload)
    if err != nil {
        return "", err
    }
    
    t := task.NewTask(topic, payloadBytes)
    t.ExecuteAt = executeAt
    t.MaxRetries = r.config.MaxRetries
    
    if err := r.storage.SaveTask(t); err != nil {
        return "", err
    }
    
    return t.ID, nil
}

// Start starts the relay workers and scheduler
func (r *Relay) Start() error {
    if r.config.DashboardAddr != "" {
        r.dashboard = dashboard.New(r.storage, r.config.DashboardAddr)
        if err := r.dashboard.Start(); err != nil {
            return err
        }
    }
    
    r.scheduler.Start()
    r.worker.Start()
    
    return nil
}

// Shutdown gracefully shuts down the relay
func (r *Relay) Shutdown(ctx context.Context) error {
    r.worker.Stop()
    r.scheduler.Stop()
    
    if closer, ok := r.storage.(interface{ Close() error }); ok {
        return closer.Close()
    }
    
    return nil
}

// EnableDashboard enables the dashboard on the specified address
func (r *Relay) EnableDashboard(addr string) {
    r.config.DashboardAddr = addr
    r.dashboard = dashboard.New(r.storage, addr)
}
