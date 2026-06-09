// Package logx is sam's logging glue. It is a thin layer over the standard
// library's log/slog: the application logs through a plain *slog.Logger,
// and logx adds the two things slog does not provide on its own — an
// in-memory ring of recent records the TUI can render in its `:logs` view,
// and a constructor that tees that ring alongside an optional file sink.
//
// The ring is implemented as a slog.Handler (slog's documented extension
// point), so no bespoke logging API is introduced; callers use the normal
// logger.Info/Warn/Error methods.
package logx

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ringCap bounds the in-memory ring; older entries are evicted once it is
// exceeded. The file sink keeps the full session, so this only caps what
// the `:logs` view can scroll.
const ringCap = 500

// Entry is one captured log record, flattened for the TUI. Msg is the
// record message; Detail is its attributes rendered to text (so an
// `logger.Error("…", "err", err)` lands err.Error() in Detail). Seq is a
// monotonic id used to tell new entries from already-seen ones.
type Entry struct {
	Seq    int
	Time   time.Time
	Level  slog.Level
	Msg    string
	Detail string
}

// DefaultPath is where New tees logs in normal operation: a per-pid file in
// the OS temp dir. Per-pid so concurrent sam processes don't clobber one
// another; ephemeral on purpose (no rotation, gone on reboot).
func DefaultPath() string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("sam-%d.log", os.Getpid()))
}

// New builds a logger whose records fan out to an in-memory Ring (returned
// for the TUI) and, when path is non-empty and openable, a text file sink.
// min is the minimum level emitted. The returned func closes the file sink
// (a no-op when there is none). A path that fails to open degrades to
// ring-only rather than failing — logging must never take sam down.
func New(min slog.Level, path string) (*slog.Logger, *Ring, func() error) {
	ring := &Ring{store: &ringStore{cap: ringCap}}
	handlers := []slog.Handler{ring}
	closer := func() error { return nil }

	if path != "" {
		if f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			handlers = append(handlers, slog.NewTextHandler(f, &slog.HandlerOptions{Level: min}))
			closer = f.Close
		}
	}

	return slog.New(&multiHandler{min: min, handlers: handlers}), ring, closer
}

// multiHandler fans a record out to several handlers, gating them all at a
// shared minimum level. The Logger consults Enabled before Handle, so the
// min check here is what filters records below the threshold.
type multiHandler struct {
	min      slog.Level
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(_ context.Context, l slog.Level) bool { return l >= m.min }

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if err := h.Handle(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	hs := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		hs[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{min: m.min, handlers: hs}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	hs := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		hs[i] = h.WithGroup(name)
	}
	return &multiHandler{min: m.min, handlers: hs}
}

// ringStore is the shared, mutex-guarded backing for a Ring and any clones
// it spawns via WithAttrs, so every write lands in the one buffer the TUI
// reads.
type ringStore struct {
	mu  sync.Mutex
	buf []Entry
	cap int
	seq int
}

// Ring is a slog.Handler that retains the most recent records in memory for
// the TUI. Its reader methods (Entries/MaxSeq/CountSince) are safe for
// concurrent use and tolerate a nil receiver, so callers without a logger
// can treat the ring as empty.
type Ring struct {
	store *ringStore
	attrs []slog.Attr // preset attrs from WithAttrs, prepended to each record's detail
}

func (r *Ring) Enabled(context.Context, slog.Level) bool { return true }

func (r *Ring) Handle(_ context.Context, rec slog.Record) error {
	detail := r.detail(rec)
	r.store.mu.Lock()
	r.store.seq++
	r.store.buf = append(r.store.buf, Entry{
		Seq:    r.store.seq,
		Time:   rec.Time,
		Level:  rec.Level,
		Msg:    rec.Message,
		Detail: detail,
	})
	if len(r.store.buf) > r.store.cap {
		r.store.buf = r.store.buf[len(r.store.buf)-r.store.cap:]
	}
	r.store.mu.Unlock()
	return nil
}

func (r *Ring) WithAttrs(attrs []slog.Attr) slog.Handler {
	na := make([]slog.Attr, 0, len(r.attrs)+len(attrs))
	na = append(na, r.attrs...)
	na = append(na, attrs...)
	return &Ring{store: r.store, attrs: na}
}

// WithGroup is a no-op: the ring renders attrs flat and sam never groups.
func (r *Ring) WithGroup(string) slog.Handler { return r }

// detail renders a record's attributes (preset + call-site) into the
// Detail text. An `err`/`detail` attr is shown bare (its value is already a
// full message); everything else is keyed.
func (r *Ring) detail(rec slog.Record) string {
	parts := make([]string, 0, len(r.attrs)+rec.NumAttrs())
	for _, a := range r.attrs {
		parts = append(parts, attrString(a))
	}
	rec.Attrs(func(a slog.Attr) bool {
		parts = append(parts, attrString(a))
		return true
	})
	return strings.Join(parts, "\n")
}

func attrString(a slog.Attr) string {
	if a.Key == "err" || a.Key == "detail" {
		return a.Value.String()
	}
	return a.Key + "=" + a.Value.String()
}

// Entries returns a snapshot of the retained entries, newest first.
func (r *Ring) Entries() []Entry {
	if r == nil {
		return nil
	}
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	out := make([]Entry, len(r.store.buf))
	for i, e := range r.store.buf {
		out[len(r.store.buf)-1-i] = e
	}
	return out
}

// MaxSeq returns the sequence id of the most recent record (0 if none),
// used as the "seen" baseline for the unseen-entry badge.
func (r *Ring) MaxSeq() int {
	if r == nil {
		return 0
	}
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	return r.store.seq
}

// CountSince reports how many retained entries at or above min have a Seq
// greater than seq — i.e. how many qualifying entries arrived since the
// caller last looked.
func (r *Ring) CountSince(min slog.Level, seq int) int {
	if r == nil {
		return 0
	}
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	n := 0
	for _, e := range r.store.buf {
		if e.Seq > seq && e.Level >= min {
			n++
		}
	}
	return n
}
