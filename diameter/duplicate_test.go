// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log/slog"
	"strings"
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

func testDuplicateCache(maxBytes int) (*duplicateCache, *testClock) {
	clock := &testClock{now: time.Unix(1_000_000, 0)}
	d := newDuplicateCache(maxBytes, slog.New(slog.DiscardHandler))
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

	e, original := d.begin(req)
	if original {
		d.finish(e, encodedAnswer(t, req))
	}

	return original
}

func checkDuplicateCache(t *testing.T, d *duplicateCache, want int) {
	t.Helper()

	d.mu.Lock()
	defer d.mu.Unlock()

	n, size := 0, 0

	for e := d.head; e != nil; e = e.next {
		if d.entries[e.key] != e {
			t.Errorf("queued entry %+v missing from the index", e.key)
		}

		if e.next != nil && e.next.expires.Before(e.expires) {
			t.Errorf("queue out of expiry order at %+v", e.key)
		}

		n++
		size += e.size
	}

	if n != len(d.entries) || n != want {
		t.Errorf("%d queued and %d indexed entries, want %d", n, len(d.entries), want)
	}

	if size != d.bytes {
		t.Errorf("accounted %d bytes, entries hold %d", d.bytes, size)
	}
}

func cached(d *duplicateCache, endToEnd uint32) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	key, _ := d.key(appRequest(0, endToEnd, "mme.example.org"))
	_, ok := d.entries[key]

	return ok
}

func TestDuplicateCacheWindow(t *testing.T) {
	d, clock := testDuplicateCache(DefaultDuplicateCacheBytes)

	if !serveOnce(t, d, appRequest(1, 7, "mme.example.org")) {
		t.Fatal("first request treated as a duplicate")
	}

	clock.advance(duplicateWindow - time.Nanosecond)

	retransmit := appRequest(2, 7, "mme.example.org")
	retransmit.AVPs[1] = UTF8String(AVPOriginHost, AVPFlagMandatory, 0, "MME.example.org")

	e, original := d.begin(retransmit)
	if original {
		t.Fatal("retransmission inside the window was not detected")
	}

	b := e.answerFor(retransmit)
	if binary.BigEndian.Uint32(b[12:16]) != 2 || binary.BigEndian.Uint32(b[16:20]) != 7 {
		t.Fatalf("cached answer has hop-by-hop %d end-to-end %d", binary.BigEndian.Uint32(b[12:16]), binary.BigEndian.Uint32(b[16:20]))
	}

	clock.advance(time.Nanosecond)

	if !serveOnce(t, d, appRequest(3, 7, "mme.example.org")) {
		t.Fatal("request after the window was treated as a duplicate")
	}

	checkDuplicateCache(t, d, 1)
}

func TestDuplicateCacheKey(t *testing.T) {
	d, _ := testDuplicateCache(DefaultDuplicateCacheBytes)
	serveOnce(t, d, appRequest(1, 7, "mme.example.org"))

	otherCommand := appRequest(2, 7, "mme.example.org")
	otherCommand.CommandCode++

	otherApp := appRequest(3, 7, "mme.example.org")
	otherApp.ApplicationID++

	tests := map[string]*Message{
		"other End-to-End":  appRequest(4, 8, "mme.example.org"),
		"other Origin-Host": appRequest(5, 7, "mme2.example.org"),
		"other command":     otherCommand,
		"other application": otherApp,
		"other content":     appRequest(6, 7, "mme.example.org", UTF8String(AVPUserName, AVPFlagMandatory, 0, "001010000000001")),
	}

	for name, req := range tests {
		if !serveOnce(t, d, req) {
			t.Errorf("%s: treated as a duplicate", name)
		}

		retransmit := *req
		retransmit.HopByHopID += 100
		retransmit.Flags |= FlagRetransmit

		if serveOnce(t, d, &retransmit) {
			t.Errorf("%s: retransmission not detected", name)
		}
	}

	checkDuplicateCache(t, d, 6)
}

