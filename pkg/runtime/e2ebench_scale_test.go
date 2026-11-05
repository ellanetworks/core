// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"flag"
	"fmt"
	"os"
	goruntime "runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/nas/fgs"
)

var (
	e2eScaleUEs         = flag.Int("e2ebench.scale.ues", 0, "UEs that register with a PDU session and stay attached in TestE2EScale; 0 skips it")
	e2eScaleRadios      = flag.Int("e2ebench.scale.radios", 100, "gNBs TestE2EScale spreads its UEs across")
	e2eScaleConcurrency = flag.Int("e2ebench.scale.concurrency", 256, "registrations TestE2EScale keeps in flight")
	e2eScaleIPv4Pool    = flag.String("e2ebench.scale.pool", "10.0.0.0/12", "IPv4 pool of the default data network in TestE2EScale")
)

const scaleProcedureTimeout = 2 * time.Minute

func TestE2EScale(t *testing.T) {
	nUEs := *e2eScaleUEs
	if nUEs <= 0 {
		t.Skip("set -e2ebench.scale.ues to run")
	}

	core := newBenchCore(t, smf.WithMaxSessions(uint32(nUEs)))
	ctx := context.Background()

	dn, err := core.db.GetDataNetwork(ctx, db.InitialDataNetworkName)
	if err != nil {
		t.Fatalf("get default data network: %v", err)
	}

	dn.IPv4Pool = *e2eScaleIPv4Pool
	if err := core.db.UpdateDataNetwork(ctx, dn); err != nil {
		t.Fatalf("widen the default data network pool: %v", err)
	}

	gnbs := make([]*gnb.GnodeB, *e2eScaleRadios)
	for i := range gnbs {
		gnbs[i], _ = core.gnbRadio(t, i)
	}

	start := time.Now()

	for i := range nUEs + 1 {
		core.provisionSubscriber(t, benchIMSI(i))
	}

	t.Logf("provisioned %d subscribers in %v", nUEs+1, time.Since(start))

	baseline := scaleMemory()

	ues := make([]*ue.UE, nUEs)
	latencies := make([]time.Duration, nUEs)
	ranUEID := func(i int) int64 { return int64(i/len(gnbs)) + 1 }

	for i := range nUEs {
		u, err := newBenchUE(gnbs[i%len(gnbs)], benchIMSI(i), true)
		if err != nil {
			t.Fatalf("create UE %d: %v", i, err)
		}

		gnbs[i%len(gnbs)].AddUE(ranUEID(i), u)
		ues[i] = u
	}

	var (
		failures atomic.Int64
		firstErr atomic.Value
		done     atomic.Int64
		wg       sync.WaitGroup
		sem      = make(chan struct{}, *e2eScaleConcurrency)
	)

	progress := time.NewTicker(10 * time.Second)
	defer progress.Stop()

	stopProgress := make(chan struct{})

	go func() {
		for {
			select {
			case <-stopProgress:
				return
			case <-progress.C:
				t.Logf("%d/%d registered, %d sessions, %d failures, %v elapsed", done.Load(), nUEs, core.cp.smf.SessionCount(), failures.Load(), time.Since(start).Round(time.Second))
			}
		}
	}()

	start = time.Now()

	for i := range nUEs {
		sem <- struct{}{}

		wg.Go(func() {
			defer func() { <-sem }()

			t0 := time.Now()
			_, err := gnbs[i%len(gnbs)].Register(ues[i], ranUEID(i), scenarios.DefaultPDUSessionID, scaleProcedureTimeout)
			latencies[i] = time.Since(t0)

			if err != nil {
				failures.Add(1)
				firstErr.CompareAndSwap(nil, fmt.Errorf("register %s: %w", benchIMSI(i), err))

				return
			}

			done.Add(1)
		})
	}

	wg.Wait()

	elapsed := time.Since(start)

	close(stopProgress)

	if err, _ := firstErr.Load().(error); err != nil {
		t.Fatalf("%d of %d registrations failed, first: %v", failures.Load(), nUEs, err)
	}

	if got := core.cp.smf.SessionCount(); got != nUEs {
		t.Fatalf("%d sessions after registering %d UEs", got, nUEs)
	}

	if got := core.cp.amf.CountRegisteredSubscribers(); got != nUEs {
		t.Fatalf("%d registered subscribers after registering %d UEs", got, nUEs)
	}

	loaded := scaleMemory()

	slices.Sort(latencies)

	pct := func(p float64) time.Duration { return latencies[min(int(float64(nUEs)*p), nUEs-1)] }

	t.Logf("registered %d UEs with a PDU session in %v (%.0f/s) over %d gNBs, %d in flight",
		nUEs, elapsed.Round(time.Millisecond), float64(nUEs)/elapsed.Seconds(), len(gnbs), *e2eScaleConcurrency)
	t.Logf("registration latency p50 %v p95 %v p99 %v max %v", pct(0.50), pct(0.95), pct(0.99), latencies[nUEs-1])
	t.Logf("process memory (core and simulator): heap in use %d MiB -> %d MiB, RSS %d MiB -> %d MiB, goroutines %d -> %d",
		baseline.heap>>20, loaded.heap>>20, baseline.rss>>20, loaded.rss>>20, baseline.goroutines, loaded.goroutines)

	extra, extraID := assertCapacityReject(t, gnbs[0], nUEs)

	if got := core.cp.smf.SessionCount(); got != nUEs {
		t.Fatalf("%d sessions after the rejected request, want %d", got, nUEs)
	}

	start = time.Now()

	for i := range nUEs {
		sem <- struct{}{}

		wg.Go(func() {
			defer func() { <-sem }()

			if err := gnbs[i%len(gnbs)].Deregister(ues[i], ranUEID(i), scaleProcedureTimeout); err != nil {
				failures.Add(1)
				firstErr.CompareAndSwap(nil, fmt.Errorf("deregister %s: %w", benchIMSI(i), err))
			}
		})
	}

	wg.Wait()

	if err := gnbs[0].Deregister(extra, extraID, scaleProcedureTimeout); err != nil {
		t.Fatalf("deregister the UE over capacity: %v", err)
	}

	if err, _ := firstErr.Load().(error); err != nil {
		t.Fatalf("%d deregistrations failed, first: %v", failures.Load(), err)
	}

	deadline := time.Now().Add(scaleProcedureTimeout)
	for core.cp.smf.SessionCount() != 0 || core.cp.amf.CountRegisteredSubscribers() != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("%d sessions and %d registrations left after deregistering", core.cp.smf.SessionCount(), core.cp.amf.CountRegisteredSubscribers())
		}

		time.Sleep(100 * time.Millisecond)
	}

	if err := verifyNoLeases(core); err != nil {
		t.Fatal(err)
	}

	t.Logf("deregistered %d UEs in %v", nUEs, time.Since(start).Round(time.Millisecond))
}

