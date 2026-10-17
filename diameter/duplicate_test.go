// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.now
}

func (c *testClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.now = c.now.Add(d)
}

func testDuplicateCache(maxEntries int) (*duplicateCache, *testClock) {
	clock := &testClock{now: time.Unix(1_000_000, 0)}
	d := newDuplicateCache(DuplicateDetection{Window: DefaultDuplicateWindow, MaxEntries: maxEntries})
	d.now = clock.Now

	return d, clock
}

func encodedAnswer(t testing.TB, req *Message) []byte {
	t.Helper()

	b, err := NewAnswer(req, Identity{OriginHost: "ella.example.org", OriginRealm: "example.org"}, ResultSuccess).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func serveOnce(t testing.TB, d *duplicateCache, req *Message) bool {
	t.Helper()

	e, original := d.begin("id:mme", req)
	if original {
		d.finish(e, encodedAnswer(t, req))
	}

	return original
}

func TestDuplicateCacheWindow(t *testing.T) {
	d, clock := testDuplicateCache(16)

	if !serveOnce(t, d, appRequest(1, 7, "mme.example.org")) {
		t.Fatal("first request treated as a duplicate")
	}

	clock.advance(DefaultDuplicateWindow - time.Nanosecond)

	e, original := d.begin("id:mme", appRequest(2, 7, "MME.example.org"))
	if original {
		t.Fatal("retransmission inside the window was not detected")
	}

	b := e.answerFor(2)
	if binary.BigEndian.Uint32(b[12:16]) != 2 || binary.BigEndian.Uint32(b[16:20]) != 7 {
		t.Fatalf("cached answer has hop-by-hop %d end-to-end %d", binary.BigEndian.Uint32(b[12:16]), binary.BigEndian.Uint32(b[16:20]))
	}

	clock.advance(time.Nanosecond)

	if !serveOnce(t, d, appRequest(3, 7, "mme.example.org")) {
		t.Fatal("request after the window was treated as a duplicate")
	}

	if len(d.entries) != 1 || d.completed.len() != 1 {
		t.Fatalf("%d entries and %d completed after expiry, want 1 and 1", len(d.entries), d.completed.len())
	}
}

func TestDuplicateCacheKey(t *testing.T) {
	d, _ := testDuplicateCache(16)
	serveOnce(t, d, appRequest(1, 7, "mme.example.org"))

	otherCommand := appRequest(2, 7, "mme.example.org")
	otherCommand.CommandCode++

	otherApp := appRequest(3, 7, "mme.example.org")
	otherApp.ApplicationID++

	tests := map[string]struct {
		peer string
		req  *Message
	}{
		"other End-to-End":  {"id:mme", appRequest(4, 8, "mme.example.org")},
		"other Origin-Host": {"id:mme", appRequest(5, 7, "mme2.example.org")},
		"other peer":        {"id:dra", appRequest(6, 7, "mme.example.org")},
		"other command":     {"id:mme", otherCommand},
		"other application": {"id:mme", otherApp},
	}

	for name, tc := range tests {
		if e, original := d.begin(tc.peer, tc.req); !original {
			t.Errorf("%s: treated as a duplicate", name)
		} else {
			d.finish(e, encodedAnswer(t, tc.req))
		}
	}
}

func TestDuplicateCacheIgnoresRetransmitFlag(t *testing.T) {
	d, _ := testDuplicateCache(16)

	retransmit := appRequest(1, 7, "mme.example.org")
	retransmit.Flags |= FlagRetransmit

	if !serveOnce(t, d, retransmit) {
		t.Fatal("T-flagged request seen first was treated as a duplicate")
	}

	if serveOnce(t, d, appRequest(2, 7, "mme.example.org")) {
		t.Fatal("original arriving after its T-flagged retransmission was not detected")
	}

	if serveOnce(t, d, retransmit) {
		t.Fatal("T-flagged retransmission was not detected")
	}
}

func TestDuplicateCacheEvictsOldestAtBound(t *testing.T) {
	d, clock := testDuplicateCache(3)

	for i := range uint32(5) {
		serveOnce(t, d, appRequest(i, i, "mme.example.org"))
		clock.advance(time.Second)
	}

	if len(d.entries) != 3 || d.completed.len() != 3 {
		t.Fatalf("%d entries and %d completed, want 3", len(d.entries), d.completed.len())
	}

	for i := range uint32(5) {
		_, ok := d.entries[duplicateKey{peer: "id:mme", originHost: "mme.example.org", endToEndID: i}]
		if ok != (i >= 2) {
			t.Errorf("End-to-End %d cached = %v", i, ok)
		}
	}
}

func TestDuplicateCacheInFlight(t *testing.T) {
	d, _ := testDuplicateCache(1)

	var originals []*duplicateEntry

	for i := range uint32(3) {
		e, original := d.begin("id:mme", appRequest(i, i, "mme.example.org"))
		if !original {
			t.Fatalf("request %d treated as a duplicate", i)
		}

		originals = append(originals, e)
	}

	for i := range uint32(3) {
		e, original := d.begin("id:mme", appRequest(10+i, i, "mme.example.org"))
		if original || e != originals[i] {
			t.Fatalf("duplicate of in-flight request %d not detected", i)
		}

		select {
		case <-e.done:
			t.Fatal("in-flight entry reported done")
		default:
		}
	}

	req := appRequest(0, 0, "mme.example.org")
	d.finish(originals[0], encodedAnswer(t, req))

	select {
	case <-originals[0].done:
	default:
		t.Fatal("finished entry not done")
	}

	if binary.BigEndian.Uint32(originals[0].answerFor(42)[12:16]) != 42 {
		t.Fatal("waiting duplicate did not get the original's answer")
	}

	d.finish(originals[1], encodedAnswer(t, req))
	d.finish(originals[2], encodedAnswer(t, req))

	if len(d.entries) != 1 || d.completed.len() != 1 {
		t.Fatalf("%d entries and %d completed, want 1", len(d.entries), d.completed.len())
	}
}

func TestDuplicateCacheSlowRequest(t *testing.T) {
	d, clock := testDuplicateCache(16)

	req := appRequest(1, 7, "mme.example.org")
	e, _ := d.begin("id:mme", req)

	clock.advance(DefaultDuplicateWindow)

	if !serveOnce(t, d, appRequest(2, 7, "mme.example.org")) {
		t.Fatal("request after the window matched an in-flight original")
	}

	d.finish(e, encodedAnswer(t, req))

	select {
	case <-e.done:
	default:
		t.Fatal("finished entry not done")
	}

	if len(d.entries) != 1 || d.completed.len() != 1 {
		t.Fatalf("%d entries and %d completed, want only the newer request", len(d.entries), d.completed.len())
	}
}

func TestDuplicateCacheConcurrent(t *testing.T) {
	d, clock := testDuplicateCache(1 << 12)

	const (
		workers = 16
		keys    = 1 << 10
	)

	var (
		handled [keys]atomic.Int32
		wg      sync.WaitGroup
	)

	for w := range workers {
		wg.Go(func() {
			for i := range uint32(keys) {
				req := appRequest(uint32(w), i, "mme.example.org")

				e, original := d.begin("id:mme", req)
				if !original {
					<-e.done

					if e.answerFor(uint32(w)) == nil {
						t.Error("duplicate got no answer")
					}

					continue
				}

				handled[i].Add(1)
				d.finish(e, encodedAnswer(t, req))

				if i%64 == 0 {
					clock.advance(time.Millisecond)
				}
			}
		})
	}

	wg.Wait()

	for i := range handled {
		if n := handled[i].Load(); n != 1 {
			t.Errorf("End-to-End %d handled %d times", i, n)
		}
	}
}

func TestDuplicateQueue(t *testing.T) {
	var q duplicateQueue

	entries := make([]*duplicateEntry, 100)
	for i := range entries {
		entries[i] = &duplicateEntry{commandCode: uint32(i)}
	}

	next := 0

	for i, e := range entries {
		q.push(e)

		if i%3 == 2 {
			if got := q.pop(); got != entries[next] {
				t.Fatalf("pop = %d, want %d", got.commandCode, next)
			}

			next++
		}
	}

	for q.len() > 0 {
		if got := q.pop(); got != entries[next] {
			t.Fatalf("pop = %d, want %d", got.commandCode, next)
		}

		next++
	}

	if next != len(entries) {
		t.Fatalf("popped %d, want %d", next, len(entries))
	}
}

func BenchmarkDuplicateCache(b *testing.B) {
	for _, resident := range []int{1 << 14, 1 << 20} {
		d := newDuplicateCache(DuplicateDetection{Window: DefaultDuplicateWindow, MaxEntries: resident})
		answer := encodedAnswer(b, appRequest(0, 0, "mme.example.org"))

		for i := range uint32(resident) {
			e, _ := d.begin("id:mme", appRequest(i, i, "mme.example.org"))
			d.finish(e, answer)
		}

		var endToEnd atomic.Uint32
		endToEnd.Store(uint32(resident))

		b.Run(fmt.Sprintf("resident=%d", resident), func(b *testing.B) {
			req := appRequest(0, 0, "mme.example.org")

			for b.Loop() {
				req.EndToEndID = endToEnd.Add(1)
				e, _ := d.begin("id:mme", req)
				d.finish(e, answer)
			}
		})

		b.Run(fmt.Sprintf("resident=%d/parallel", resident), func(b *testing.B) {
			b.RunParallel(func(pb *testing.PB) {
				req := appRequest(0, 0, "mme.example.org")

				for pb.Next() {
					req.EndToEndID = endToEnd.Add(1)
					e, _ := d.begin("id:mme", req)
					d.finish(e, answer)
				}
			})
		})
	}
}
