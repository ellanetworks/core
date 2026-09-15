// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package logger

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func fileSink(t *testing.T) (*sink, *os.File) {
	t.Helper()

	f, err := os.Create(filepath.Join(t.TempDir(), "system.log"))
	if err != nil {
		t.Fatalf("create log file: %v", err)
	}

	s := newSink()

	core := zapcore.NewCore(zapcore.NewJSONEncoder(jsonEncoderConfig()), zapcore.AddSync(f), zapcore.DebugLevel)
	closeAll(s.swap(core, []*os.File{f}))

	return s, f
}

func TestSinkCloseRetiresTheCoreBeforeClosingItsFiles(t *testing.T) {
	s, f := fileSink(t)

	log := zap.New(&followingCore{sink: s})
	log.Info("before shutdown")

	if err := s.close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	log.Info("after shutdown")

	if err := log.Sync(); err != nil {
		t.Errorf("logging after Close reached a closed file: %v", err)
	}

	if _, err := f.Write([]byte("x")); !errors.Is(err, os.ErrClosed) {
		t.Errorf("close did not close the sink's file, got %v", err)
	}
}

func TestSinkCloseInvalidatesDerivedCores(t *testing.T) {
	s, _ := fileSink(t)

	derived := (&followingCore{sink: s}).With([]zapcore.Field{zap.String("k", "v")})

	if !derived.Enabled(zapcore.InfoLevel) {
		t.Fatal("a configured sink reported its derived core disabled")
	}

	if err := s.close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if derived.Enabled(zapcore.InfoLevel) {
		t.Error("a derived core kept following the sink across Close")
	}
}
