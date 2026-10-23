// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"go.uber.org/zap"
)

func TestLogRingKeepsOrderBeforeFull(t *testing.T) {
	r := newLogRing(3)

	for _, line := range []string{"one\n", "two\n"} {
		if _, err := r.Write([]byte(line)); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	if got := string(r.snapshot()); got != "one\ntwo\n" {
		t.Fatalf("unexpected snapshot: %q", got)
	}
}

func TestLogRingEvictsOldestWhenFull(t *testing.T) {
	r := newLogRing(3)

	for _, line := range []string{"one\n", "two\n", "three\n", "four\n"} {
		if _, err := r.Write([]byte(line)); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	if got := string(r.snapshot()); got != "two\nthree\nfour\n" {
		t.Fatalf("unexpected snapshot: %q", got)
	}
}

func TestLogRingWritesCopyNotAlias(t *testing.T) {
	r := newLogRing(2)

	payload := []byte("original\n")
	if _, err := r.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}

	payload[0] = 'X'

	if got := string(r.snapshot()); got != "original\n" {
		t.Fatalf("snapshot aliased the caller buffer: %q", got)
	}
}

func TestLogRingConcurrentWrites(t *testing.T) {
	r := newLogRing(64)

	var wg sync.WaitGroup

	for i := 0; i < 128; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			_, _ = r.Write([]byte("x\n"))
		}()
	}

	wg.Wait()

	if got := string(r.snapshot()); len(got) != 64*2 {
		t.Fatalf("expected %d bytes under race, got %d", 64*2, len(got))
	}
}

