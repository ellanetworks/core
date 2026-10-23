// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package logger

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestSwapSystemCoreRespectsReplacementLevel(t *testing.T) {
	for _, threshold := range []zapcore.Level{zapcore.WarnLevel, zapcore.DebugLevel} {
		t.Run(threshold.String(), func(t *testing.T) {
			previousLevel := atomicLevel.Level()
			atomicLevel.SetLevel(zapcore.InfoLevel)
			t.Cleanup(func() { atomicLevel.SetLevel(previousLevel) })

			previousRing := recentLogRing
			recentLogRing = newLogRing(16)

			t.Cleanup(func() { recentLogRing = previousRing })

			var output bytes.Buffer

			core := zapcore.NewCore(zapcore.NewJSONEncoder(jsonEncoderConfig()), zapcore.AddSync(&output), threshold)
			t.Cleanup(SwapSystemCore(core))

			for _, level := range []zapcore.Level{zapcore.DebugLevel, zapcore.InfoLevel, zapcore.WarnLevel} {
				message := threshold.String() + " replacement " + level.String()
				want := threshold.Enabled(level)

				if got := EllaLog.Core().Enabled(level); got != want {
					t.Errorf("replacement Enabled(%s) = %v, want %v", level, got, want)
				}

				EllaLog.Log(level, message)

				if got := strings.Contains(output.String(), message); got != want {
					t.Errorf("replacement output contains %q = %v, want %v: %s", message, got, want, output.String())
				}

				if got := bytes.Contains(RecentLogs(), []byte(message)); got != want {
					t.Errorf("recent logs contain %q = %v, want %v: %s", message, got, want, RecentLogs())
				}
			}
		})
	}
}

func TestConfiguredLoggingFollowsLevelChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "system.log")
	t.Cleanup(func() { _ = ConfigureLogging("info", "stdout", "", "stdout", "") })

	for _, step := range []struct {
		name      string
		threshold zapcore.Level
		configure bool
	}{
		{"configure warn", zapcore.WarnLevel, true},
		{"set debug", zapcore.DebugLevel, false},
		{"set error", zapcore.ErrorLevel, false},
		{"reconfigure info", zapcore.InfoLevel, true},
	} {
		t.Run(step.name, func(t *testing.T) {
			if step.configure {
				if err := ConfigureLogging(step.threshold.String(), "file", path, "stdout", ""); err != nil {
					t.Fatalf("configure: %v", err)
				}
			} else if err := SetLevel(step.threshold.String()); err != nil {
				t.Fatalf("set level: %v", err)
			}

			for _, level := range []zapcore.Level{zapcore.DebugLevel, zapcore.InfoLevel, zapcore.WarnLevel, zapcore.ErrorLevel} {
				message := step.name + " message " + level.String()
				want := step.threshold.Enabled(level)

				EllaLog.Log(level, message)

				output, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read log file: %v", err)
				}

				if got := bytes.Contains(output, []byte(message)); got != want {
					t.Errorf("configured output contains %q = %v, want %v: %s", message, got, want, output)
				}

				if got := bytes.Contains(RecentLogs(), []byte(message)); got != want {
					t.Errorf("recent logs contain %q = %v, want %v: %s", message, got, want, RecentLogs())
				}
			}
		})
	}
}

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
