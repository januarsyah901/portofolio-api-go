package authapi

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/januarsyah901/portofolio-api-go/internal/httpx"
)

// MsgTooManyAttempts is returned when a client exhausts its login budget (PRD S6).
const MsgTooManyAttempts = "Terlalu banyak percobaan. Coba lagi nanti."

const (
	rateBurst    = 5
	ratePerMin   = 5
	sweepEvery   = 5 * time.Minute
	rateInterval = time.Minute / ratePerMin
)

type bucket struct {
	tokens float64
	last   time.Time
}

// limiter is a per-key token bucket (5 requests/minute, burst 5). Idle keys are
// evicted lazily so the map stays bounded. It is safe for concurrent use.
//
// golang.org/x/time/rate is not in go.mod (and shared deps must not be edited by
// this module), so the bucket is implemented here; the algorithm is identical.
type limiter struct {
	mu        sync.Mutex
	buckets   map[string]*bucket
	now       func() time.Time
	lastSweep time.Time
	// ip picks the bucket key; nil means the TCP peer address.
	ip func(*http.Request) string
}

func newLimiter() *limiter {
	l := &limiter{buckets: map[string]*bucket{}, now: time.Now}
	l.lastSweep = l.now()
	return l
}

// allow reports whether key may proceed, consuming one token if so.
func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.sweepLocked(now)

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: rateBurst, last: now}
		l.buckets[key] = b
	}
	elapsed := now.Sub(b.last)
	if elapsed > 0 {
		b.tokens += float64(elapsed) / float64(rateInterval)
		if b.tokens > rateBurst {
			b.tokens = rateBurst
		}
		b.last = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// sweepLocked drops buckets that have fully refilled (idle long enough that
// forgetting them changes nothing).
func (l *limiter) sweepLocked(now time.Time) {
	if now.Sub(l.lastSweep) < sweepEvery {
		return
	}
	l.lastSweep = now
	full := rateInterval * rateBurst
	for k, b := range l.buckets {
		if now.Sub(b.last) >= full {
			delete(l.buckets, k)
		}
	}
}

func (l *limiter) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}

// clientIP keys by the TCP peer address. It is the default when no trusted proxy is
// configured; X-Forwarded-For is client-controlled and ignored here.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// middleware rejects over-budget clients with 429.
func (l *limiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := clientIP(r)
		if l.ip != nil {
			key = l.ip(r)
		}
		if !l.allow(key) {
			w.Header().Set("Retry-After", "12")
			httpx.Error(w, http.StatusTooManyRequests, MsgTooManyAttempts)
			return
		}
		next.ServeHTTP(w, r)
	})
}
