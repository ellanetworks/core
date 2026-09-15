// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestCloseFlushesPendingRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "system.log")

	if err := ConfigureLogging("info", "file", path, "stdout", ""); err != nil {
		t.Fatalf("configure: %v", err)
	}

	t.Cleanup(func() { _ = ConfigureLogging("info", "stdout", "", "stdout", "") })

	MmeLog.Info("before shutdown")

	if err := Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}

	if !strings.Contains(string(body), "before shutdown") {
		t.Errorf("Close did not flush the record, file holds %q", body)
	}
}

func TestCloseDisablesFurtherLogging(t *testing.T) {
	path := filepath.Join(t.TempDir(), "system.log")

	if err := ConfigureLogging("info", "file", path, "stdout", ""); err != nil {
		t.Fatalf("configure: %v", err)
	}

	t.Cleanup(func() { _ = ConfigureLogging("info", "stdout", "", "stdout", "") })

	if !MmeLog.Core().Enabled(zapcore.ErrorLevel) {
		t.Fatal("errors are not logged before Close")
	}

	if err := Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if MmeLog.Core().Enabled(zapcore.ErrorLevel) {
		t.Error("logging is still enabled after Close, so a straggler would write to a closed file")
	}
}