func TestDuplicateCacheAcrossPaths(t *testing.T) {
	d, _ := testDuplicateCache(DefaultDuplicateCacheBytes)

	proxyInfo := func(host string) AVP {
		return Grouped(AVPProxyInfo, AVPFlagMandatory, 0,
			UTF8String(280, AVPFlagMandatory, 0, host),
			OctetString(33, AVPFlagMandatory, 0, []byte(host)))
	}

	original := appRequest(1, 7, "mme.example.org", UTF8String(AVPRouteRecord, AVPFlagMandatory, 0, "mme.example.org"), proxyInfo("dra1.example.org"))
	if !serveOnce(t, d, original) {
		t.Fatal("first request treated as a duplicate")
	}

	failover := appRequest(2, 7, "mme.example.org", UTF8String(AVPRouteRecord, AVPFlagMandatory, 0, "dra1.example.org"), proxyInfo("dra2.example.org"))
	failover.Flags |= FlagRetransmit

	e, fresh := d.begin(failover)
	if fresh {
		t.Fatal("retransmission through another agent was not detected")
	}

	ans, err := Unmarshal(e.answerFor(failover))
	if err != nil {
		t.Fatal(err)
	}

	got := FindAll(ans.AVPs, AVPProxyInfo, 0)
	if ans.HopByHopID != 2 || ans.EndToEndID != 7 || len(got) != 1 || !bytes.Equal(got[0].Data, proxyInfo("dra2.example.org").Data) {
		t.Fatalf("replayed answer hop-by-hop %d end-to-end %d Proxy-Info %v", ans.HopByHopID, ans.EndToEndID, got)
	}
}

