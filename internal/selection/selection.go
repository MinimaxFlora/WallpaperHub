// Package selection implements the four wallpaper selection modes: random,
// seed, daily and session.
package selection

import (
	"crypto/rand"
	"hash/fnv"
	"math/big"
)

// Mode decides which wallpaper a request returns.
type Mode string

// Supported selection modes.
const (
	ModeRandom  Mode = "random"
	ModeSeed    Mode = "seed"
	ModeDaily   Mode = "daily"
	ModeSession Mode = "session"
)

// ParseMode maps a raw query value to a Mode. Unknown values fall back to
// random, as required by the API contract.
func ParseMode(raw string) Mode {
	switch Mode(raw) {
	case ModeSeed:
		return ModeSeed
	case ModeDaily:
		return ModeDaily
	case ModeSession:
		return ModeSession
	default:
		return ModeRandom
	}
}

// Key builds the mode-specific selection key. Random ignores the key.
func Key(mode Mode, seed, date, session string) string {
	switch mode {
	case ModeSeed:
		return seed
	case ModeDaily:
		return date
	case ModeSession:
		return session
	default:
		return ""
	}
}

// Index returns an index in [0, n) for the given mode and key. Random uses a
// cryptographically secure source; the deterministic modes hash the key so
// that equal keys and candidate counts yield the same index.
func Index(mode Mode, key string, n int) int {
	if n <= 0 {
		return 0
	}
	switch mode {
	case ModeSeed, ModeDaily, ModeSession:
		return int(hash32(key) % uint32(n))
	default:
		return randomIndex(n)
	}
}

// hash32 computes the FNV-1a 32-bit hash of key.
func hash32(key string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return h.Sum32()
}

// randomIndex returns a uniform random index in [0, n).
func randomIndex(n int) int {
	if n <= 1 {
		return 0
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0
	}
	return int(v.Int64())
}
