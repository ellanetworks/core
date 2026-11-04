// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const scaleProbeAttempts = 3

type scaleParams struct {
	UEs              int
	StartIMSI        string
	Concurrency      int
	ProbeEvery       int
	ProbeAddress     string
	ExpectCapacityAt int
	Hold             time.Duration
	Timeout          time.Duration
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name: "gnb/scale",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &scaleParams{}
			fs.IntVar(&p.UEs, "ues", 1000, "UEs to register, each with one PDU session, spread across every --gnb")
			fs.StringVar(&p.StartIMSI, "start-imsi", "001010000000001", "IMSI of the first UE; the rest follow consecutively")
			fs.IntVar(&p.Concurrency, "concurrency", 256, "registrations in flight")
			fs.IntVar(&p.ProbeEvery, "probe-every", 1000, "send a UDP probe through the user plane for every Nth UE; 0 disables probes")
			fs.StringVar(&p.ProbeAddress, "probe-address", "10.6.0.3", "data network host that echoes UDP on the probe port")
			fs.IntVar(&p.ExpectCapacityAt, "expect-capacity-at", 0, "when non-zero, register one more UE and expect its PDU session rejected with #26 once this many sessions exist")
			fs.DurationVar(&p.Hold, "hold", 0, "how long to keep every UE registered before deregistering")
			fs.DurationVar(&p.Timeout, "timeout", 2*time.Minute, "per-procedure timeout")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runScale(ctx, env, params.(*scaleParams))
		},
	})
}

type scaleUE struct {
	ue      *ue.UE
	gnb     *gnb.GnodeB
	ranID   int64
	session gnb.PDUSessionResult
	latency time.Duration
}

func runScale(ctx context.Context, env scenarios.Env, p *scaleParams) error {
	if p.UEs <= 0 || p.Concurrency <= 0 {
		return fmt.Errorf("--ues and --concurrency must be positive")
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

	ues := make([]*scaleUE, p.UEs)

	for i := range p.UEs {
		g := radios[i%len(radios)]

		u, err := newDefaultUE(g, incrementIMSI(p.StartIMSI, i)[5:], scenarios.DefaultKey, scenarios.DefaultOPC, scenarios.DefaultSequenceNumber, uint8(fgs.PDUSessionTypeIPv4))
		if err != nil {
			return fmt.Errorf("create UE %d: %w", i, err)
		}

		ues[i] = &scaleUE{ue: u, gnb: g, ranID: int64(i/len(radios)) + 1}
		g.AddUE(ues[i].ranID, u)
	}

	start := time.Now()

	err = forEachScaleUE(ctx, ues, p.Concurrency, "registered", func(s *scaleUE) error {
		t0 := time.Now()

		res, err := s.gnb.Register(s.ue, s.ranID, scenarios.DefaultPDUSessionID, p.Timeout)
		if err != nil {
			return fmt.Errorf("register %s: %w", s.ue.UeSecurity.Supi, err)
		}

		s.latency = time.Since(t0)
		s.session = res.Session

		return nil
	})
	if err != nil {
		return err
	}

	elapsed := time.Since(start)
	latencies := make([]time.Duration, len(ues))

	for i, s := range ues {
		latencies[i] = s.latency
	}

	slices.Sort(latencies)

	pct := func(q float64) time.Duration { return latencies[min(int(float64(len(latencies))*q), len(latencies)-1)] }

	logger.Logger.Info("registration complete",
		zap.Int("ues", p.UEs), zap.Int("gnbs", len(radios)), zap.Duration("elapsed", elapsed),
		zap.Float64("per_second", float64(p.UEs)/elapsed.Seconds()),
		zap.Duration("p50", pct(0.5)), zap.Duration("p95", pct(0.95)), zap.Duration("p99", pct(0.99)), zap.Duration("max", pct(1)))

	if p.ProbeEvery > 0 {
		if err := probeScaleUEs(ctx, ues, p); err != nil {
			return err
		}
	}

	var extra *scaleUE

	if p.ExpectCapacityAt > 0 {
		extra, err = expectCapacityReject(radios[0], p, len(ues))
		if err != nil {
			return err
		}
	}

	if p.Hold > 0 {
		logger.Logger.Info("holding sessions", zap.Duration("hold", p.Hold))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(p.Hold):
		}

		if p.ProbeEvery > 0 {
			if err := probeScaleUEs(ctx, ues, p); err != nil {
				return fmt.Errorf("after hold: %w", err)
			}
		}
	}

	if extra != nil {
		ues = append(ues, extra)
	}

	start = time.Now()

	err = forEachScaleUE(ctx, ues, p.Concurrency, "deregistered", func(s *scaleUE) error {
		if err := s.gnb.Deregister(s.ue, s.ranID, p.Timeout); err != nil {
			return fmt.Errorf("deregister %s: %w", s.ue.UeSecurity.Supi, err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	logger.Logger.Info("deregistration complete", zap.Int("ues", len(ues)), zap.Duration("elapsed", time.Since(start)))

	return nil
}

func startScaleGNBs(env scenarios.Env) ([]*gnb.GnodeB, error) {
	radios := make([]*gnb.GnodeB, 0, len(env.GNBs))

	for i, spec := range env.GNBs {
		g, err := gnb.Start(&gnb.StartOpts{
			GnbID:           fmt.Sprintf("%06x", 0x100+i),
			MCC:             scenarios.DefaultMCC,
			MNC:             scenarios.DefaultMNC,
			SST:             scenarios.DefaultSST,
			SD:              scenarios.DefaultSD,
			DNN:             scenarios.DefaultDNN,
			TAC:             scenarios.DefaultTAC,
			Name:            "scale-" + spec.Name,
			CoreN2Addresses: env.CoreN2Addresses,
			GnbN2Address:    spec.N2Address,
			GnbN3Address:    spec.N3Address,
		})
		if err == nil {
			_, err = g.WaitForMessage(gnb.Successful, ngap.ProcNGSetup, scenarios.NGSetupTimeout)
			if err != nil {
				g.Close()
			}
		}

		if err != nil {
			for _, r := range radios {
				r.Close()
			}

			return nil, fmt.Errorf("start gNB %s: %w", spec.Name, err)
		}

		radios = append(radios, g)
	}

	return radios, nil
}

func forEachScaleUE(ctx context.Context, ues []*scaleUE, concurrency int, verb string, fn func(*scaleUE) error) error {
	var (
		wg       sync.WaitGroup
		done     atomic.Int64
		failed   atomic.Int64
		firstErr atomic.Pointer[error]
		sem      = make(chan struct{}, concurrency)
		stop     = make(chan struct{})
	)

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				logger.Logger.Info("progress", zap.String("phase", verb), zap.Int64("done", done.Load()), zap.Int("total", len(ues)), zap.Int64("failed", failed.Load()))
			}
		}
	}()

	defer close(stop)

	for _, s := range ues {
		if ctx.Err() != nil {
			break
		}

		sem <- struct{}{}

		wg.Go(func() {
			defer func() { <-sem }()

			if err := fn(s); err != nil {
				failed.Add(1)
				firstErr.CompareAndSwap(nil, &err)

				return
			}

			done.Add(1)
		})
	}

	wg.Wait()

	if err := firstErr.Load(); err != nil {
		return fmt.Errorf("%d of %d UEs not %s, first: %w", failed.Load(), len(ues), verb, *err)
	}

	return ctx.Err()
}

func probeScaleUEs(ctx context.Context, ues []*scaleUE, p *scaleParams) error {
	dst := netip.AddrPortFrom(netip.MustParseAddr(p.ProbeAddress), scenarios.DefaultProbePort)

	var sample []*scaleUE

	for i := 0; i < len(ues); i += p.ProbeEvery {
		sample = append(sample, ues[i])
	}

	var errs []error

	for i, s := range sample {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		ueIP, err := netip.ParseAddr(s.session.UEIPv4)
		if err != nil {
			return fmt.Errorf("UE %s has no IPv4 address: %w", s.ue.UeSecurity.Supi, err)
		}

		upf, err := netip.ParseAddr(s.session.UpfAddress)
		if err != nil {
			return fmt.Errorf("UE %s has no UPF address: %w", s.ue.UeSecurity.Supi, err)
		}

		srcPort := uint16(40000 + i%20000)

		s.gnb.WatchTEID(s.session.DLTEID)
		before := s.gnb.DownlinkUDPCount(s.session.DLTEID, srcPort)

		packet := scenarios.UDPPacket(netip.AddrPortFrom(ueIP, srcPort), dst, []byte("ella-scale-probe"))

		for range scaleProbeAttempts {
			if err := s.gnb.SendGPDUWithQFI(s.session.ULTEID, upf, s.session.QFI, packet); err != nil {
				return err
			}

			deadline := time.Now().Add(time.Second)
			for s.gnb.DownlinkUDPCount(s.session.DLTEID, srcPort) == before && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
			}

			if s.gnb.DownlinkUDPCount(s.session.DLTEID, srcPort) != before {
				break
			}
		}

		if s.gnb.DownlinkUDPCount(s.session.DLTEID, srcPort) == before {
			errs = append(errs, fmt.Errorf("no echo for UE %s (%s, UL TEID %#x)", s.ue.UeSecurity.Supi, ueIP, s.session.ULTEID))
		}
	}

	logger.Logger.Info("user plane probes complete", zap.Int("probed", len(sample)), zap.Int("failed", len(errs)))

	return errors.Join(errs...)
}

