// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package logger

import (
	"sync/atomic"

	"go.uber.org/zap/zapcore"
)

// dedupeCore drops a field whose key is named again later on the same record.
// The JSON encoder does not deduplicate and Loki's json parser keeps only the
// last occurrence, so a repeated key silently loses data. Later wins, which
// orders precedence as the ambient context, then the logger the call site
// named, then the fields on the log statement itself.
type dedupeCore struct {
	base    zapcore.Core
	fields  []zapcore.Field
	derived atomic.Pointer[zapcore.Core]
}

func newDedupeCore(base zapcore.Core) zapcore.Core {
	return &dedupeCore{base: base}
}

// resolve builds base.With(fields) on first use. Deriving it eagerly in With
// would clone the encoder and re-encode every field for loggers that never
// emit, which is the common case for a Debug statement on the signalling path.
func (c *dedupeCore) resolve() zapcore.Core {
	if d := c.derived.Load(); d != nil {
		return *d
	}

	core := c.base
	if len(c.fields) > 0 {
		core = c.base.With(c.fields)
	}

	c.derived.Store(&core)

	return core
}

// Enabled asks the undecorated base: a level does not depend on the accumulated
// fields, so answering it must not derive the core.
func (c *dedupeCore) Enabled(l zapcore.Level) bool { return c.base.Enabled(l) }

func (c *dedupeCore) With(fields []zapcore.Field) zapcore.Core {
	merged := make([]zapcore.Field, 0, len(c.fields)+len(fields))
	merged = append(merged, c.fields...)
	merged = append(merged, fields...)

	return &dedupeCore{base: c.base, fields: dedupe(merged)}
}

func (c *dedupeCore) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(e.Level) {
		return ce.AddCore(e, c)
	}

	return ce
}

func (c *dedupeCore) Write(e zapcore.Entry, fields []zapcore.Field) error {
	if !c.shadows(fields) {
		return c.resolve().Write(e, fields)
	}

	merged := make([]zapcore.Field, 0, len(c.fields)+len(fields))
	merged = append(merged, c.fields...)
	merged = append(merged, fields...)

	return c.base.Write(e, dedupe(merged))
}

func (c *dedupeCore) Sync() error { return c.resolve().Sync() }

// shadows reports whether an entry field repeats a key already accumulated on
// this core. The accumulated fields are pre-encoded into the derived core, so a
// record that repeats one has to be written through the undecorated base.
func (c *dedupeCore) shadows(fields []zapcore.Field) bool {
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
