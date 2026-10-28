package app

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

type session struct {
	User, CSRF string
	Expires    time.Time
}
type attempt struct {
	Start time.Time
	Count int
}
type Auth struct {
	mu           sync.Mutex
	sessions     map[[32]byte]session
	attempts     map[string]attempt
	passwordSlot chan struct{}
	Secure       bool
}

func NewAuth(secure bool) *Auth {
	return &Auth{sessions: map[[32]byte]session{}, attempts: map[string]attempt{}, passwordSlot: make(chan struct{}, 1), Secure: secure}
}
func (a *Auth) allowed(ip string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	for k, v := range a.attempts {
		if now.Sub(v.Start) > 10*time.Minute {
			delete(a.attempts, k)
		}
	}
	v := a.attempts[ip]
	if now.Sub(v.Start) > time.Minute {
		v = attempt{Start: now}
	}
	v.Count++
	if len(a.attempts) >= 1024 && v.Count == 1 {
		return false
	}
	a.attempts[ip] = v
	return v.Count <= 5
}
func (a *Auth) create(w http.ResponseWriter, user string) session {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	for k, v := range a.sessions {
		if v.Expires.Before(now) {
			delete(a.sessions, k)
		}
	}
	if len(a.sessions) >= 128 {
		for k := range a.sessions {
			delete(a.sessions, k)
			break
		}
	}
	raw := token(32)
	s := session{User: user, CSRF: token(24), Expires: now.Add(12 * time.Hour)}
	a.sessions[sha256.Sum256([]byte(raw))] = s
	http.SetCookie(w, &http.Cookie{Name: "wgs_session", Value: raw, Path: "/", HttpOnly: true, Secure: a.Secure, SameSite: http.SameSiteStrictMode, MaxAge: 12 * 3600})
	return s
}
func (a *Auth) get(r *http.Request) (session, bool) {
	c, e := r.Cookie("wgs_session")
	if e != nil {
		return session{}, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	key := sha256.Sum256([]byte(c.Value))
	s, ok := a.sessions[key]
	if ok && time.Now().Before(s.Expires) {
		return s, true
	}
	delete(a.sessions, key)
	return session{}, false
}
func (a *Auth) logout(w http.ResponseWriter, r *http.Request) {
	if c, e := r.Cookie("wgs_session"); e == nil {
		a.mu.Lock()
		delete(a.sessions, sha256.Sum256([]byte(c.Value)))
		a.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "wgs_session", Value: "", Path: "/", HttpOnly: true, Secure: a.Secure, SameSite: http.SameSiteStrictMode, MaxAge: -1})
}
func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
func hashPassword(password string) (string, error) {
	if len(password) < 12 || len(password) > 256 {
		return "", errors.New("parola 12–256 karakter olmalı")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, 3, 32*1024, 1, 32)
	return fmt.Sprintf("$argon2id$v=19$m=32768,t=3,p=1$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}
func verifyPassword(encoded, password string) bool {
	if len(password) > 256 {
		return false
	}
	p := strings.Split(encoded, "$")
	if len(p) != 6 || p[1] != "argon2id" || p[2] != "v=19" || p[3] != "m=32768,t=3,p=1" {
		return false
	}
	salt, e := base64.RawStdEncoding.DecodeString(p[4])
	if e != nil || len(salt) != 16 {
		return false
	}
	expected, e := base64.RawStdEncoding.DecodeString(p[5])
	if e != nil || len(expected) != 32 {
		return false
	}
	key := argon2.IDKey([]byte(password), salt, 3, 32*1024, 1, 32)
	return subtle.ConstantTimeCompare(key, expected) == 1
}
func (s *HTTPServer) authMutation(w http.ResponseWriter, r *http.Request) (session, bool) {
	user, ok := s.auth.get(r)
	if !ok {
		apiError(w, 401, "oturum açmanız gerekiyor")
		return session{}, false
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(user.CSRF)) != 1 {
			apiError(w, 403, "istek doğrulanamadı; sayfayı yenileyin")
			return session{}, false
		}
	}
	return user, true
}
func parsePositiveInt(v string, fallback int) int {
	n, e := strconv.Atoi(v)
	if e != nil || n < 1 {
		return fallback
	}
	return n
}
