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

func TestSinkCloseDropsLaterRecordsInsteadOfWritingToAClosedFile(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "system.log"))
	if err != nil {
		t.Fatalf("create log file: %v", err)
	}

	s := newSink()
	closeAll(s.swap(zapcore.NewCore(zapcore.NewJSONEncoder(jsonEncoderConfig()), zapcore.AddSync(f), zapcore.DebugLevel), []*os.File{f}))

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
