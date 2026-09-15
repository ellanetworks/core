// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package logger

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCloseFlushesAndClosesTheLogFile(t *testing.T) {
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

	files := trackFiles(nil)
	if len(files) != 0 {
		t.Errorf("Close left %d file(s) tracked", len(files))
	}
}

func TestReconfiguringClosesThePreviousLogFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "system.log")

	if err := ConfigureLogging("info", "file", path, "stdout", ""); err != nil {
		t.Fatalf("configure: %v", err)
	}

	t.Cleanup(func() { _ = ConfigureLogging("info", "stdout", "", "stdout", "") })

	first := trackFiles(nil)
	trackFiles(first)

	if len(first) != 1 {
		t.Fatalf("expected 1 tracked file, got %d", len(first))
	}

	if err := ConfigureLogging("info", "stdout", "", "stdout", ""); err != nil {
		t.Fatalf("reconfigure: %v", err)
	}

	if _, err := first[0].Write([]byte("x")); !errors.Is(err, os.ErrClosed) {
		t.Errorf("reconfiguring left the previous log file open, got %v", err)
	}
}
