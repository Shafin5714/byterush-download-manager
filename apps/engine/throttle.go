package main

import (
	"sync"
	"time"
)

type Limiter struct {
	mu     sync.Mutex
	rate   float64 // bytes per second, 0 = unlimited
	tokens float64
	last   time.Time
}

func NewLimiter() *Limiter {
	return &Limiter{last: time.Now()}
}

func (l *Limiter) SetRate(kbs int) {
	l.mu.Lock()
	if kbs <= 0 {
		l.rate = 0
	} else {
		l.rate = float64(kbs) * 1024
	}
	l.mu.Unlock()
}

// Wait blocks until n tokens are available. Unlimited when rate == 0.
func (l *Limiter) Wait(n int) {
	if n <= 0 {
		return
	}
	l.mu.Lock()
	if l.rate <= 0 {
		l.mu.Unlock()
		return
	}
	now := time.Now()
	elapsed := now.Sub(l.last).Seconds()
	l.last = now
	l.tokens += elapsed * l.rate
	if l.tokens > l.rate {
		l.tokens = l.rate
	}
	for l.tokens < float64(n) {
		need := float64(n) - l.tokens
		l.mu.Unlock()
		time.Sleep(time.Duration(need / l.rate * float64(time.Second)))
		l.mu.Lock()
		now := time.Now()
		elapsed := now.Sub(l.last).Seconds()
		l.last = now
		l.tokens += elapsed * l.rate
		if l.tokens > l.rate {
			l.tokens = l.rate
		}
	}
	l.tokens -= float64(n)
	l.mu.Unlock()
}
