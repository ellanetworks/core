package logger

import (
	"os"
	"sync"
	"sync/atomic"

	"go.uber.org/zap/zapcore"
)

type sink struct {
	mu    sync.RWMutex
	gen   uint64
	core  zapcore.Core
	files []*os.File
}

func newSink() *sink {
	return &sink{core: zapcore.NewNopCore()}
}

func (s *sink) load() (zapcore.Core, uint64) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.core, s.gen
}

func (s *sink) swap(core zapcore.Core, files []*os.File) []*os.File {
	s.mu.Lock()
	defer s.mu.Unlock()

	old := s.files
	s.core, s.files = core, files
	s.gen++

	return old
}

func (s *sink) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_ = s.core.Sync()

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

	return &followingCore{sink: c.sink, fields: merged}
}

func (c *followingCore) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(e.Level) {
		return ce.AddCore(e, c)
	}

	return ce
}

func (c *followingCore) Write(e zapcore.Entry, fields []zapcore.Field) error {
	return c.resolve().Write(e, fields)
}

func (c *followingCore) Sync() error { return c.resolve().Sync() }
