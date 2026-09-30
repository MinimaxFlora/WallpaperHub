package selection

import "testing"

func TestParseModeFallsBackToRandom(t *testing.T) {
	cases := map[string]Mode{
		"seed":    ModeSeed,
		"daily":   ModeDaily,
		"session": ModeSession,
		"random":  ModeRandom,
		"":        ModeRandom,
		"bogus":   ModeRandom,
	}
	for raw, want := range cases {
		if got := ParseMode(raw); got != want {
			t.Errorf("ParseMode(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestIndexDeterministicForSeededModes(t *testing.T) {
	for _, mode := range []Mode{ModeSeed, ModeDaily, ModeSession} {
		first := Index(mode, "key", 7)
		for i := 0; i < 50; i++ {
			if got := Index(mode, "key", 7); got != first {
				t.Fatalf("Index(%q) not stable: %d != %d", mode, got, first)
			}
		}
		if first < 0 || first >= 7 {
			t.Fatalf("Index(%q) = %d, out of range", mode, first)
		}
	}
}

func TestIndexSingleAndEmptyCandidateSets(t *testing.T) {
	for _, mode := range []Mode{ModeRandom, ModeSeed, ModeDaily, ModeSession} {
		if got := Index(mode, "k", 1); got != 0 {
			t.Errorf("Index(%q, n=1) = %d, want 0", mode, got)
		}
		if got := Index(mode, "k", 0); got != 0 {
			t.Errorf("Index(%q, n=0) = %d, want 0", mode, got)
		}
	}
}

func TestIndexRandomStaysInRange(t *testing.T) {
	for i := 0; i < 1000; i++ {
		if got := Index(ModeRandom, "", 5); got < 0 || got >= 5 {
			t.Fatalf("random index = %d, out of range", got)
		}
	}
}

func TestKeyUsesModeSpecificInput(t *testing.T) {
	if got := Key(ModeSeed, "s", "d", "x"); got != "s" {
		t.Errorf("seed key = %q, want s", got)
	}
	if got := Key(ModeDaily, "s", "2026-01-02", "x"); got != "2026-01-02" {
		t.Errorf("daily key = %q", got)
	}
	if got := Key(ModeSession, "s", "d", "session-id"); got != "session-id" {
		t.Errorf("session key = %q", got)
	}
	if got := Key(ModeRandom, "s", "d", "x"); got != "" {
		t.Errorf("random key = %q, want empty", got)
	}
}
