// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"errors"
	"net"
	"os"
	"syscall"
	"testing"

	"github.com/hashicorp/go-hclog"
	hraft "github.com/hashicorp/raft"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type causeOnlyError struct{ cause error }

func (e causeOnlyError) Error() string { return "msgpack decode error [pos 0]: " + e.cause.Error() }
func (e causeOnlyError) Cause() error  { return e.cause }

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestZapRaftLoggerErrorLevels(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want zapcore.Level
	}{
		{"transport shutdown", hraft.ErrTransportShutdown, zapcore.DebugLevel},
		{"connection refused", &net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}, zapcore.WarnLevel},
		{"timeout behind Cause", causeOnlyError{cause: &net.OpError{Op: "read", Err: timeoutError{}}}, zapcore.WarnLevel},
		{"other error", errors.New("boom"), zapcore.ErrorLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zapcore.DebugLevel)
			l := &zapRaftLogger{zap: zap.New(core), name: "raft"}

			l.Log(hclog.Error, "msg", "error", tt.err)

			entries := logs.All()
			if len(entries) != 1 {
				t.Fatalf("expected 1 entry, got %d", len(entries))
			}

			if entries[0].Level != tt.want {
				t.Fatalf("level = %s, want %s", entries[0].Level, tt.want)
			}
		})
	}
}
