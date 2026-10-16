package auth

import (
	"sync"
	"time"
)

// maxTrackedKeys limita la memoria del limitador: al superarlo se descartan
// las entradas caducadas antes de añadir otra.
const maxTrackedKeys = 100_000

// Limiter cuenta intentos fallidos por clave (un email o una IP). Si una clave
// acumula maxFailures fallos dentro de la ventana, queda bloqueada durante
// lockout. Guarda el estado en memoria, así que cada instancia lleva su cuenta.
type Limiter struct {
	maxFailures int
	lockout     time.Duration
	now         func() time.Time

	mu      sync.Mutex
	entries map[string]*attempts
}

type attempts struct {
	failures    int
	windowStart time.Time
	lockedUntil time.Time
}

// NewLimiter crea un limitador. Con maxFailures <= 0 nunca bloquea.
func NewLimiter(maxFailures int, lockout time.Duration) *Limiter {
	return &Limiter{
		maxFailures: maxFailures,
		lockout:     lockout,
		now:         time.Now,
		entries:     make(map[string]*attempts),
	}
}

// Blocked devuelve cuánto falta para que la clave se desbloquee (0 si no lo está).
func (l *Limiter) Blocked(key string) time.Duration {
	if l.maxFailures <= 0 {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	a, ok := l.entries[key]
	if !ok {
		return 0
	}
	if wait := a.lockedUntil.Sub(l.now()); wait > 0 {
		return wait
	}
	return 0
}

// Fail registra un intento fallido y bloquea la clave al llegar al máximo.
func (l *Limiter) Fail(key string) {
	if l.maxFailures <= 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	a, ok := l.entries[key]
	if !ok || l.expired(a, now) {
		if !ok && len(l.entries) >= maxTrackedKeys {
			l.sweep(now)
		}
		a = &attempts{windowStart: now}
		l.entries[key] = a
	}
	a.failures++
	if a.failures >= l.maxFailures {
		a.lockedUntil = now.Add(l.lockout)
		// Tras el bloqueo se empieza a contar de cero.
		a.failures = 0
		a.windowStart = a.lockedUntil
	}
}

// Reset olvida los fallos de la clave (p. ej. tras un login correcto).
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

// expired indica si la ventana de la entrada terminó y no está bloqueada.
func (l *Limiter) expired(a *attempts, now time.Time) bool {
	return !now.Before(a.lockedUntil) && now.Sub(a.windowStart) >= l.lockout
}

func (l *Limiter) sweep(now time.Time) {
	for k, a := range l.entries {
		if l.expired(a, now) {
			delete(l.entries, k)
		}
	}
}
