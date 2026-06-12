package registry

import (
    "encoding/json"
    "fmt"
    "reflect"
    "time"
)

// HandlerFunc is the function signature for task handlers
type HandlerFunc func(payload interface{}) error

type Handler struct {
    fn          HandlerFunc
    payloadType reflect.Type
    maxRetries  int
    timeout     time.Duration
    middleware  []MiddlewareFunc
}

type MiddlewareFunc func(next HandlerFunc) HandlerFunc

func NewHandler(fn HandlerFunc, payloadType interface{}, opts ...HandlerOption) *Handler {
    t := reflect.TypeOf(payloadType)
    if t.Kind() == reflect.Ptr {
        t = t.Elem()
    }
    
    h := &Handler{
        fn:          fn,
        payloadType: t,
        maxRetries:  3,
        timeout:     30 * time.Second,
        middleware:  []MiddlewareFunc{},
    }
    
    for _, opt := range opts {
        opt(h)
    }
    
    return h
}

type HandlerOption func(*Handler)

func WithMaxRetries(retries int) HandlerOption {
    return func(h *Handler) {
        h.maxRetries = retries
    }
}

func WithTimeout(timeout time.Duration) HandlerOption {
    return func(h *Handler) {
        h.timeout = timeout
    }
}

func WithMiddleware(middleware ...MiddlewareFunc) HandlerOption {
    return func(h *Handler) {
        h.middleware = append(h.middleware, middleware...)
    }
}

func (h *Handler) Execute(payload json.RawMessage) error {
    // Create new instance of payload type
    payloadValue := reflect.New(h.payloadType).Interface()
    
    // Unmarshal JSON
    if err := json.Unmarshal(payload, payloadValue); err != nil {
        return fmt.Errorf("failed to unmarshal payload: %w", err)
    }
    
    // Apply middleware chain
    handler := h.fn
    for i := len(h.middleware) - 1; i >= 0; i-- {
        handler = h.middleware[i](handler)
    }
    
    // Execute with timeout
    done := make(chan error, 1)
    go func() {
        done <- handler(payloadValue)
    }()
    
    select {
    case err := <-done:
        return err
    case <-time.After(h.timeout):
        return fmt.Errorf("handler execution timeout after %v", h.timeout)
    }
}

func (h *Handler) MaxRetries() int {
    return h.maxRetries
}