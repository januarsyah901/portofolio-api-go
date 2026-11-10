package authapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func newTestLimiter() (*limiter, *time.Time) {
	now := time.Unix(1_700_000_000, 0)
	l := newLimiter()
	l.now = func() time.Time { return now }
	l.lastSweep = now
	return l, &now
}

func TestLimiterBurstThenRefill(t *testing.T) {
	l, now := newTestLimiter()
	for i := 0; i < 5; i++ {
		if !l.allow("a") {
			t.Fatalf("request %d should pass within burst", i+1)
		}
	}
	if l.allow("a") {
		t.Fatal("6th request must be limited")
	}
	if !l.allow("b") {
		t.Fatal("other keys are independent")
	}
	*now = now.Add(11 * time.Second)
	if l.allow("a") {
		t.Fatal("token not yet refilled after 11s")
	}
	*now = now.Add(2 * time.Second) // 13s total > 12s per token
	if !l.allow("a") {
		t.Fatal("one token should have refilled")
	}
	if l.allow("a") {
		t.Fatal("only one token refilled")
	}
}

func TestLimiterEvictsIdle(t *testing.T) {
	l, now := newTestLimiter()
	for i := 0; i < 100; i++ {
		l.allow(fmt.Sprintf("ip-%d", i))
	}
	if l.size() != 100 {
		t.Fatalf("size = %d", l.size())
	}
	*now = now.Add(10 * time.Minute)
	l.allow("fresh")
	if l.size() != 1 {
		t.Fatalf("idle entries not evicted, size = %d", l.size())
	}
}

func TestLimiterConcurrent(t *testing.T) {
	l := newLimiter()
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.allow("same") {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	// burst 5 plus at most a sliver of refill during the run
	if allowed < 5 || allowed > 6 {
		t.Fatalf("allowed = %d, want 5", allowed)
	}
}

func TestMiddlewareKeysByRemoteAddrIgnoringXFF(t *testing.T) {
	l, _ := newTestLimiter()
	h := l.middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	do := func(remote, xff string) int {
		req := httptest.NewRequest("POST", "/", nil)
		req.RemoteAddr = remote
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	for i := 0; i < 5; i++ {
		if c := do("1.2.3.4:1000"+fmt.Sprint(i), fmt.Sprintf("9.9.9.%d", i)); c != 204 {
			t.Fatalf("req %d = %d", i, c)
		}
	}
	// different port + spoofed XFF, same host -> still limited
	if c := do("1.2.3.4:55555", "8.8.8.8"); c != 429 {
		t.Fatalf("spoofed XFF got %d, want 429", c)
	}
	if c := do("5.6.7.8:1", ""); c != 204 {
		t.Fatalf("other host got %d", c)
	}
}
