// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/ue"
)

const (
	stormTimeout = 60 * time.Second
	t3510        = 15 * time.Second
)

type benchTimer interface {
	StartTimer()
	StopTimer()
}

type noTimer struct{}

func (noTimer) StartTimer() {}

func (noTimer) StopTimer() {}

type stormRAN interface {
	prepare(i int) error
	register(i int) error
	deregister(i int) error
}

type storm struct {
	core *benchCore
	ran  stormRAN
	ues  int
}

func newStorm(tb testing.TB, core *benchCore, r rat, nUEs, nRadios int) *storm {
	tb.Helper()

	var ran stormRAN

	switch r {
	case rat5G:
		st := &gnbStorm{gnbs: make([]*gnb.GnodeB, nRadios), ues: make([]*ue.UE, nUEs)}
		for i := range st.gnbs {
			st.gnbs[i], _ = core.gnbRadio(tb, i)
		}

		ran = st
	case rat4G:
		st := &enbStorm{enbs: make([]*s1enb.ENB, nRadios), ues: make([]*s1enb.UE, nUEs), results: make([]*s1enb.AttachResult, nUEs)}
		for i := range st.enbs {
			st.enbs[i] = core.enbRadio(tb, i)
		}

		ran = st
	}

	for i := range nUEs {
		core.provisionSubscriber(tb, benchIMSI(i))
	}

	return &storm{core: core, ran: ran, ues: nUEs}
}

func (st *storm) round(tb testing.TB, timer benchTimer, s samples) int {
	tb.Helper()

	timer.StopTimer()

	for i := range st.ues {
		if err := st.ran.prepare(i); err != nil {
			tb.Fatal(err)
		}
	}

	latencies := make([]time.Duration, st.ues)
	errs := make([]error, st.ues)

	var wg sync.WaitGroup

	timer.StartTimer()

	for i := range st.ues {
		wg.Go(func() {
			t0 := time.Now()
			errs[i] = st.ran.register(i)
			latencies[i] = time.Since(t0)
		})
	}

	wg.Wait()

	timer.StopTimer()

	overT3510 := 0

	for i, err := range errs {
		if err != nil {
			tb.Fatalf("register %s: %v", benchIMSI(i), err)
		}

		s.add("reg", latencies[i])

		if latencies[i] > t3510 {
			overT3510++
		}
	}

	for i := range st.ues {
		wg.Go(func() {
			errs[i] = st.ran.deregister(i)
		})
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			tb.Fatalf("deregister %s: %v", benchIMSI(i), err)
		}
	}

	st.awaitSettled(tb)

	timer.StartTimer()

	return overT3510
}

func (st *storm) awaitSettled(tb testing.TB) {
	tb.Helper()

	cp := st.core.cp
	deadline := time.Now().Add(stormTimeout)

	for cp.smf.SessionCount() != 0 || cp.amf.CountRegisteredSubscribers() != 0 || cp.mme.CountRegisteredSubscribers() != 0 {
		if time.Now().After(deadline) {
			tb.Fatalf("%d sessions, %d 5G and %d 4G registrations left after the deregistration storm",
				cp.smf.SessionCount(), cp.amf.CountRegisteredSubscribers(), cp.mme.CountRegisteredSubscribers())
		}

		time.Sleep(time.Millisecond)
	}

	if err := verifyNoLeases(st.core); err != nil {
		tb.Fatal(err)
	}
}

func benchmarkStorm(b *testing.B, r rat) {
	core := newBenchCore(b)
	st := newStorm(b, core, r, *e2eBenchStormUEs, *e2eBenchStormRadios)
	all := samples{}
	overT3510 := 0
	before := core.counters()

	b.ReportAllocs()

	for b.Loop() {
		overT3510 += st.round(b, b, all)
	}

	core.reportDBWork(b, before)
	b.ReportMetric(float64(overT3510)/float64(b.N), "over-T3510/op")
	all.report(b)
}

type gnbStorm struct {
	gnbs []*gnb.GnodeB
	ues  []*ue.UE
}

func (st *gnbStorm) ranUEID(i int) int64 {
	return int64(i/len(st.gnbs)) + 1
}

func (st *gnbStorm) prepare(i int) error {
	g := st.gnbs[i%len(st.gnbs)]

	u, err := newBenchUE(g, benchIMSI(i), true)
	if err != nil {
		return fmt.Errorf("create UE: %w", err)
	}

	g.DiscardReceivedFrames()
	g.AddUE(st.ranUEID(i), u)
	st.ues[i] = u

	return nil
}

func (st *gnbStorm) register(i int) error {
	_, err := st.gnbs[i%len(st.gnbs)].Register(st.ues[i], st.ranUEID(i), scenarios.DefaultPDUSessionID, stormTimeout)

	return err
}

func (st *gnbStorm) deregister(i int) error {
	return st.gnbs[i%len(st.gnbs)].Deregister(st.ues[i], st.ranUEID(i), stormTimeout)
}

type enbStorm struct {
	enbs    []*s1enb.ENB
	ues     []*s1enb.UE
	results []*s1enb.AttachResult
}

func (st *enbStorm) prepare(i int) error {
	e := st.enbs[i%len(st.enbs)]

	e.DiscardReceivedFrames()
	st.ues[i] = e.NewUE(benchIMSI(i), benchK, benchOPc)

	return nil
}

func (st *enbStorm) register(i int) error {
	e := st.enbs[i%len(st.enbs)]

	res, err := e.Attach(st.ues[i], stormTimeout)
	if err != nil {
		return err
	}

	st.results[i] = res

	return e.WaitForEMMInformation(st.ues[i], res.ENBUES1APID, stormTimeout)
}

func (st *enbStorm) deregister(i int) error {
	res := st.results[i]

	return st.enbs[i%len(st.enbs)].SwitchOff(st.ues[i], res.MMEUES1APID, res.ENBUES1APID, stormTimeout)
}