func assertCapacityReject(t *testing.T, g *gnb.GnodeB, index int) (*ue.UE, int64) {
	t.Helper()

	u, err := newBenchUE(g, benchIMSI(index), true)
	if err != nil {
		t.Fatalf("create UE over capacity: %v", err)
	}

	ranUEID := int64(1 << 30)
	g.AddUE(ranUEID, u)

	if err := u.SendRegistrationRequest(ranUEID, uint8(fgs.RegistrationTypeInitial)); err != nil {
		t.Fatalf("send registration over capacity: %v", err)
	}

	if _, err := u.WaitForNASGMMMessage(uint8(fgs.MsgRegistrationAccept), scaleProcedureTimeout); err != nil {
		t.Fatalf("registration over session capacity: %v", err)
	}

	raw, err := u.WaitForNASGSMMessage(uint8(fgs.MsgPDUSessionEstablishmentReject), scaleProcedureTimeout)
	if err != nil {
		t.Fatalf("await PDU session reject over capacity: %v", err)
	}

	reject, err := fgs.ParsePDUSessionEstablishmentReject(raw)
	if err != nil {
		t.Fatalf("parse PDU session reject: %v", err)
	}

	if reject.Cause != fgs.GSMCauseInsufficientResources {
		t.Fatalf("reject cause %v, want insufficient resources (#26)", reject.Cause)
	}

	t.Logf("session %d rejected with cause #%d, back-off %v", index+1, reject.Cause, reject.BackoffTimer)

	return u, ranUEID
}

type scaleMem struct {
	heap       uint64
	rss        uint64
	goroutines int
}

func scaleMemory() scaleMem {
	goruntime.GC()

	var ms goruntime.MemStats
	goruntime.ReadMemStats(&ms)

	m := scaleMem{heap: ms.HeapInuse, goroutines: goruntime.NumGoroutine()}

	status, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return m
	}

	for line := range strings.SplitSeq(string(status), "\n") {
		if v, ok := strings.CutPrefix(line, "VmRSS:"); ok {
			var kb uint64
			if _, err := fmt.Sscanf(strings.TrimSpace(v), "%d kB", &kb); err == nil {
				m.rss = kb << 10
			}
		}
	}

	return m
}
