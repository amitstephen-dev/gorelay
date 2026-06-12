package worker

import (
    "math"
    "time"
)

type RetryStrategy interface {
    NextDelay(attempt int) time.Duration
}

type ExponentialBackoff struct {
    BaseDelay time.Duration
    MaxDelay  time.Duration
    Multiplier float64
}

func NewExponentialBackoff(base, max time.Duration) *ExponentialBackoff {
    return &ExponentialBackoff{
        BaseDelay:  base,
        MaxDelay:   max,
        Multiplier: 2.0,
    }
}

func (e *ExponentialBackoff) NextDelay(attempt int) time.Duration {
    if attempt <= 0 {
        return e.BaseDelay
    }
    
    delay := float64(e.BaseDelay) * math.Pow(e.Multiplier, float64(attempt-1))
    if delay > float64(e.MaxDelay) {
        delay = float64(e.MaxDelay)
    }
    
    return time.Duration(delay)
}

type LinearBackoff struct {
    BaseDelay time.Duration
    MaxDelay  time.Duration
}

func NewLinearBackoff(base, max time.Duration) *LinearBackoff {
    return &LinearBackoff{
        BaseDelay: base,
        MaxDelay:  max,
    }
}

func (l *LinearBackoff) NextDelay(attempt int) time.Duration {
    delay := time.Duration(attempt) * l.BaseDelay
    if delay > l.MaxDelay {
        delay = l.MaxDelay
    }
    return delay
}

type FixedBackoff struct {
    Delay time.Duration
}

func NewFixedBackoff(delay time.Duration) *FixedBackoff {
    return &FixedBackoff{Delay: delay}
}

func (f *FixedBackoff) NextDelay(attempt int) time.Duration {
    return f.Delay
}

type NoBackoff struct{}

func (n *NoBackoff) NextDelay(attempt int) time.Duration {
    return 0
}

func CalculateRetryDelay(attempt int, maxRetries int) time.Duration {
    if attempt >= maxRetries {
        return 0
    }
    
    // Exponential backoff with jitter: min(2^attempt, max) seconds + jitter
    delay := time.Duration(math.Pow(2, float64(attempt))) * time.Second
    if delay > time.Hour {
        delay = time.Hour
    }
    
    // Add jitter (random ±20%)
    jitter := time.Duration(float64(delay) * (0.8 + 0.4*float64(time.Now().UnixNano()%100)/100))
    return jitter
}