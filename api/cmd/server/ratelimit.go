package main

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Rate limiting protects the two classes of route called out in GH #13: unauthenticated
// writes (POST /collect) and routes that proxy arbitrary caller-supplied league IDs
// through to Sleeper (GET /league/...). Without a cap, a scripted client can drive enough
// traffic through our IP to trip Sleeper's own ~1,000 req/min limit and risk an IP-wide
// block (see CLAUDE.md), or bloat the visits table on a 1GB-RAM host. These numbers are
// sized for a single low-traffic Pi, not for scale — a normal browser session never
// approaches them, but a tight retry loop or scraper does.
const (
	rateLimitPerSecond rate.Limit = 5
	rateLimitBurst                = 10

	// rateLimitMaxTrackedIPs bounds the visitor map itself, independent of any single
	// IP's rate — otherwise a distributed scan (many source IPs, each individually under
	// the per-IP limit) could still grow this map without bound.
	rateLimitMaxTrackedIPs = 5000
)

// ipRateLimiter grants each client IP its own token-bucket limiter. Entries are created
// lazily on first use; the map is capped at rateLimitMaxTrackedIPs, evicting the
// least-recently-seen IP to make room for a new one once full.
type ipRateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newIPRateLimiter() *ipRateLimiter {
	return &ipRateLimiter{visitors: make(map[string]*visitor)}
}

func (l *ipRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	v, ok := l.visitors[ip]
	if !ok {
		if len(l.visitors) >= rateLimitMaxTrackedIPs {
			l.evictOldestLocked()
		}
		v = &visitor{limiter: rate.NewLimiter(rateLimitPerSecond, rateLimitBurst)}
		l.visitors[ip] = v
	}
	v.lastSeen = time.Now()
	return v.limiter.Allow()
}

// evictOldestLocked removes the least-recently-seen visitor to make room for a new one.
// Caller must hold l.mu.
func (l *ipRateLimiter) evictOldestLocked() {
	var oldestIP string
	var oldestSeen time.Time
	first := true
	for ip, v := range l.visitors {
		if first || v.lastSeen.Before(oldestSeen) {
			oldestIP, oldestSeen = ip, v.lastSeen
			first = false
		}
	}
	if !first {
		delete(l.visitors, oldestIP)
	}
}

// middleware rejects requests once the caller's IP has exhausted its token bucket.
func (l *ipRateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(clientIP(r)) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP extracts the request's source IP, stripping the port. Falls back to the raw
// RemoteAddr if it isn't in host:port form.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
