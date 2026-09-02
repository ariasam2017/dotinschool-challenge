package service

import (
	"testing"
	"time"
)

func TestRateLimiter_FixedWindow(t *testing.T) {
	r := &RateLimiter{buckets: map[string]*window{}}

	for i := 0; i < 3; i++ {
		if !r.Allow("b", "1.2.3.4", 3, time.Minute) {
			t.Fatalf("hit %d should be allowed", i+1)
		}
	}
	if r.Allow("b", "1.2.3.4", 3, time.Minute) {
		t.Fatal("4th hit over a limit of 3 must be rejected")
	}
	// a different IP has its own window
	if !r.Allow("b", "9.9.9.9", 3, time.Minute) {
		t.Fatal("other IP must not be affected")
	}
	// Clear resets the bucket
	r.Clear("b", "1.2.3.4")
	if !r.Allow("b", "1.2.3.4", 3, time.Minute) {
		t.Fatal("after Clear the IP should be allowed again")
	}
}

func TestRateLimiter_WindowExpiry(t *testing.T) {
	r := &RateLimiter{buckets: map[string]*window{}}
	if !r.Allow("b", "ip", 1, 20*time.Millisecond) {
		t.Fatal("first hit allowed")
	}
	if r.Allow("b", "ip", 1, 20*time.Millisecond) {
		t.Fatal("second hit within window rejected")
	}
	time.Sleep(30 * time.Millisecond)
	if !r.Allow("b", "ip", 1, 20*time.Millisecond) {
		t.Fatal("after the window elapses the count resets")
	}
}
