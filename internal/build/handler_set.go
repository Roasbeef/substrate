package build

import (
	"context"
	"log/slog"

	"github.com/btcsuite/btclog"
	btclogv2 "github.com/btcsuite/btclog/v2"
)

// HandlerSet is an implementation of btclog.Handler that fans out log records
// to multiple underlying handlers. This enables dual-stream logging where
// messages go to both the console and a rotating log file.
type HandlerSet struct {
	level btclog.Level
	set   []btclogv2.Handler
}

// NewHandlerSet constructs a new HandlerSet from the given handlers. All
// handlers are initialized to the Info log level.
func NewHandlerSet(handlers ...btclogv2.Handler) *HandlerSet {
	h := &HandlerSet{
		set:   handlers,
		level: btclog.LevelInfo,
	}
	h.SetLevel(h.level)

	return h
}

// Enabled reports whether the handler handles records at the given level.
//
// NOTE: this is part of the slog.Handler interface.
func (h *HandlerSet) Enabled(ctx context.Context,
	level slog.Level) bool {

	for _, handler := range h.set {
		if !handler.Enabled(ctx, level) {
			return false
		}
	}

	return true
}

// Handle handles the Record by dispatching to all underlying handlers.
//
// NOTE: this is part of the slog.Handler interface.
func (h *HandlerSet) Handle(ctx context.Context,
	record slog.Record) error {

	for _, handler := range h.set {
		if err := handler.Handle(ctx, record); err != nil {
			return err
		}
	}

	return nil
}

// WithAttrs returns a new Handler whose attributes consist of both the
// receiver's attributes and the arguments.
//
// NOTE: this is part of the slog.Handler interface.
func (h *HandlerSet) WithAttrs(attrs []slog.Attr) slog.Handler {
	newSet := &reducedSet{set: make([]slog.Handler, len(h.set))}
	for i, handler := range h.set {
		newSet.set[i] = handler.WithAttrs(attrs)
	}

	return newSet
}

// WithGroup returns a new Handler with the given group appended to the
// receiver's existing groups.
//
// NOTE: this is part of the slog.Handler interface.
func (h *HandlerSet) WithGroup(name string) slog.Handler {
	newSet := &reducedSet{set: make([]slog.Handler, len(h.set))}
	for i, handler := range h.set {
		newSet.set[i] = handler.WithGroup(name)
	}

	return newSet
}

// SubSystem creates a new Handler with the given sub-system tag.
//
// NOTE: this is part of the btclog.Handler interface.
func (h *HandlerSet) SubSystem(tag string) btclogv2.Handler {
	newSet := &HandlerSet{set: make([]btclogv2.Handler, len(h.set))}
	for i, handler := range h.set {
		newSet.set[i] = handler.SubSystem(tag)
	}

	return newSet
}

// SetLevel changes the logging level on all underlying handlers.
//
// NOTE: this is part of the btclog.Handler interface.
func (h *HandlerSet) SetLevel(level btclog.Level) {
	for _, handler := range h.set {
		handler.SetLevel(level)
	}
	h.level = level
}

// Level returns the current logging level.
//
// NOTE: this is part of the btclog.Handler interface.
func (h *HandlerSet) Level() btclog.Level {
	return h.level
}

// WithPrefix returns a copy of the Handler but with the given string
// prefixed to each log message.
//
// NOTE: this is part of the btclog.Handler interface.
func (h *HandlerSet) WithPrefix(prefix string) btclogv2.Handler {
	newSet := &HandlerSet{
		set: make([]btclogv2.Handler, len(h.set)),
	}
	for i, handler := range h.set {
		newSet.set[i] = handler.WithPrefix(prefix)
	}

	return newSet
}

// Ensure HandlerSet implements btclog.Handler at compile time.
var _ btclogv2.Handler = (*HandlerSet)(nil)

// reducedSet is an implementation of the slog.Handler interface which is
// backed by multiple slog.Handlers. This is used by HandlerSet's WithGroup
// and WithAttrs methods which produce slog.Handlers rather than
// btclog.Handlers.
type reducedSet struct {
	set []slog.Handler
}

// Enabled reports whether the handler handles records at the given level.
//
// NOTE: this is part of the slog.Handler interface.
func (r *reducedSet) Enabled(ctx context.Context,
	level slog.Level) bool {

	for _, handler := range r.set {
		if !handler.Enabled(ctx, level) {
			return false
		}
	}

	return true
}

// Handle handles the Record by dispatching to all underlying handlers.
//
// NOTE: this is part of the slog.Handler interface.
func (r *reducedSet) Handle(ctx context.Context,
	record slog.Record) error {

	for _, handler := range r.set {
		if err := handler.Handle(ctx, record); err != nil {
			return err
		}
	}

	return nil
}

// WithAttrs returns a new Handler whose attributes consist of both the
// receiver's attributes and the arguments.
//
// NOTE: this is part of the slog.Handler interface.
func (r *reducedSet) WithAttrs(attrs []slog.Attr) slog.Handler {
	newSet := &reducedSet{
		set: make([]slog.Handler, len(r.set)),
	}
	for i, handler := range r.set {
		newSet.set[i] = handler.WithAttrs(attrs)
	}

	return newSet
}

// WithGroup returns a new Handler with the given group appended to the
// receiver's existing groups.
//
// NOTE: this is part of the slog.Handler interface.
func (r *reducedSet) WithGroup(name string) slog.Handler {
	newSet := &reducedSet{
		set: make([]slog.Handler, len(r.set)),
	}
	for i, handler := range r.set {
		newSet.set[i] = handler.WithGroup(name)
	}

	return newSet
}

// Ensure reducedSet implements slog.Handler at compile time.
var _ slog.Handler = (*reducedSet)(nil)
