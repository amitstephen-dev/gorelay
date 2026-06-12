package gorelay

import (
    "time"
)

type Config struct {
    // Worker configuration
    WorkerCount      int           `json:"worker_count"`
    WorkerBatchSize  int           `json:"worker_batch_size"`
    
    // Retry configuration
    MaxRetries       int           `json:"max_retries"`
    RetryBackoff     time.Duration `json:"retry_backoff"`
    RetryMaxDelay    time.Duration `json:"retry_max_delay"`
    
    // Storage configuration
    StorageType      string        `json:"storage_type"`
    StorageDSN       string        `json:"storage_dsn"`
    
    // Dashboard configuration
    DashboardAddr    string        `json:"dashboard_addr"`
    DashboardAuth    bool          `json:"dashboard_auth"`
    DashboardUser    string        `json:"dashboard_user"`
    DashboardPass    string        `json:"dashboard_pass"`
    
    // Task configuration
    TaskRetention    time.Duration `json:"task_retention"`
    FailedRetention  time.Duration `json:"failed_retention"`
    VisibilityTimeout time.Duration `json:"visibility_timeout"`
    
    // Performance
    BatchSize        int           `json:"batch_size"`
    RingBufferSize   int           `json:"ring_buffer_size"`
    MemoryLimit      int64         `json:"memory_limit"`
}

type Option func(*Config)

func DefaultConfig() *Config {
    return &Config{
        WorkerCount:       4,
        WorkerBatchSize:   10,
        MaxRetries:        3,
        RetryBackoff:      time.Second,
        RetryMaxDelay:     time.Hour,
        StorageType:       "sqlite",
        StorageDSN:        "relay.db",
        DashboardAddr:     "",
        TaskRetention:     7 * 24 * time.Hour,
        FailedRetention:   30 * 24 * time.Hour,
        VisibilityTimeout: 30 * time.Second,
        BatchSize:         100,
        RingBufferSize:    65536,
        MemoryLimit:       512,
    }
}

func (c *Config) Validate() error {
    if c.WorkerCount <= 0 {
        c.WorkerCount = 4
    }
    if c.BatchSize <= 0 {
        c.BatchSize = 100
    }
    if c.RingBufferSize&(c.RingBufferSize-1) != 0 {
        c.RingBufferSize = 65536
    }
    if c.MaxRetries < 0 {
        c.MaxRetries = 3
    }
    return nil
}

func WithWorkerCount(count int) Option {
    return func(c *Config) {
        c.WorkerCount = count
    }
}

func WithMaxRetries(retries int) Option {
    return func(c *Config) {
        c.MaxRetries = retries
    }
}

func WithDashboard(addr string) Option {
    return func(c *Config) {
        c.DashboardAddr = addr
    }
}

func WithStorage(dsn string) Option {
    return func(c *Config) {
        c.StorageDSN = dsn
    }
}