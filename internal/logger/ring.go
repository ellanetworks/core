// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package logger

import (
	"strconv"
	"sync"

	"go.uber.org/zap/zapcore"
)

const (
	maxRecentLogLines      = 100_000
	maxRecentLogEntryBytes = 64 * 1024
	maxRecentLogTotalBytes = 16 * 1024 * 1024
)

type logRing struct {
	mu      sync.Mutex
	entries [][]byte
	head    int
	count   int
	total   int
}

func newLogRing(capacity int) *logRing {
	if capacity < 1 {
		capacity = 1
	}

	return &logRing{entries: make([][]byte, capacity)}
}

func copyLogLine(p []byte) []byte {
	if len(p) > maxRecentLogEntryBytes {
		return []byte(`{"msg":"oversized log entry omitted","reason":"entry_size_limit","original_bytes":` + strconv.Itoa(len(p)) + "}\n")
	}

	line := make([]byte, len(p))
	copy(line, p)

	return line
}

func (r *logRing) Write(p []byte) (int, error) {
	if len(r.entries) == 0 {
		return len(p), nil
	}

	line := copyLogLine(p)

	r.mu.Lock()
	defer r.mu.Unlock()

	pos := (r.head + r.count) % len(r.entries)
	r.total -= len(r.entries[pos])
	r.entries[pos] = line
	r.total += len(line)

	if r.count < len(r.entries) {
		r.count++
	} else {
		r.head = (r.head + 1) % len(r.entries)
	}

	for r.total > maxRecentLogTotalBytes && r.count > 0 {
		r.total -= len(r.entries[r.head])
		r.entries[r.head] = nil
		r.head = (r.head + 1) % len(r.entries)
		r.count--
	}

	return len(p), nil
}

func (r *logRing) Sync() error {
	return nil
}

func (r *logRing) snapshot() []byte {
	r.mu.Lock()

	refs := make([][]byte, r.count)
	end := r.head + r.count

	if end <= len(r.entries) {
		copy(refs, r.entries[r.head:end])
	} else {
		first := len(r.entries) - r.head
		copy(refs[:first], r.entries[r.head:])
		copy(refs[first:], r.entries[:end-len(r.entries)])
	}

	r.mu.Unlock()

	var size int

	for _, e := range refs {
		size += len(e)
	}

	out := make([]byte, 0, size)

	for _, e := range refs {
		out = append(out, e...)
	}

	return out
}

var recentLogRing = newLogRing(maxRecentLogLines)

func RecentLogs() []byte {
	return recentLogRing.snapshot()
}

func recentLogCore(level zapcore.LevelEnabler) zapcore.Core {
	enc := zapcore.NewJSONEncoder(jsonEncoderConfig())

	return zapcore.NewCore(enc, recentLogRing, level)
}

var _ zapcore.WriteSyncer = (*logRing)(nil)
