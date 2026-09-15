// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"fmt"
	"io"
	"log"

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
		zap:  logger.RaftLog,
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
	return &zapRaftLogger{zap: logger.RaftLog.Named(name), name: name}
}

func (l *zapRaftLogger) SetLevel(hclog.Level)                                    {}
func (l *zapRaftLogger) GetLevel() hclog.Level                                   { return hclog.Debug }
func (l *zapRaftLogger) StandardLogger(*hclog.StandardLoggerOptions) *log.Logger { return nil }
func (l *zapRaftLogger) StandardWriter(*hclog.StandardLoggerOptions) io.Writer   { return io.Discard }

// newZapRaftSubLogger returns the raft logger tagged with a subsystem
// field. hashicorp/raft's file snapshot store and TCP transport accept an
// hclog.Logger directly, so their output reaches zap with the level and
// key/value pairs intact instead of being rendered to text first.
func newZapRaftSubLogger(subsystem string) hclog.Logger {
	return newZapRaftLogger().With("subsystem", subsystem)
}

func argsToFields(args []interface{}) []zap.Field {
	fields := make([]zap.Field, 0, len(args)/2)

	for i := 0; i+1 < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			continue
		}

		if f, ok := args[i+1].(hclog.Format); ok {
			fields = append(fields, zap.String(key, formatValue(f)))
			continue
		}

		fields = append(fields, zap.Any(key, args[i+1]))
	}

	return fields
}

// formatValue renders an hclog.Format, whose first element is a Printf
// format string for the remaining elements. Without this the whole slice
// is marshalled verbatim and the format verbs end up in the log output.
func formatValue(f hclog.Format) string {
	if len(f) == 0 {
		return ""
	}

	format, ok := f[0].(string)
	if !ok {
		return fmt.Sprintf("%v", []interface{}(f))
	}

	return fmt.Sprintf(format, f[1:]...)
}
