package logx

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewCapturesErrorDetail(t *testing.T) {
	logger, ring, closer := New(slog.LevelInfo, "")
	defer closer()

	logger.Error("load issues", "err", errors.New("gh: not authenticated"))

	got := ring.Entries()
	if len(got) != 1 {
		t.Fatalf("Entries() = %d entries, want 1", len(got))
	}
	e := got[0]
	if e.Msg != "load issues" {
		t.Errorf("Msg = %q, want %q", e.Msg, "load issues")
	}
	if e.Level != slog.LevelError {
		t.Errorf("Level = %v, want ERROR", e.Level)
	}
	if e.Detail != "gh: not authenticated" {
		t.Errorf("Detail = %q, want bare error message", e.Detail)
	}
	if e.Seq != 1 {
		t.Errorf("Seq = %d, want 1", e.Seq)
	}
}

func TestKeyedAttrsRenderInDetail(t *testing.T) {
	logger, ring, _ := New(slog.LevelDebug, "")

	logger.Debug("loaded", "resource", "issues", "n", 3)

	e := ring.Entries()[0]
	// Order follows the call site; both attrs are keyed.
	if !strings.Contains(e.Detail, "resource=issues") || !strings.Contains(e.Detail, "n=3") {
		t.Errorf("Detail = %q, want keyed resource/n attrs", e.Detail)
	}
}

func TestLevelGating(t *testing.T) {
	logger, ring, _ := New(slog.LevelWarn, "")

	logger.Debug("d")
	logger.Info("i")
	logger.Warn("w")
	logger.Error("e")

	got := ring.Entries()
	if len(got) != 2 {
		t.Fatalf("Entries() = %d, want 2 (warn+error only)", len(got))
	}
	// Newest first: error then warn.
	if got[0].Msg != "e" || got[1].Msg != "w" {
		t.Errorf("entries = %q, %q; want e, w", got[0].Msg, got[1].Msg)
	}
}

func TestEntriesNewestFirstAndSeq(t *testing.T) {
	logger, ring, _ := New(slog.LevelInfo, "")
	for _, m := range []string{"a", "b", "c"} {
		logger.Info(m)
	}
	got := ring.Entries()
	if len(got) != 3 {
		t.Fatalf("Entries() = %d, want 3", len(got))
	}
	if got[0].Msg != "c" || got[2].Msg != "a" {
		t.Errorf("order = %q..%q, want newest-first c..a", got[0].Msg, got[2].Msg)
	}
	if got[0].Seq <= got[2].Seq {
		t.Errorf("Seq not monotonic: newest %d, oldest %d", got[0].Seq, got[2].Seq)
	}
}

func TestRingCapEvictsOldest(t *testing.T) {
	ring := &Ring{store: &ringStore{cap: 3}}
	logger := slog.New(&multiHandler{min: slog.LevelInfo, handlers: []slog.Handler{ring}})
	for _, m := range []string{"a", "b", "c", "d", "e"} {
		logger.Info(m)
	}
	got := ring.Entries()
	if len(got) != 3 {
		t.Fatalf("Entries() = %d, want cap 3", len(got))
	}
	// Newest three retained: e, d, c.
	if got[0].Msg != "e" || got[2].Msg != "c" {
		t.Errorf("retained = %q..%q, want e..c", got[0].Msg, got[2].Msg)
	}
	if ring.MaxSeq() != 5 {
		t.Errorf("MaxSeq() = %d, want 5 (monotonic across eviction)", ring.MaxSeq())
	}
}

func TestCountSince(t *testing.T) {
	logger, ring, _ := New(slog.LevelDebug, "")
	logger.Info("i1")  // seq 1
	logger.Warn("w1")  // seq 2
	logger.Error("e1") // seq 3

	seen := ring.MaxSeq() // 3
	logger.Info("i2")     // seq 4
	logger.Warn("w2")     // seq 5

	// Unseen warn+error since `seen`: only w2.
	if n := ring.CountSince(slog.LevelWarn, seen); n != 1 {
		t.Errorf("CountSince(WARN, %d) = %d, want 1", seen, n)
	}
	// From the start, warn+error overall: w1, e1, w2.
	if n := ring.CountSince(slog.LevelWarn, 0); n != 3 {
		t.Errorf("CountSince(WARN, 0) = %d, want 3", n)
	}
	// Errors only, from the start: e1.
	if n := ring.CountSince(slog.LevelError, 0); n != 1 {
		t.Errorf("CountSince(ERROR, 0) = %d, want 1", n)
	}
}

func TestFileSinkWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sam-test.log")
	logger, _, closer := New(slog.LevelInfo, path)

	logger.Error("boom", "err", errors.New("kaboom"))
	if err := closer(); err != nil {
		t.Fatalf("closer() = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile = %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "boom") || !strings.Contains(out, "kaboom") {
		t.Errorf("file sink missing message/detail: %q", out)
	}
	if !strings.Contains(out, "level=ERROR") {
		t.Errorf("file sink missing level: %q", out)
	}
}

func TestFanoutHitsRingAndFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fan.log")
	logger, ring, closer := New(slog.LevelInfo, path)
	defer closer()

	logger.Info("both")

	if len(ring.Entries()) != 1 {
		t.Errorf("ring did not capture the record")
	}
	closer()
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "both") {
		t.Errorf("file did not capture the record: %q", data)
	}
}

func TestNilRingReadersAreSafe(t *testing.T) {
	var r *Ring
	if got := r.Entries(); got != nil {
		t.Errorf("nil Entries() = %v, want nil", got)
	}
	if got := r.MaxSeq(); got != 0 {
		t.Errorf("nil MaxSeq() = %d, want 0", got)
	}
	if got := r.CountSince(slog.LevelWarn, 0); got != 0 {
		t.Errorf("nil CountSince() = %d, want 0", got)
	}
}
