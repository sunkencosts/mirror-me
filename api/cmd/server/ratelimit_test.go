package main

import (
	"fmt"
	"testing"
)

func TestIPRateLimiter_AllowsUpToBurstThenBlocks(t *testing.T) {
	l := newIPRateLimiter()

	for i := 0; i < rateLimitBurst; i++ {
		if !l.allow("1.2.3.4") {
			t.Fatalf("request %d: expected allow within burst of %d", i, rateLimitBurst)
		}
	}
	if l.allow("1.2.3.4") {
		t.Fatal("expected request beyond burst to be denied")
	}
}

func TestIPRateLimiter_PerIPIndependent(t *testing.T) {
	l := newIPRateLimiter()

	for i := 0; i < rateLimitBurst; i++ {
		if !l.allow("1.1.1.1") {
			t.Fatalf("ip1 request %d: expected allow", i)
		}
	}
	if l.allow("1.1.1.1") {
		t.Fatal("expected ip1 to be exhausted")
	}

	// A different IP has its own bucket and should be unaffected by ip1's usage.
	if !l.allow("2.2.2.2") {
		t.Fatal("expected a fresh IP to have its own budget")
	}
}

func TestIPRateLimiter_EvictsOldestWhenTrackedIPsFull(t *testing.T) {
	l := newIPRateLimiter()

	for i := 0; i < rateLimitMaxTrackedIPs; i++ {
		l.allow(ipFromInt(i))
	}
	if len(l.visitors) != rateLimitMaxTrackedIPs {
		t.Fatalf("expected %d tracked IPs, got %d", rateLimitMaxTrackedIPs, len(l.visitors))
	}

	// The very first IP tracked is the least-recently-seen; inserting one more distinct
	// IP should evict it rather than growing the map further.
	firstIP := ipFromInt(0)
	l.allow(ipFromInt(rateLimitMaxTrackedIPs))

	if len(l.visitors) != rateLimitMaxTrackedIPs {
		t.Fatalf("expected map to stay capped at %d, got %d", rateLimitMaxTrackedIPs, len(l.visitors))
	}
	if _, tracked := l.visitors[firstIP]; tracked {
		t.Error("expected the least-recently-seen IP to be evicted")
	}
}

func ipFromInt(n int) string {
	return fmt.Sprintf("10.0.0.%d", n)
}
