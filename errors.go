package gorelay

import "errors"

var (
    // Core errors
    ErrHandlerNotFound     = errors.New("handler not found for topic")
    ErrTaskNotFound        = errors.New("task not found")
    ErrTaskAlreadyExists   = errors.New("task already exists")
    ErrTaskAlreadyComplete = errors.New("task already completed")
    
    // Storage errors
    ErrStorageFull         = errors.New("storage is full")
    ErrStorageTimeout      = errors.New("storage operation timeout")
    ErrConnectionFailed    = errors.New("connection to storage failed")
    
    // Worker errors
    ErrWorkerPoolFull      = errors.New("worker pool is full")
    ErrWorkerTimeout       = errors.New("worker execution timeout")
    
    // Queue errors
    ErrQueueFull           = errors.New("queue is full")
    ErrQueueEmpty          = errors.New("queue is empty")
    
    // Validation errors
    ErrInvalidTopic        = errors.New("invalid topic name")
    ErrInvalidPayload      = errors.New("invalid payload")
    ErrInvalidScheduleTime = errors.New("invalid schedule time")
    
    // Configuration errors
    ErrInvalidConfig       = errors.New("invalid configuration")
    ErrInvalidStorageType  = errors.New("invalid storage type")
)

type RelayError struct {
    Op  string
    Err error
}

func (e *RelayError) Error() string {
    return e.Op + ": " + e.Err.Error()
}

func (e *RelayError) Unwrap() error {
    return e.Err
}
