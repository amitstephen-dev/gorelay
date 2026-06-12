package registry

import (
    "encoding/json"
    "fmt"
)

type Registry struct {
    handlers map[string]*Handler
}

func New() *Registry {
    return &Registry{
        handlers: make(map[string]*Handler),
    }
}

func (r *Registry) Register(topic string, fn HandlerFunc, payloadType interface{}) error {
    if _, exists := r.handlers[topic]; exists {
        return fmt.Errorf("handler already registered for topic: %s", topic)
    }
    
    r.handlers[topic] = NewHandler(fn, payloadType)
    return nil
}

func (r *Registry) Get(topic string) (*Handler, bool) {
    h, ok := r.handlers[topic]
    return h, ok
}

func (r *Registry) Execute(topic string, payload json.RawMessage) error {
    handler, ok := r.Get(topic)
    if !ok {
        return fmt.Errorf("no handler registered for topic: %s", topic)
    }
    return handler.Execute(payload)
}

func (r *Registry) Has(topic string) bool {
    _, ok := r.handlers[topic]
    return ok
}

func (r *Registry) Remove(topic string) {
    delete(r.handlers, topic)
}

func (r *Registry) Topics() []string {
    topics := make([]string, 0, len(r.handlers))
    for topic := range r.handlers {
        topics = append(topics, topic)
    }
    return topics
}