func TestLogRingTruncatesOversizedEntries(t *testing.T) {
	r := newLogRing(2)

	big := bytes.Repeat([]byte{'a'}, maxRecentLogEntryBytes+4096)
	if _, err := r.Write(big); err != nil {
		t.Fatalf("write: %v", err)
	}

	snapshot := r.snapshot()
	if !json.Valid(bytes.TrimSpace(snapshot)) || len(snapshot) >= maxRecentLogEntryBytes {
		t.Fatalf("expected a bounded JSON omission marker, got %q", snapshot)
	}

	if snapshot[len(snapshot)-1] != '\n' {
		t.Fatalf("expected truncated entry to keep its line terminator")
	}

	if _, err := r.Write([]byte("next\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	if !bytes.HasSuffix(r.snapshot(), []byte("next\n")) {
		t.Fatalf("entry after a truncated one was dropped: %q", r.snapshot())
	}
}

func TestOversizedNestedLogIsOmitted(t *testing.T) {
	r := newLogRing(2)
	inner := `{"tmsi":"a1b2c3d4"}`

	line, err := json.Marshal(map[string]string{"event": inner, "padding": strings.Repeat("x", maxRecentLogEntryBytes)})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := r.Write(line); err != nil {
		t.Fatalf("write: %v", err)
	}

	var marker struct {
		Msg           string `json:"msg"`
		Reason        string `json:"reason"`
		OriginalBytes int    `json:"original_bytes"`
	}

	if err := json.Unmarshal(r.snapshot(), &marker); err != nil {
		t.Fatalf("unmarshal omission marker: %v", err)
	}

	if marker.Msg != "oversized log entry omitted" || marker.Reason != "entry_size_limit" || marker.OriginalBytes != len(line) {
		t.Fatalf("unexpected omission marker: %#v", marker)
	}
}

func TestLogRingEnforcesTotalByteBudget(t *testing.T) {
	capacity := 300
	r := newLogRing(capacity)

	for i := range capacity {
		payload := bytes.Repeat([]byte{'.'}, maxRecentLogEntryBytes)
		payload[0] = byte('0' + i/100)
		payload[1] = byte('0' + i/10%10)
		payload[2] = byte('0' + i%10)
		payload[maxRecentLogEntryBytes-1] = '\n'

		if _, err := r.Write(payload); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	snapshot := r.snapshot()

	if len(snapshot) > maxRecentLogTotalBytes {
		t.Fatalf("snapshot of %d bytes exceeds budget %d", len(snapshot), maxRecentLogTotalBytes)
	}

	if len(snapshot) < maxRecentLogTotalBytes-maxRecentLogEntryBytes {
		t.Fatalf("snapshot of %d bytes left budget %d underused", len(snapshot), maxRecentLogTotalBytes)
	}

	first := capacity - len(snapshot)/maxRecentLogEntryBytes
	expected := []byte{byte('0' + first/100), byte('0' + first/10%10), byte('0' + first%10)}

	if !bytes.HasPrefix(snapshot, expected) {
		t.Fatalf("expected oldest retained entry %q, got %q", expected, snapshot[:3])
	}
}

func TestRecentLogsCapturesSystemLogsNotAudit(t *testing.T) {
	if err := ConfigureLogging("info", "stdout", "", "stdout", ""); err != nil {
		t.Fatalf("configure: %v", err)
	}

	t.Cleanup(func() { _ = ConfigureLogging("info", "stdout", "", "stdout", "") })

	EllaLog.Info("recent logs system marker")
	AuditLog.Info("recent logs audit marker")

	snapshot := string(RecentLogs())

	if !strings.Contains(snapshot, "recent logs system marker") {
		t.Fatalf("RecentLogs does not contain system log line: %q", snapshot)
	}

	if strings.Contains(snapshot, "recent logs audit marker") {
		t.Fatalf("RecentLogs contains audit log line: %q", snapshot)
	}
}

func TestRecentLogsPreservesIdentitiesAndMessages(t *testing.T) {
	if err := ConfigureLogging("info", "stdout", "", "stdout", ""); err != nil {
		t.Fatalf("configure: %v", err)
	}

	t.Cleanup(func() { _ = ConfigureLogging("info", "stdout", "", "stdout", "") })

	EllaLog.Info("registration", SUPI("imsi-001010000000001"), TMSI("a1b2c3d4"))
	EllaLog.Warn("[NAS] UE Context released, abort retransmission of Registration Accept")
	EllaLog.Error("registration failed for imsi-00101123456789 and alice@example.com", SUPI("imsi-00101123456789"), zap.String("email", "alice@example.com"))

	snapshot := RecentLogs()

	if !bytes.Equal(snapshot, recentLogRing.snapshot()) {
		t.Fatalf("RecentLogs changed the raw ring snapshot: %q", snapshot)
	}

	expected := map[string]map[string]string{
		"registration": {
			"level": "info",
			"supi":  "imsi-001010000000001",
			"tmsi":  "a1b2c3d4",
		},
		"[NAS] UE Context released, abort retransmission of Registration Accept": {
			"level": "warn",
		},
		"registration failed for imsi-00101123456789 and alice@example.com": {
			"level": "error",
			"supi":  "imsi-00101123456789",
			"email": "alice@example.com",
		},
	}

	for _, line := range bytes.Split(bytes.TrimSpace(snapshot), []byte{'\n'}) {
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("unmarshal log entry: %v", err)
		}

		message, _ := entry["msg"].(string)

		for field, want := range expected[message] {
			if got := entry[field]; got != want {
				t.Fatalf("message %q: expected %s=%q, got %v", message, field, want, got)
			}
		}

		delete(expected, message)
	}

	if len(expected) != 0 {
		t.Fatalf("RecentLogs missing original messages: %v", expected)
	}
}

func TestRecentLogsSurvivesReconfiguration(t *testing.T) {
	if err := ConfigureLogging("info", "stdout", "", "stdout", ""); err != nil {
		t.Fatalf("configure: %v", err)
	}

	EllaLog.Info("before reconfigure marker")

	if err := ConfigureLogging("info", "stdout", "", "stdout", ""); err != nil {
		t.Fatalf("reconfigure: %v", err)
	}

	t.Cleanup(func() { _ = ConfigureLogging("info", "stdout", "", "stdout", "") })

	EllaLog.Info("after reconfigure marker")

	snapshot := string(RecentLogs())

	if !strings.Contains(snapshot, "before reconfigure marker") {
		t.Fatalf("RecentLogs lost history across reconfiguration: %q", snapshot)
	}

	if !strings.Contains(snapshot, "after reconfigure marker") {
		t.Fatalf("RecentLogs missing post-reconfiguration line: %q", snapshot)
	}
}
