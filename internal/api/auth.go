package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	sessionCookieName = "updog_session"
	sessionTTL        = 7 * 24 * time.Hour
)

type authManager struct {
	enabled      bool
	passwordHash [32]byte

	mu       sync.RWMutex
	sessions map[string]time.Time
}

func newAuthManager(password string) *authManager {
	a := &authManager{
		enabled:  password != "",
		sessions: make(map[string]time.Time),
	}
	if a.enabled {
		a.passwordHash = sha256.Sum256([]byte(password))
	}
	return a
}

func (a *authManager) verifyPassword(candidate string) bool {
	if !a.enabled {
		return true
	}
	sum := sha256.Sum256([]byte(candidate))
	return subtle.ConstantTimeCompare(a.passwordHash[:], sum[:]) == 1
}

func (a *authManager) createSession() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)

	now := time.Now().UTC()
	a.mu.Lock()
	// Opportunistically prune expired sessions
	for k, exp := range a.sessions {
		if now.After(exp) {
			delete(a.sessions, k)
		}
	}
	a.sessions[token] = now.Add(sessionTTL)
	a.mu.Unlock()
	return token
}

func (a *authManager) deleteSession(token string) {
	a.mu.Lock()
	delete(a.sessions, token)
	a.mu.Unlock()
}

func (a *authManager) isAuthenticated(r *http.Request) bool {
	if !a.enabled {
		return true
	}

	// Allow Authorization: Bearer <password> for scripts/CLI
	if authz := r.Header.Get("Authorization"); strings.HasPrefix(authz, "Bearer ") {
		if a.verifyPassword(strings.TrimPrefix(authz, "Bearer ")) {
			return true
		}
	}

	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}

	a.mu.RLock()
	exp, ok := a.sessions[cookie.Value]
	a.mu.RUnlock()
	if !ok || time.Now().UTC().After(exp) {
		if ok {
			a.deleteSession(cookie.Value)
		}
		return false
	}
	return true
}

// --- Lightweight Fixed-Window Rate Limiter ---

type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]*rateEntry
}

type rateEntry struct {
	count   int
	resetAt time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		limit:   limit,
		window:  window,
		entries: make(map[string]*rateEntry),
	}
}

func (rl *rateLimiter) Allow(key string) bool {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if len(rl.entries) > 5000 {
		for k, e := range rl.entries {
			if now.After(e.resetAt) {
				delete(rl.entries, k)
			}
		}
	}

	e, ok := rl.entries[key]
	if !ok || now.After(e.resetAt) {
		rl.entries[key] = &rateEntry{count: 1, resetAt: now.Add(rl.window)}
		return true
	}
	if e.count >= rl.limit {
		return false
	}
	e.count++
	return true
}

func clientIP(r *http.Request) string {
	if cf := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cf != "" {
		return cf
	}
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func maskSecretURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if u, err := url.Parse(raw); err == nil && u.Scheme != "" && u.Host != "" {
		tail := ""
		if len(raw) >= 4 {
			tail = raw[len(raw)-4:]
		}
		return u.Scheme + "://" + u.Host + "/••••" + tail
	}
	if len(raw) <= 6 {
		return "••••"
	}
	return "••••" + raw[len(raw)-4:]
}
