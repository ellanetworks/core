// Copyright 2026 Ella Networks

package raft

import (
	"bytes"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/hashicorp/go-hclog"
	"go.uber.org/zap"
)

// zapRaftLogger adapts the application's zap logger to hashicorp/raft's
// hclog.Logger interface.
type zapRaftLogger struct {
	zap  *zap.Logger
	name string
}

func newZapRaftLogger() hclog.Logger {
	return &zapRaftLogger{
		zap:  logger.DBLog,
		name: "raft",
	}
}

func (l *zapRaftLogger) Log(level hclog.Level, msg string, args ...interface{}) {
	fields := argsToFields(args)

	switch level {
	case hclog.Trace, hclog.Debug:
		l.zap.Debug(msg, fields...)
	case hclog.Info:
		l.zap.Info(msg, fields...)
	case hclog.Warn:
		l.zap.Warn(msg, fields...)
	case hclog.Error:
		l.zap.Error(msg, fields...)
	}
}

func (l *zapRaftLogger) Trace(msg string, args ...interface{}) { l.Log(hclog.Trace, msg, args...) }
func (l *zapRaftLogger) Debug(msg string, args ...interface{}) { l.Log(hclog.Debug, msg, args...) }
func (l *zapRaftLogger) Info(msg string, args ...interface{})  { l.Log(hclog.Info, msg, args...) }
func (l *zapRaftLogger) Warn(msg string, args ...interface{})  { l.Log(hclog.Warn, msg, args...) }
func (l *zapRaftLogger) Error(msg string, args ...interface{}) { l.Log(hclog.Error, msg, args...) }

func (l *zapRaftLogger) IsTrace() bool { return false }
func (l *zapRaftLogger) IsDebug() bool { return true }
func (l *zapRaftLogger) IsInfo() bool  { return true }
func (l *zapRaftLogger) IsWarn() bool  { return true }
func (l *zapRaftLogger) IsError() bool { return true }

func (l *zapRaftLogger) ImpliedArgs() []interface{} { return nil }

func (l *zapRaftLogger) With(args ...interface{}) hclog.Logger {
	return &zapRaftLogger{zap: l.zap.With(argsToFields(args)...), name: l.name}
}

func (l *zapRaftLogger) Name() string { return l.name }

func (l *zapRaftLogger) Named(name string) hclog.Logger {
	newName := l.name + "." + name
	return &zapRaftLogger{zap: l.zap.Named(name), name: newName}
}

func (l *zapRaftLogger) ResetNamed(name string) hclog.Logger {
	return &zapRaftLogger{zap: logger.DBLog.Named(name), name: name}
}

func (l *zapRaftLogger) SetLevel(hclog.Level)                                    {}
func (l *zapRaftLogger) GetLevel() hclog.Level                                   { return hclog.Debug }
func (l *zapRaftLogger) StandardLogger(*hclog.StandardLoggerOptions) *log.Logger { return nil }
func (l *zapRaftLogger) StandardWriter(*hclog.StandardLoggerOptions) io.Writer   { return io.Discard }

// zapIOWriter adapts an io.Writer to zap. hashicorp/raft's file snapshot
// store and TCP transport build a stdlib *log.Logger over the writer
// supplied at construction time; without this adapter their output bypasses
// the structured logger and goes to bare stderr. Lines are buffered until a
// newline and then forwarded to DBLog at Info level with a subsystem field,
// so operators can grep one stream instead of three.
type zapIOWriter struct {
	mu        sync.Mutex
	buf       []byte
	zap       *zap.Logger
	subsystem string
}

func newZapIOWriter(subsystem string) io.Writer {
	return &zapIOWriter{
		zap:       logger.DBLog,
		subsystem: subsystem,
	}
}

func (w *zapIOWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf = append(w.buf, p...)

	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}

		line := strings.TrimRight(string(w.buf[:i]), "\r")
		w.buf = w.buf[i+1:]

		if line == "" {
			continue
		}

		w.zap.Info(line, zap.String("subsystem", w.subsystem))
	}

	return len(p), nil
}

func argsToFields(args []interface{}) []zap.Field {
	fields := make([]zap.Field, 0, len(args)/2)

	for i := 0; i+1 < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			continue
		}

		fields = append(fields, zap.Any(key, args[i+1]))
	}

	return fields
}
