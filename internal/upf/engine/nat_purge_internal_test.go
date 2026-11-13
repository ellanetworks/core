// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"sync"
	"testing"
	"time"
)

func TestNATPurgeCoalescesConcurrentReleases(t *testing.T) {
	var (
		p        natPurger
		mu       sync.Mutex
		scans    [][]uint32
		purged   = map[uint32]bool{}
		release  = make(chan struct{})
		first    = make(chan struct{})
		firstRun sync.Once
	)

	purgeSet := func(addrs map[uint32]struct{}) {
		firstRun.Do(func() {
			close(first)
			<-release
		})

		mu.Lock()
		defer mu.Unlock()

		var scan []uint32
		for a := range addrs {
			scan = append(scan, a)
			purged[a] = true
		}

		scans = append(scans, scan)
	}

	var wg sync.WaitGroup

	wg.Go(func() { p.purge(1, purgeSet) })

	<-first

	const waiting = 50

	returned := make(chan uint32, waiting)

	for a := uint32(2); a < 2+waiting; a++ {
		wg.Go(func() {
			p.purge(a, purgeSet)

			mu.Lock()
			done := purged[a]
			mu.Unlock()

			if !done {
				t.Errorf("purge of %d returned before its address was scanned", a)
			}

			returned <- a
		})
	}

	deadline := time.Now().Add(5 * time.Second)

	for {
		p.mu.Lock()
		queued := len(p.pending)
		p.mu.Unlock()

		if queued == waiting {
			break
		}

		if time.Now().After(deadline) {
			t.Fatalf("%d of %d releases queued", queued, waiting)
		}

		time.Sleep(time.Millisecond)
	}

	select {
	case a := <-returned:
		t.Fatalf("purge of %d returned while the scan it waits on was still running", a)
	default:
	}

	close(release)
	wg.Wait()

	if len(scans) != 2 || len(scans[0]) != 1 || len(scans[1]) != waiting {
		sizes := make([]int, len(scans))
		for i, s := range scans {
			sizes[i] = len(s)
		}

		t.Fatalf("scan sizes %v, want [1 %d]: releases queued behind a scan share the next one", sizes, waiting)
	}
}

func TestNATPurgeRunsAgainForAnAddressReleasedTwice(t *testing.T) {
	var (
		p     natPurger
		scans int
	)

	purgeSet := func(map[uint32]struct{}) { scans++ }

	p.purge(7, purgeSet)
	p.purge(7, purgeSet)

	if scans != 2 {
		t.Fatalf("%d scans, want one per sequential release", scans)
	}
}
