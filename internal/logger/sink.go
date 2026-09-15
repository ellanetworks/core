// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package logger

import (
	"os"
	"sync"
	"sync/atomic"

	"go.uber.org/zap/zapcore"
)

type sinkState struct {
	gen  uint64
	core zapcore.Core
}

type sink struct {
	mu    sync.Mutex
	state atomic.Pointer[sinkState]
	files []*os.File
}

func newSink() *sink {
	s := &sink{}
	s.state.Store(&sinkState{core: zapcore.NewNopCore()})

	return s
}

func (s *sink) load() (zapcore.Core, uint64) {
	state := s.state.Load()

	return state.core, state.gen
}

func (s *sink) swap(core zapcore.Core, files []*os.File) []*os.File {
	s.mu.Lock()
	defer s.mu.Unlock()

	old := s.files
	s.files = files
	s.state.Store(&sinkState{gen: s.state.Load().gen + 1, core: core})

	return old
}

func (s *sink) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	prev := s.state.Load()

	_ = prev.core.Sync()

	s.state.Store(&sinkState{gen: prev.gen + 1, core: zapcore.NewNopCore()})

	var err error

	for _, f := range s.files {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}

	s.files = nil

	return err
}

type derivedCore struct {
	gen  uint64
	core zapcore.Core
}

type followingCore struct {
	sink   *sink
	fields []zapcore.Field
	cache  atomic.Pointer[derivedCore]
}

func (c *followingCore) resolve() zapcore.Core {
	base, gen := c.sink.load()
	if d := c.cache.Load(); d != nil && d.gen == gen {
		return d.core
	}

	core := base
	if len(c.fields) > 0 {
		core = base.With(c.fields)
	}

	c.cache.Store(&derivedCore{gen: gen, core: core})

	return core
}

func (c *followingCore) Enabled(l zapcore.Level) bool { return c.resolve().Enabled(l) }

func (c *followingCore) With(fields []zapcore.Field) zapcore.Core {
	merged := make([]zapcore.Field, 0, len(c.fields)+len(fields))
	merged = append(merged, c.fields...)
	merged = append(merged, fields...)

	return &followingCore{sink: c.sink, fields: dedupe(merged)}
}

func (c *followingCore) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(e.Level) {
		return ce.AddCore(e, c)
	}

	return ce
}

func (c *followingCore) Write(e zapcore.Entry, fields []zapcore.Field) error {
	if !c.shadows(fields) {
		return c.resolve().Write(e, fields)
	}

	merged := make([]zapcore.Field, 0, len(c.fields)+len(fields))
	merged = append(merged, c.fields...)
	merged = append(merged, fields...)

	base, _ := c.sink.load()

	return base.Write(e, dedupe(merged))
}

// shadows reports whether an entry field repeats a key already accumulated on
// this core. The accumulated fields are pre-encoded into the derived core, so a
// record that repeats one has to be written through the bare sink instead.
func (c *followingCore) shadows(fields []zapcore.Field) bool {
	for _, f := range fields {
		if !keyed(f) {
			continue
		}

		for _, acc := range c.fields {
			if keyed(acc) && acc.Key == f.Key {
				return true
			}
		}
	}

	return false
}

func keyed(f zapcore.Field) bool {
	return f.Type != zapcore.SkipType && f.Key != ""
}

// dedupe drops every field whose key reappears later in the slice, so a record
// never carries the same key twice: the JSON encoder does not deduplicate, and
// Loki's json parser keeps only the last occurrence. The later field wins,
// which orders precedence as ambient context, then the logger the call site
// named, then the fields on the log statement itself.
func dedupe(fields []zapcore.Field) []zapcore.Field {
	if !hasDuplicate(fields) {
		return fields
	}

	out := make([]zapcore.Field, 0, len(fields))

	for i, f := range fields {
		if keyed(f) && lastIndexOfKey(fields, f.Key) != i {
			continue
		}

		out = append(out, f)
	}

	return out
}

func hasDuplicate(fields []zapcore.Field) bool {
	for i, f := range fields {
		if keyed(f) && lastIndexOfKey(fields, f.Key) != i {
			return true
		}
	}

	return false
}

func lastIndexOfKey(fields []zapcore.Field, key string) int {
	for i := len(fields) - 1; i >= 0; i-- {
		if keyed(fields[i]) && fields[i].Key == key {
			return i
		}
	}

	return -1
}

func (c *followingCore) Sync() error { return c.resolve().Sync() }
