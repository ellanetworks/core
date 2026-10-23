// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package logger

import (
	"bytes"
	"testing"
)

func fillRing(r *logRing, line []byte, n int) {
	for range n {
		_, _ = r.Write(line)
	}
}

func BenchmarkLogRingWriteFull(b *testing.B) {
	r := newLogRing(maxRecentLogLines)
	line := bytes.Repeat([]byte{'x'}, 100)
	line[len(line)-1] = '\n'

	fillRing(r, line, maxRecentLogLines)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, _ = r.Write(line)
	}
}

func BenchmarkLogRingWriteOversized(b *testing.B) {
	r := newLogRing(1024)
	line := bytes.Repeat([]byte{'y'}, 1024*1024)
	line[len(line)-1] = '\n'

	fillRing(r, line, 512)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, _ = r.Write(line)
	}
}

func BenchmarkLogRingSnapshotFull(b *testing.B) {
	r := newLogRing(maxRecentLogLines)
	line := bytes.Repeat([]byte{'z'}, 100)
	line[len(line)-1] = '\n'

	fillRing(r, line, maxRecentLogLines)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = r.snapshot()
	}
}

func BenchmarkLogRingSnapshotOversized(b *testing.B) {
	r := newLogRing(1024)
	line := bytes.Repeat([]byte{'w'}, 1024*1024)
	line[len(line)-1] = '\n'

	fillRing(r, line, 512)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = r.snapshot()
	}
}
