// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"encoding/binary"
	"hash/maphash"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	duplicateWindow         = 4 * time.Minute
	duplicateEntryOverhead  = 256
	duplicateReportInterval = time.Minute
)

type duplicateKey struct {
	originHost string
	endToEndID uint32
	digest     uint64
}

type duplicateEntry struct {
	key     duplicateKey
	done    chan struct{}
	answer  []byte
	size    int
	expires time.Time
	next    *duplicateEntry
}

type duplicateCache struct {
	maxBytes int
	seed     maphash.Seed
	now      func() time.Time
	logger   *slog.Logger

	mu       sync.Mutex
	entries  map[duplicateKey]*duplicateEntry
	head     *duplicateEntry
	tail     *duplicateEntry
	bytes    int
	evicted  int
	reported time.Time
}

func newDuplicateCache(maxBytes int, logger *slog.Logger) *duplicateCache {
	return &duplicateCache{
		maxBytes: maxBytes,
		seed:     maphash.MakeSeed(),
		now:      time.Now,
		logger:   logger,
		entries:  make(map[duplicateKey]*duplicateEntry),
	}
}

func (d *duplicateCache) begin(req *Message) (*duplicateEntry, bool) {
	key, ok := d.key(req)
	if !ok {
		return nil, true
	}

	d.mu.Lock()

	now := d.now()
	d.expire(now)

	if e, ok := d.entries[key]; ok {
		d.mu.Unlock()
		return e, false
	}

	e := &duplicateEntry{
		key:     key,
		done:    make(chan struct{}),
		size:    duplicateEntryOverhead + len(key.originHost),
		expires: now.Add(duplicateWindow),
	}
	d.push(e)
	evicted := d.trim(now)

	d.mu.Unlock()
	d.report(evicted)

	return e, true
}

func (d *duplicateCache) key(req *Message) (duplicateKey, bool) {
	host, ok := req.Find(AVPOriginHost, 0)
	if !ok {
		return duplicateKey{}, false
	}

	var h maphash.Hash

	h.SetSeed(d.seed)

	var hdr [9]byte

	hdr[0] = req.Flags &^ FlagRetransmit
	binary.BigEndian.PutUint32(hdr[1:5], req.CommandCode)
	binary.BigEndian.PutUint32(hdr[5:9], req.ApplicationID)
	_, _ = h.Write(hdr[:])

	for _, a := range req.AVPs {
		if a.VendorID == 0 && (a.Code == AVPOriginHost || a.Code == AVPRouteRecord || a.Code == AVPProxyInfo) {
			continue
		}

		var avp [13]byte

		binary.BigEndian.PutUint32(avp[0:4], a.Code)
		avp[4] = a.Flags
		binary.BigEndian.PutUint32(avp[5:9], a.VendorID)
		binary.BigEndian.PutUint32(avp[9:13], uint32(len(a.Data)))
		_, _ = h.Write(avp[:])
		_, _ = h.Write(a.Data)
	}

	return duplicateKey{originHost: strings.ToLower(host.UTF8String()), endToEndID: req.EndToEndID, digest: h.Sum64()}, true
}

func (d *duplicateCache) finish(e *duplicateEntry, answer []byte) {
	if e == nil {
		return
	}

	d.mu.Lock()

	e.answer = answer
	close(e.done)

	evicted := 0

	if d.entries[e.key] == e {
		e.size += len(answer)
		d.bytes += len(answer)
		evicted = d.trim(d.now())
	}

	d.mu.Unlock()
	d.report(evicted)
}

func (d *duplicateCache) expire(now time.Time) {
	for d.head != nil && !now.Before(d.head.expires) {
		d.pop()
	}
}

func (d *duplicateCache) trim(now time.Time) int {
	d.expire(now)

	for d.bytes > d.maxBytes && d.head != nil {
		d.pop()
		d.evicted++
	}

	if d.evicted == 0 || now.Sub(d.reported) < duplicateReportInterval {
		return 0
	}

	n := d.evicted
	d.evicted, d.reported = 0, now

	return n
}

func (d *duplicateCache) report(evicted int) {
	if evicted > 0 {
		d.logger.Warn("Diameter duplicate detection cache is full; forgot requests before the End-to-End Identifier uniqueness period ended",
			slog.Int("evicted", evicted), slog.Int("max_bytes", d.maxBytes))
	}
}

func (d *duplicateCache) push(e *duplicateEntry) {
	d.entries[e.key] = e
	d.bytes += e.size

	if d.tail == nil {
		d.head = e
	} else {
		d.tail.next = e
	}

	d.tail = e
}

func (d *duplicateCache) pop() {
	e := d.head

	d.head = e.next
	if d.head == nil {
		d.tail = nil
	}

	e.next = nil
	delete(d.entries, e.key)
	d.bytes -= e.size
}

func (e *duplicateEntry) answerFor(req *Message) []byte {
	ans, err := Unmarshal(e.answer)
	if err != nil {
		return nil
	}

	ans.HopByHopID = req.HopByHopID
	ans.AVPs = slices.DeleteFunc(ans.AVPs, func(a AVP) bool { return a.Code == AVPProxyInfo && a.VendorID == 0 })
	ans.AVPs = append(ans.AVPs, FindAll(req.AVPs, AVPProxyInfo, 0)...)

	b, err := ans.Marshal()
	if err != nil {
		return nil
	}

	return b
}
