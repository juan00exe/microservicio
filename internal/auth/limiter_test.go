package auth

import (
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestLimiter(max int, lockout time.Duration) (*Limiter, *fakeClock) {
	clock := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := NewLimiter(max, lockout)
	l.now = clock.now
	return l, clock
}

func TestLimiterBlocksAfterMaxFailures(t *testing.T) {
	l, clock := newTestLimiter(3, 15*time.Minute)

	for i := range 2 {
		l.Fail("k")
		if l.Blocked("k") != 0 {
			t.Fatalf("blocked after %d failures", i+1)
		}
	}
	l.Fail("k")
	if got := l.Blocked("k"); got != 15*time.Minute {
		t.Fatalf("Blocked = %v, want 15m", got)
	}
	if l.Blocked("otra") != 0 {
		t.Fatal("other keys must not be blocked")
	}

	clock.advance(10 * time.Minute)
	if got := l.Blocked("k"); got != 5*time.Minute {
		t.Fatalf("Blocked = %v, want 5m", got)
	}

	clock.advance(5 * time.Minute)
	if l.Blocked("k") != 0 {
		t.Fatal("still blocked after lockout")
	}
	// Tras el bloqueo se empieza de cero: hacen falta otros 3 fallos.
	l.Fail("k")
	l.Fail("k")
	if l.Blocked("k") != 0 {
		t.Fatal("blocked too early after lockout ended")
	}
}

func TestLimiterWindowExpires(t *testing.T) {
	l, clock := newTestLimiter(3, 15*time.Minute)
	l.Fail("k")
	l.Fail("k")
	// Fuera de la ventana los fallos anteriores ya no cuentan.
	clock.advance(15 * time.Minute)
	l.Fail("k")
	if l.Blocked("k") != 0 {
		t.Fatal("old failures should have expired")
	}
}

func TestLimiterReset(t *testing.T) {
	l, _ := newTestLimiter(3, time.Minute)
	l.Fail("k")
	l.Fail("k")
	l.Reset("k")
	l.Fail("k")
	l.Fail("k")
	if l.Blocked("k") != 0 {
		t.Fatal("Reset should clear failures")
	}
}

func TestLimiterDisabled(t *testing.T) {
	l, _ := newTestLimiter(0, time.Minute)
	for range 100 {
		l.Fail("k")
	}
	if l.Blocked("k") != 0 {
		t.Fatal("disabled limiter must never block")
	}
}
