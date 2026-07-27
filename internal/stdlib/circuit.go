package stdlib

import (
	"net/url"
	"sync"
	"time"
)

// Per-host circuit breaker for outbound HTTP (opt-in via request opts).
// States: closed → open (after threshold failures) → half-open after cooldown.

type circuitState int

const (
	circuitClosed circuitState = iota
	circuitOpen
	circuitHalfOpen
)

type hostCircuit struct {
	state         circuitState
	failures      int
	openedAt      time.Time
	halfOpenProbe bool // only one in-flight probe when half-open
}

type circuitBook struct {
	mu    sync.Mutex
	hosts map[string]*hostCircuit
}

var globalCircuits = &circuitBook{hosts: make(map[string]*hostCircuit)}

func hostKey(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL
	}
	return u.Host
}

// circuitAllow returns false when the circuit is open (fail-fast).
// On half-open it allows a single probe.
func (b *circuitBook) allow(host string, cooldown time.Duration) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	h := b.hosts[host]
	if h == nil {
		return true
	}
	switch h.state {
	case circuitClosed:
		return true
	case circuitOpen:
		if time.Since(h.openedAt) >= cooldown {
			h.state = circuitHalfOpen
			h.halfOpenProbe = true
			return true
		}
		return false
	case circuitHalfOpen:
		if h.halfOpenProbe {
			// another probe already in flight
			return false
		}
		h.halfOpenProbe = true
		return true
	default:
		return true
	}
}

func (b *circuitBook) success(host string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	h := b.hosts[host]
	if h == nil {
		return
	}
	h.failures = 0
	h.state = circuitClosed
	h.halfOpenProbe = false
}

func (b *circuitBook) failure(host string, threshold int, cooldown time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	h := b.hosts[host]
	if h == nil {
		h = &hostCircuit{}
		b.hosts[host] = h
	}
	h.halfOpenProbe = false
	h.failures++
	if h.state == circuitHalfOpen || h.failures >= threshold {
		h.state = circuitOpen
		h.openedAt = time.Now()
	}
}

// circuitOpenMsg is returned when fail-fast.
const circuitOpenMsg = "circuit open for host"