func TestDuplicateCacheIgnoresRetransmitFlag(t *testing.T) {
	d, _ := testDuplicateCache(DefaultDuplicateCacheBytes)

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

func TestDuplicateCacheWithoutOriginHost(t *testing.T) {
	d, _ := testDuplicateCache(DefaultDuplicateCacheBytes)

	req := &Message{Flags: FlagRequest, CommandCode: 8388645, ApplicationID: testAppID, HopByHopID: 1, EndToEndID: 7}

	for range 2 {
		e, original := d.begin(req)
		if e != nil || !original {
			t.Fatal("request without Origin-Host was tracked")
		}

		d.finish(e, nil)
	}

	checkDuplicateCache(t, d, 0)
}

func TestDuplicateCacheExpiresInArrivalOrder(t *testing.T) {
	d, clock := testDuplicateCache(DefaultDuplicateCacheBytes)

	slowReq := appRequest(1, 1, "mme.example.org")
	slow, _ := d.begin(slowReq)

	clock.advance(10 * time.Second)

	fastReq := appRequest(2, 2, "mme.example.org")
	fast, _ := d.begin(fastReq)

	clock.advance(10 * time.Second)
	d.finish(fast, encodedAnswer(t, fastReq))

	clock.advance(40 * time.Second)
	d.finish(slow, encodedAnswer(t, slowReq))

	clock.advance(duplicateWindow - time.Minute)
	serveOnce(t, d, appRequest(3, 3, "mme.example.org"))

	if cached(d, 1) || !cached(d, 2) {
		t.Fatalf("after the slow request's window: slow cached = %v, fast cached = %v", cached(d, 1), cached(d, 2))
	}

	checkDuplicateCache(t, d, 2)
}

func TestDuplicateCacheByteBudget(t *testing.T) {
	answer := encodedAnswer(t, appRequest(0, 0, "mme.example.org"))
	entrySize := duplicateEntryOverhead + len("mme.example.org") + len(answer)

	var logs bytes.Buffer

	d, clock := testDuplicateCache(3 * entrySize)
	d.logger = slog.New(slog.NewTextHandler(&logs, nil))

	for i := range uint32(5) {
		e, _ := d.begin(appRequest(i, i, "mme.example.org"))
		d.finish(e, answer)
		clock.advance(time.Second)
	}

	checkDuplicateCache(t, d, 3)

	for i := range uint32(5) {
		if cached(d, i) != (i >= 2) {
			t.Errorf("End-to-End %d cached = %v", i, cached(d, i))
		}
	}

	if n := strings.Count(logs.String(), "cache is full"); n != 1 {
		t.Fatalf("logged %d eviction warnings before the report interval, want 1:\n%s", n, logs.String())
	}

	clock.advance(duplicateReportInterval)

	e, _ := d.begin(appRequest(5, 5, "mme.example.org"))
	d.finish(e, answer)

	if !strings.Contains(logs.String(), "evicted=2") {
		t.Fatalf("evictions since the last warning not reported:\n%s", logs.String())
	}
}

func TestDuplicateCacheOversizedAnswer(t *testing.T) {
	d, _ := testDuplicateCache(duplicateEntryOverhead + 64)

	req := appRequest(1, 7, "mme.example.org")
	e, _ := d.begin(req)
	d.finish(e, encodedAnswer(t, req))

	if e.answerFor(req) == nil {
		t.Fatal("waiting duplicates lost the answer")
	}

	checkDuplicateCache(t, d, 0)
}

func TestDuplicateCacheInFlight(t *testing.T) {
	d, _ := testDuplicateCache(DefaultDuplicateCacheBytes)

	var originals []*duplicateEntry

	for i := range uint32(3) {
		e, original := d.begin(appRequest(i, i, "mme.example.org"))
		if !original {
			t.Fatalf("request %d treated as a duplicate", i)
		}

		originals = append(originals, e)
	}

	for i := range uint32(3) {
		e, original := d.begin(appRequest(10+i, i, "mme.example.org"))
		if original || e != originals[i] {
			t.Fatalf("duplicate of in-flight request %d not detected", i)
		}

		select {
		case <-e.done:
			t.Fatal("in-flight entry reported done")
		default:
		}
	}

	d.mu.Lock()
	d.maxBytes = 0
	d.mu.Unlock()

	req := appRequest(0, 0, "mme.example.org")
	d.finish(originals[0], encodedAnswer(t, req))

	select {
	case <-originals[0].done:
	default:
		t.Fatal("finished entry not done")
	}

	if binary.BigEndian.Uint32(originals[0].answerFor(appRequest(42, 0, "mme.example.org"))[12:16]) != 42 {
		t.Fatal("waiting duplicate did not get the original's answer")
	}

	checkDuplicateCache(t, d, 0)

	d.finish(originals[1], encodedAnswer(t, req))

	select {
	case <-originals[1].done:
	default:
		t.Fatal("evicted in-flight entry not released on finish")
	}

	checkDuplicateCache(t, d, 0)
}

func TestDuplicateCacheSlowRequest(t *testing.T) {
	d, clock := testDuplicateCache(DefaultDuplicateCacheBytes)

	req := appRequest(1, 7, "mme.example.org")
	e, _ := d.begin(req)

	clock.advance(duplicateWindow)

	if !serveOnce(t, d, appRequest(2, 7, "mme.example.org")) {
		t.Fatal("request after the window matched an in-flight original")
	}

	d.finish(e, encodedAnswer(t, req))

	select {
	case <-e.done:
	default:
		t.Fatal("finished entry not done")
	}

	checkDuplicateCache(t, d, 1)
}

func TestDuplicateCacheConcurrent(t *testing.T) {
	d, clock := testDuplicateCache(DefaultDuplicateCacheBytes)

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

				e, original := d.begin(req)
				if !original {
					<-e.done

					if e.answerFor(req) == nil {
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

	checkDuplicateCache(t, d, keys)
}

func BenchmarkDuplicateCache(b *testing.B) {
	answer := encodedAnswer(b, appRequest(0, 0, "mme.example.org"))
	entrySize := duplicateEntryOverhead + len("mme.example.org") + len(answer)

	for _, resident := range []int{1 << 14, 1 << 20} {
		d := newDuplicateCache(resident*entrySize, slog.New(slog.DiscardHandler))

		for i := range uint32(resident) {
			e, _ := d.begin(appRequest(i, i, "mme.example.org"))
			d.finish(e, answer)
		}

		var endToEnd atomic.Uint32
		endToEnd.Store(uint32(resident))

		b.Run(fmt.Sprintf("resident=%d", resident), func(b *testing.B) {
			req := appRequest(0, 0, "mme.example.org")

			for b.Loop() {
				req.EndToEndID = endToEnd.Add(1)
				e, _ := d.begin(req)
				d.finish(e, answer)
			}
		})

		b.Run(fmt.Sprintf("resident=%d/parallel", resident), func(b *testing.B) {
			b.RunParallel(func(pb *testing.PB) {
				req := appRequest(0, 0, "mme.example.org")

				for pb.Next() {
					req.EndToEndID = endToEnd.Add(1)
					e, _ := d.begin(req)
					d.finish(e, answer)
				}
			})
		})
	}
}
