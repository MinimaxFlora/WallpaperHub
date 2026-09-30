package server

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"wallpaper-api/internal/errs"
)

// withCORS adds permissive CORS headers and short-circuits preflight requests.
func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, If-None-Match, Range")
		w.Header().Set("Access-Control-Expose-Headers", "ETag, Content-Range, Accept-Ranges, Retry-After, X-Cache")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withRateLimit enforces the per-client request budget.
func (s *Server) withRateLimit(next http.Handler) http.Handler {
	if s.limiter == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok, retryAfter := s.limiter.allow(clientIP(r), s.now())
		if !ok {
			seconds := int(math.Ceil(retryAfter.Seconds()))
			if seconds < 1 {
				seconds = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(seconds))
			s.writeErr(w, errs.TooMany("rate_limited", "too many requests"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withLogging records one line per request, including the cache outcome.
func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := s.now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration", s.now().Sub(start).String(),
			"cache", rec.Header().Get("X-Cache"),
			"remote", clientIP(r),
		)
	})
}

// withRecover converts a panic into a structured 500 response.
func (s *Server) withRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.logger.Error("panic recovered", "path", r.URL.Path, "panic", rec)
				s.writeErr(w, errs.New(http.StatusInternalServerError, "internal", "internal error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.status = code
	rec.ResponseWriter.WriteHeader(code)
}

// rateLimiter is a per-client sliding window counter.
type rateLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{hits: make(map[string][]time.Time), limit: limit, window: window}
}

// allow records a hit and reports whether the request stays within budget.
func (l *rateLimiter) allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := now.Add(-l.window)
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.limit {
		retryAfter := kept[0].Add(l.window).Sub(now)
		if retryAfter < 0 {
			retryAfter = 0
		}
		l.hits[key] = kept
		return false, retryAfter
	}
	l.hits[key] = append(kept, now)
	return true, 0
}

// hotlinkAllowed reports whether the request origin may fetch image bytes.
func (s *Server) hotlinkAllowed(r *http.Request) bool {
	if len(s.cfg.HotlinkAllowlist) == 0 {
		return true
	}
	host := requestOriginHost(r)
	if host == "" {
		return true
	}
	for _, allow := range s.cfg.HotlinkAllowlist {
		if hostMatches(allow, host) {
			return true
		}
	}
	return false
}

func requestOriginHost(r *http.Request) string {
	for _, header := range []string{"Origin", "Referer"} {
		raw := strings.TrimSpace(r.Header.Get(header))
		if raw == "" {
			continue
		}
		if host := hostFromURL(raw); host != "" {
			return host
		}
	}
	return ""
}

func hostFromURL(raw string) string {
	if i := strings.Index(raw, "://"); i >= 0 {
		raw = raw[i+3:]
	}
	if i := strings.IndexAny(raw, "/?#"); i >= 0 {
		raw = raw[:i]
	}
	raw = strings.TrimSpace(raw)
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	}
	return strings.ToLower(raw)
}

// hostMatches reports whether host satisfies an allowlist entry, where a "*.x"
// entry matches x and any subdomain of x.
func hostMatches(pattern, host string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	if pattern == "" {
		return false
	}
	if suffix, ok := strings.CutPrefix(pattern, "*."); ok {
		return host == suffix || strings.HasSuffix(host, "."+suffix)
	}
	return pattern == host
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if first := strings.TrimSpace(strings.Split(fwd, ",")[0]); first != "" {
			return first
		}
	}
	if real := strings.TrimSpace(r.Header.Get("X-Real-IP")); real != "" {
		return real
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
