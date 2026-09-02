package service

import (
	"sync"
	"time"
)

// RateLimiter is a tiny in-memory fixed-window limiter keyed by client IP and a
// named bucket. It is enough for this scale (one event, one server) and guards
// the sensitive routes against brute force and enumeration.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*window
}

type window struct {
	count int
	start time.Time
}

func newRateLimiter() *RateLimiter {
	r := &RateLimiter{buckets: make(map[string]*window)}
	go r.sweepLoop()
	return r
}

// Allow records one hit for (bucket, ip) and reports whether it is within the
// limit. A hit that trips the limit still returns false (the request should be
// rejected with 429).
func (r *RateLimiter) Allow(bucket, ip string, max int, per time.Duration) bool {
	key := bucket + ":" + ip
	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	rec := r.buckets[key]
	if rec == nil || now.Sub(rec.start) > per {
		r.buckets[key] = &window{count: 1, start: now}
		return true
	}
	rec.count++
	return rec.count <= max
}

// Clear resets a bucket for an IP — called after a successful admin login so a
// legitimate user is never locked out by earlier failed attempts.
func (r *RateLimiter) Clear(bucket, ip string) {
	r.mu.Lock()
	delete(r.buckets, bucket+":"+ip)
	r.mu.Unlock()
}

func (r *RateLimiter) sweepLoop() {
	t := time.NewTicker(30 * time.Minute)
	defer t.Stop()
	for range t.C {
		now := time.Now()
		r.mu.Lock()
		for k, w := range r.buckets {
			if now.Sub(w.start) > time.Hour {
				delete(r.buckets, k)
			}
		}
		r.mu.Unlock()
	}
}
