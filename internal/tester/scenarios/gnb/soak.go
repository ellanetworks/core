// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

var errProbeLate = errors.New("probe echo arrived late")

type soakParams struct {
	UEs          int
	StartIMSI    string
	Duration     time.Duration
	Pause        time.Duration
	ProbeAddress string
	Timeout      time.Duration
	MaxFailures  int
}

type soakCycle struct {
	name string
	run  func(s *soakUE, p *soakParams, probe func(gnb.PDUSessionResult)) error
}

var soakCycles = []soakCycle{
	{"connected", soakConnected},
	{"idle", soakIdle},
	{"pdu-release", soakPDURelease},
	{"idle-deregister", soakIdleDeregister},
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name: "gnb/soak",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &soakParams{}
			fs.IntVar(&p.UEs, "ues", 30, "UEs cycling concurrently, spread across every --gnb")
			fs.StringVar(&p.StartIMSI, "start-imsi", "001010000000001", "IMSI of the first UE; the rest follow consecutively")
			fs.DurationVar(&p.Duration, "duration", 10*time.Minute, "how long every UE keeps cycling")
			fs.DurationVar(&p.Pause, "pause", 0, "pause between two cycles of the same UE")
			fs.StringVar(&p.ProbeAddress, "probe-address", "10.6.0.3", "data network host that echoes UDP on the probe port")
			fs.DurationVar(&p.Timeout, "timeout", 30*time.Second, "per-procedure timeout")
			fs.IntVar(&p.MaxFailures, "max-failures", 0, "failed cycles tolerated before the scenario fails")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runSoak(ctx, env, params.(*soakParams))
		},
	})
}

type soakUE struct {
	ue    *ue.UE
	gnb   *gnb.GnodeB
	ranID int64
	msin  string
	port  uint16
}

type soakStats struct {
	mu        sync.Mutex
	cycles    map[string]int
	failures  map[string]int
	latencies map[string][]time.Duration
	firstErr  error
	failed    atomic.Int64
	completed atomic.Int64
	probes    atomic.Int64
	lost      atomic.Int64
	late      atomic.Int64
}

func (st *soakStats) probe(err error) {
	st.probes.Add(1)

	switch {
	case err == nil:
	case errors.Is(err, errProbeLate):
		st.late.Add(1)
	default:
		st.lost.Add(1)
	}

	if err != nil {
		logger.Logger.Warn("soak probe failed", zap.Error(err))
	}
}

func (st *soakStats) record(kind string, d time.Duration, err error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if err != nil {
		st.failures[kind]++

		if st.firstErr == nil {
			st.firstErr = err
		}

		st.failed.Add(1)

		return
	}

	st.cycles[kind]++
	st.latencies[kind] = append(st.latencies[kind], d)
	st.completed.Add(1)
}

func runSoak(ctx context.Context, env scenarios.Env, p *soakParams) error {
	if p.UEs <= 0 || p.Duration <= 0 {
		return fmt.Errorf("--ues and --duration must be positive")
	}

	radios, err := startScaleGNBs(env)
	if err != nil {
		return err
	}

	defer func() {
		for _, g := range radios {
			g.Close()
		}
	}()

	dst := netip.AddrPortFrom(netip.MustParseAddr(p.ProbeAddress), scenarios.DefaultProbePort)

	stats := &soakStats{
		cycles:    map[string]int{},
		failures:  map[string]int{},
		latencies: map[string][]time.Duration{},
	}

	deadline := time.Now().Add(p.Duration)

	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	stopProgress := make(chan struct{})

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-stopProgress:
				return
			case <-ticker.C:
				logger.Logger.Info("soak progress", zap.Int64("cycles", stats.completed.Load()), zap.Int64("failed", stats.failed.Load()),
					zap.Int64("probes_lost", stats.lost.Load()), zap.Int64("probes_late", stats.late.Load()),
					zap.Duration("remaining", time.Until(deadline).Round(time.Second)))
			}
		}
	}()

	var wg sync.WaitGroup

	for i := range p.UEs {
		s := &soakUE{
			gnb:  radios[i%len(radios)],
			msin: incrementIMSI(p.StartIMSI, i)[5:],
			port: uint16(40000 + i%20000),
		}

		wg.Go(func() {
			probe := func(session gnb.PDUSessionResult) {
				stats.probe(soakProbe(s, session, dst))
			}

			for k := 0; ctx.Err() == nil; k++ {
				c := soakCycles[(i+k)%len(soakCycles)]
				s.ranID = int64(k)*int64(p.UEs) + int64(i) + 1

				u, err := newDefaultUE(s.gnb, s.msin, scenarios.DefaultKey, scenarios.DefaultOPC, scenarios.DefaultSequenceNumber, uint8(fgs.PDUSessionTypeIPv4))
				if err != nil {
					stats.record(c.name, 0, fmt.Errorf("create UE %s: %w", s.msin, err))
					return
				}

				s.ue = u
				s.gnb.AddUE(s.ranID, u)

				t0 := time.Now()

				err = c.run(s, p, probe)
				if err != nil {
					err = fmt.Errorf("%s cycle of %s: %w", c.name, s.ue.UeSecurity.Supi, err)
					logger.Logger.Warn("soak cycle failed", zap.Error(err))

					_ = s.gnb.Deregister(s.ue, s.ranID, 5*time.Second)
				}

				stats.record(c.name, time.Since(t0), err)

				if p.Pause > 0 {
					select {
					case <-ctx.Done():
					case <-time.After(p.Pause):
					}
				}
			}
		})
	}

	wg.Wait()
	close(stopProgress)

	stats.mu.Lock()
	defer stats.mu.Unlock()

	for _, c := range soakCycles {
		lat := stats.latencies[c.name]
		if len(lat) == 0 {
			logger.Logger.Info("soak cycle summary", zap.String("cycle", c.name), zap.Int("completed", 0), zap.Int("failed", stats.failures[c.name]))
			continue
		}

		logger.Logger.Info("soak cycle summary", append([]zap.Field{
			zap.String("cycle", c.name), zap.Int("completed", stats.cycles[c.name]), zap.Int("failed", stats.failures[c.name]),
		}, scenarios.LatencyFields(lat)...)...)
	}

	logger.Logger.Info("soak complete", zap.Int("ues", p.UEs), zap.Int("gnbs", len(radios)), zap.Duration("duration", p.Duration),
		zap.Int64("cycles", stats.completed.Load()), zap.Int64("failed", stats.failed.Load()),
		zap.Int64("probes", stats.probes.Load()), zap.Int64("probes_lost", stats.lost.Load()), zap.Int64("probes_late", stats.late.Load()))

	if stats.completed.Load() == 0 {
		return fmt.Errorf("no soak cycle completed")
	}

	if failed := stats.failed.Load(); failed > int64(p.MaxFailures) {
		return fmt.Errorf("%d soak cycles failed (tolerated %d), first: %w", failed, p.MaxFailures, stats.firstErr)
	}

	return nil
}