func expectCapacityReject(g *gnb.GnodeB, p *scaleParams, index int) (*scaleUE, error) {
	u, err := newDefaultUE(g, incrementIMSI(p.StartIMSI, index)[5:], scenarios.DefaultKey, scenarios.DefaultOPC, scenarios.DefaultSequenceNumber, uint8(fgs.PDUSessionTypeIPv4))
	if err != nil {
		return nil, err
	}

	s := &scaleUE{ue: u, gnb: g, ranID: 1 << 30}
	g.AddUE(s.ranID, u)

	if err := u.SendRegistrationRequest(s.ranID, uint8(fgs.RegistrationTypeInitial)); err != nil {
		return nil, fmt.Errorf("send registration over capacity: %w", err)
	}

	if _, err := u.WaitForNASGMMMessage(uint8(fgs.MsgRegistrationAccept), p.Timeout); err != nil {
		return nil, fmt.Errorf("registration over session capacity: %w", err)
	}

	raw, err := u.WaitForNASGSMMessage(uint8(fgs.MsgPDUSessionEstablishmentReject), p.Timeout)
	if err != nil {
		return nil, fmt.Errorf("await PDU session reject over capacity: %w", err)
	}

	reject, err := fgs.ParsePDUSessionEstablishmentReject(raw)
	if err != nil {
		return nil, err
	}

	if reject.Cause != fgs.GSMCauseInsufficientResources {
		return nil, fmt.Errorf("PDU session over capacity rejected with cause %d, want #26", reject.Cause)
	}

	backoff := time.Duration(0)
	if reject.BackoffTimer != nil {
		backoff, _ = reject.BackoffTimer.Duration()
	}

	logger.Logger.Info("PDU session over capacity rejected", zap.Int("sessions", p.ExpectCapacityAt), zap.Uint8("cause", uint8(reject.Cause)), zap.Duration("backoff", backoff))

	return s, nil
}
