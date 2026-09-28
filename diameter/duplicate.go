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

const duplicateWindow = 4 * time.Minute

type duplicateKey struct {
	peer       string
	originHost string
	endToEndID uint32
}

type duplicateEntry struct {
	done          chan struct{}
	commandCode   uint32
	applicationID uint32
	answer        []byte
	expires       time.Time
}

type duplicateCache struct {
	mu        sync.Mutex
	max       int
	entries   map[duplicateKey]*duplicateEntry
	lastSweep time.Time
}

func (d *duplicateCache) begin(peerKey string, req *Message) (duplicateKey, *duplicateEntry, bool) {
	host, _ := req.Find(AVPOriginHost, 0)
	key := duplicateKey{peer: peerKey, originHost: strings.ToLower(host.UTF8String()), endToEndID: req.EndToEndID}
	now := time.Now()

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.entries == nil {
		d.entries = make(map[duplicateKey]*duplicateEntry)
	}

	if now.Sub(d.lastSweep) > duplicateWindow/4 || len(d.entries) >= d.max {
		d.sweep(now)
	}

	if e, ok := d.entries[key]; ok && now.Before(e.expires) {
		if e.commandCode == req.CommandCode && e.applicationID == req.ApplicationID {
			return key, e, false
		}

		return key, nil, true
	}

	if len(d.entries) >= d.max {
		return key, nil, true
	}

	e := &duplicateEntry{
		done:          make(chan struct{}),
		commandCode:   req.CommandCode,
		applicationID: req.ApplicationID,
		expires:       now.Add(duplicateWindow),
	}
	d.entries[key] = e

	return key, e, true
}

func (d *duplicateCache) finish(key duplicateKey, e *duplicateEntry, answer []byte) {
	if e == nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	e.answer = answer
	close(e.done)

	if d.entries[key] == e {
		e.expires = time.Now().Add(duplicateWindow)
	}
}

func (d *duplicateCache) sweep(now time.Time) {
	d.lastSweep = now

	for k, e := range d.entries {
		if now.After(e.expires) {
			select {
			case <-e.done:
				delete(d.entries, k)
			default:
			}
		}
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
