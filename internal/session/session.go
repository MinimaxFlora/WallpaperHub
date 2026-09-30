// Package session derives a stable client identity for mode=session requests.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

// CookieName is the cookie that keeps a client's session stable across
// requests when no explicit session id is supplied.
const CookieName = "wallpaper_session"

// Derive resolves the session id for a request. An explicit query value wins,
// then the cookie, then a freshly generated id. The boolean result reports
// whether the caller should set a cookie, which happens only when no id was
// supplied by the client.
func Derive(r *http.Request, query string) (id string, setCookie bool) {
	if v := strings.TrimSpace(query); v != "" {
		return v, false
	}
	if c, err := r.Cookie(CookieName); err == nil {
		if v := strings.TrimSpace(c.Value); v != "" {
			return v, false
		}
	}
	return NewID(), true
}

// NewID returns a random hexadecimal identifier.
func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "anonymous"
	}
	return hex.EncodeToString(b[:])
}
