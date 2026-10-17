// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"encoding/binary"
	"slices"
	"strings"
	"sync"
	"time"
)

type duplicateKey struct {
	peer       string
	originHost string
	endToEndID uint32
}

type duplicateEntry struct {
	key           duplicateKey
	done          chan struct{}
	commandCode   uint32
	applicationID uint32
	answer        []byte
	expires       time.Time
}

type duplicateCache struct {
	window time.Duration
	max    int
	now    func() time.Time

	mu        sync.Mutex
	entries   map[duplicateKey]*duplicateEntry
	completed duplicateQueue
}

func newDuplicateCache(cfg DuplicateDetection) *duplicateCache {
	return &duplicateCache{
		window:  cfg.Window,
		max:     cfg.MaxEntries,
		now:     time.Now,
		entries: make(map[duplicateKey]*duplicateEntry),
	}
}

func (d *duplicateCache) begin(peerKey string, req *Message) (*duplicateEntry, bool) {
	host, _ := req.Find(AVPOriginHost, 0)
	key := duplicateKey{peer: peerKey, originHost: strings.ToLower(host.UTF8String()), endToEndID: req.EndToEndID}
	now := d.now()

	d.mu.Lock()
	defer d.mu.Unlock()

	d.expire(now)

	if e, ok := d.entries[key]; ok && now.Before(e.expires) {
		if e.commandCode == req.CommandCode && e.applicationID == req.ApplicationID {
			return e, false
		}

		return nil, true
	}

	e := &duplicateEntry{
		key:           key,
		done:          make(chan struct{}),
		commandCode:   req.CommandCode,
		applicationID: req.ApplicationID,
		expires:       now.Add(d.window),
	}
	d.entries[key] = e

	return e, true
}

func (d *duplicateCache) finish(e *duplicateEntry, answer []byte) {
	if e == nil {
		return
	}

	now := d.now()

	d.mu.Lock()
	defer d.mu.Unlock()

	e.answer = answer
	close(e.done)

	if d.entries[e.key] != e {
		return
	}

	if !now.Before(e.expires) {
		delete(d.entries, e.key)
		return
	}

	d.expire(now)

	if d.completed.len() >= d.max {
		d.evict(d.completed.pop())
	}

	d.completed.push(e)
}

func (d *duplicateCache) expire(now time.Time) {
	for d.completed.len() > 0 && !now.Before(d.completed.front().expires) {
		d.evict(d.completed.pop())
	}
}

func (d *duplicateCache) evict(e *duplicateEntry) {
	if d.entries[e.key] == e {
		delete(d.entries, e.key)
	}
}

func (e *duplicateEntry) answerFor(hopByHop uint32) []byte {
	if len(e.answer) < headerLen {
		return nil
	}

	b := slices.Clone(e.answer)
	binary.BigEndian.PutUint32(b[12:16], hopByHop)

	return b
}

type duplicateQueue struct {
	buf  []*duplicateEntry
	head int
	n    int
}

func (q *duplicateQueue) len() int {
	return q.n
}

func (q *duplicateQueue) front() *duplicateEntry {
	return q.buf[q.head]
}

func (q *duplicateQueue) push(e *duplicateEntry) {
	if q.n == len(q.buf) {
		buf := make([]*duplicateEntry, max(16, 2*len(q.buf)))
		k := copy(buf, q.buf[q.head:])
		copy(buf[k:], q.buf[:q.head])
		q.buf, q.head = buf, 0
	}

	q.buf[(q.head+q.n)%len(q.buf)] = e
	q.n++
}

func (q *duplicateQueue) pop() *duplicateEntry {
	e := q.buf[q.head]
	q.buf[q.head] = nil
	q.head = (q.head + 1) % len(q.buf)
	q.n--

	return e
}