func soakConnected(s *soakUE, p *soakParams, probe func(gnb.PDUSessionResult)) error {
	res, err := s.gnb.Register(s.ue, s.ranID, scenarios.DefaultPDUSessionID, p.Timeout)
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}

	probe(res.Session)

	return s.gnb.Deregister(s.ue, s.ranID, p.Timeout)
}

func soakIdle(s *soakUE, p *soakParams, probe func(gnb.PDUSessionResult)) error {
	res, err := s.gnb.Register(s.ue, s.ranID, scenarios.DefaultPDUSessionID, p.Timeout)
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}

	probe(res.Session)

	if err := s.gnb.ReleaseContext(s.ue, s.ranID, []uint8{scenarios.DefaultPDUSessionID}, gnb.CauseUserInactivity, p.Timeout); err != nil {
		return fmt.Errorf("release to idle: %w", err)
	}

	sr, err := s.gnb.ServiceRequest(s.ue, s.ranID, scenarios.DefaultPDUSessionID, p.Timeout, nil)
	if err != nil {
		return fmt.Errorf("service request: %w", err)
	}

	sr.Session.UEIPv4 = res.Session.UEIPv4

	probe(sr.Session)

	return s.gnb.Deregister(s.ue, s.ranID, p.Timeout)
}

func soakPDURelease(s *soakUE, p *soakParams, probe func(gnb.PDUSessionResult)) error {
	res, err := s.gnb.Register(s.ue, s.ranID, scenarios.DefaultPDUSessionID, p.Timeout)
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}

	probe(res.Session)

	if err := s.gnb.ReleasePDUSession(s.ue, s.ranID, scenarios.DefaultPDUSessionID, p.Timeout); err != nil {
		return fmt.Errorf("release PDU session: %w", err)
	}

	return s.gnb.Deregister(s.ue, s.ranID, p.Timeout)
}

func soakIdleDeregister(s *soakUE, p *soakParams, probe func(gnb.PDUSessionResult)) error {
	res, err := s.gnb.Register(s.ue, s.ranID, scenarios.DefaultPDUSessionID, p.Timeout)
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}

	probe(res.Session)

	if err := s.gnb.ReleaseContext(s.ue, s.ranID, []uint8{scenarios.DefaultPDUSessionID}, gnb.CauseUserInactivity, p.Timeout); err != nil {
		return fmt.Errorf("release to idle: %w", err)
	}

	return s.gnb.DeregisterFromIdle(s.ue, s.ranID, p.Timeout)
}

func soakProbe(s *soakUE, session gnb.PDUSessionResult, dst netip.AddrPort) error {
	ueIP, err := netip.ParseAddr(session.UEIPv4)
	if err != nil {
		return fmt.Errorf("no UE IPv4 address: %w", err)
	}

	upf, err := netip.ParseAddr(session.UpfAddress)
	if err != nil {
		return fmt.Errorf("no UPF address: %w", err)
	}

	s.gnb.WatchTEID(session.DLTEID)

	before := s.gnb.DownlinkUDPCount(session.DLTEID, s.port)
	packet := scenarios.UDPPacket(netip.AddrPortFrom(ueIP, s.port), dst, []byte("ella-soak-probe"))

	start := time.Now()

	for range scaleProbeAttempts {
		if err := s.gnb.SendGPDUWithQFI(session.ULTEID, upf, session.QFI, packet); err != nil {
			return fmt.Errorf("send probe: %w", err)
		}

		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if s.gnb.DownlinkUDPCount(session.DLTEID, s.port) != before {
				return nil
			}

			time.Sleep(10 * time.Millisecond)
		}
	}

	late := time.Now().Add(5 * time.Second)
	for time.Now().Before(late) {
		if s.gnb.DownlinkUDPCount(session.DLTEID, s.port) != before {
			return fmt.Errorf("%w: %s (UL TEID %#x) after %v", errProbeLate, ueIP, session.ULTEID, time.Since(start).Round(100*time.Millisecond))
		}

		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("no echo for %s (UL TEID %#x)", ueIP, session.ULTEID)
}
