// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas/eps"
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
		Name: "s1enb/scale",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &scaleParams{}
			fs.IntVar(&p.UEs, "ues", 1000, "UEs to attach, each with one default PDN connection, spread across every --gnb")
			fs.StringVar(&p.StartIMSI, "start-imsi", "001010000000001", "IMSI of the first UE; the rest follow consecutively")
			fs.IntVar(&p.Concurrency, "concurrency", 256, "attaches in flight")
			fs.IntVar(&p.ProbeEvery, "probe-every", 1000, "send a UDP probe through the user plane for every Nth UE; 0 disables probes")
			fs.StringVar(&p.ProbeAddress, "probe-address", "10.6.0.3", "data network host that echoes UDP on the probe port")
			fs.IntVar(&p.ExpectCapacityAt, "expect-capacity-at", 0, "when non-zero, attach one more UE and expect its default PDN connection rejected with ESM #26 once this many sessions exist")
			fs.DurationVar(&p.Hold, "hold", 0, "how long to keep every UE attached before detaching")
			fs.DurationVar(&p.Timeout, "timeout", 2*time.Minute, "per-procedure timeout")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runScale(ctx, env, params.(*scaleParams))
		},
	})
}

type scaleUE struct {
	ue      *s1enb.UE
	enb     *s1enb.ENB
	res     *s1enb.AttachResult
	latency time.Duration
}

func runScale(ctx context.Context, env scenarios.Env, p *scaleParams) error {
	if p.UEs <= 0 || p.Concurrency <= 0 {
		return fmt.Errorf("--ues and --concurrency must be positive")
	}

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	radios, err := startScaleENBs(env)
	if err != nil {
		return err
	}

	defer func() {
		for _, e := range radios {
			_ = e.Close()
		}
	}()

	ues := make([]*scaleUE, p.UEs)

	for i := range p.UEs {
		e := radios[i%len(radios)]

		u := e.NewUE(nthIMSI(p.StartIMSI, i), k, opc)
		u.RequestPDNType(uint8(eps.PDNTypeIPv4))

		ues[i] = &scaleUE{ue: u, enb: e}
	}

	start := time.Now()

	err = scenarios.ForEachConcurrently(ctx, ues, p.Concurrency, "attached", func(s *scaleUE) error {
		t0 := time.Now()

		res, err := s.enb.Attach(s.ue, p.Timeout)
		if err != nil {
			return fmt.Errorf("attach %s: %w", s.ue.IMSI, err)
		}

		s.latency = time.Since(t0)
		s.res = res

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

	logger.Logger.Info("registration complete", append([]zap.Field{
		zap.Int("ues", p.UEs), zap.Int("enbs", len(radios)), zap.Duration("elapsed", elapsed),
		zap.Float64("per_second", float64(p.UEs)/elapsed.Seconds()),
	}, scenarios.LatencyFields(latencies)...)...)

	if p.ProbeEvery > 0 {
		if err := probeScaleUEs(ctx, ues, p); err != nil {
			return err
		}
	}

	if p.ExpectCapacityAt > 0 {
		if err := expectCapacityReject(radios[0], p, k, opc); err != nil {
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

	start = time.Now()

	err = scenarios.ForEachConcurrently(ctx, ues, p.Concurrency, "detached", func(s *scaleUE) error {
		if err := s.enb.Detach(s.ue, s.res.MMEUES1APID, s.res.ENBUES1APID, p.Timeout); err != nil {
			return fmt.Errorf("detach %s: %w", s.ue.IMSI, err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	logger.Logger.Info("deregistration complete", zap.Int("ues", len(ues)), zap.Duration("elapsed", time.Since(start)))

	return nil
}

func startScaleENBs(env scenarios.Env) ([]*s1enb.ENB, error) {
	radios := make([]*s1enb.ENB, 0, len(env.GNBs))

	closeAll := func() {
		for _, r := range radios {
			_ = r.Close()
		}
	}

	for i, spec := range env.GNBs {
		cores := env.CoreN2Addresses
		if target, ok := env.GNBCoreTargets[spec.Name]; ok {
			cores = []string{target}
		}

		s1mme := make([]string, 0, len(cores))

		for _, c := range cores {
			addr, err := s1mmeAddress(c)
			if err != nil {
				closeAll()

				return nil, err
			}

			s1mme = append(s1mme, addr)
		}

		e, err := s1enb.Start(&s1enb.StartOpts{
			ENBID:              uint32(0x100 + i),
			MCC:                scenarios.DefaultMCC,
			MNC:                scenarios.DefaultMNC,
			TAC:                scenarios.DefaultTAC,
			Name:               "scale-" + spec.Name,
			CoreS1MMEAddresses: s1mme,
			ENBAddress:         spec.N2Address,
			ENBN3Address:       spec.N3Address,
			EnableDatapath:     true,
		})
		if err != nil {
			closeAll()

			return nil, fmt.Errorf("start eNB %s: %w", spec.Name, err)
		}

		radios = append(radios, e)
	}

	if len(radios) == 0 {
		return nil, fmt.Errorf("no eNB configured: pass at least one --gnb")
	}

	return radios, nil
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

		ueIP, err := netip.ParseAddr(s.res.UEIPv4)
		if err != nil {
			return fmt.Errorf("UE %s has no IPv4 address: %w", s.ue.IMSI, err)
		}

		upf, err := netip.ParseAddr(s.res.UpfAddress)
		if err != nil {
			return fmt.Errorf("UE %s has no UPF address: %w", s.ue.IMSI, err)
		}

		srcPort := uint16(40000 + i%20000)

		s.enb.WatchTEID(s.res.DLTEID)
		before := s.enb.DownlinkUDPCount(s.res.DLTEID, srcPort)

		packet := scenarios.UDPPacket(netip.AddrPortFrom(ueIP, srcPort), dst, []byte("ella-scale-probe"))

		for range scaleProbeAttempts {
			if err := s.enb.SendGPDU(s.res.ULTEID, upf, packet); err != nil {
				return err
			}

			deadline := time.Now().Add(time.Second)
			for s.enb.DownlinkUDPCount(s.res.DLTEID, srcPort) == before && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
			}

			if s.enb.DownlinkUDPCount(s.res.DLTEID, srcPort) != before {
				break
			}
		}

		if s.enb.DownlinkUDPCount(s.res.DLTEID, srcPort) == before {
			errs = append(errs, fmt.Errorf("no echo for UE %s (%s, UL TEID %#x)", s.ue.IMSI, ueIP, s.res.ULTEID))
		}
	}

	logger.Logger.Info("user plane probes complete", zap.Int("probed", len(sample)), zap.Int("failed", len(errs)))

	return errors.Join(errs...)
}

func expectCapacityReject(e *s1enb.ENB, p *scaleParams, k, opc [16]byte) error {
	u := e.NewUE(nthIMSI(p.StartIMSI, p.UEs), k, opc)
	u.RequestPDNType(uint8(eps.PDNTypeIPv4))

	reject, esm, err := e.AttachExpectESMReject(u, p.Timeout)
	if err != nil {
		return fmt.Errorf("attach over session capacity: %w", err)
	}

	if reject.Cause != eps.EMMCauseESMFailure {
		return fmt.Errorf("attach over capacity rejected with EMM cause %d, want #19", reject.Cause)
	}

	if esm.Cause != eps.ESMCauseInsufficientResources {
		return fmt.Errorf("PDN connectivity over capacity rejected with ESM cause %d, want #26", esm.Cause)
	}

	backoff := time.Duration(0)
	if esm.BackoffTimer != nil {
		backoff, _ = esm.BackoffTimer.Duration()
	}

	logger.Logger.Info("PDN connectivity over capacity rejected", zap.Int("sessions", p.ExpectCapacityAt),
		zap.Uint8("emm_cause", uint8(reject.Cause)), zap.Uint8("esm_cause", uint8(esm.Cause)), zap.Duration("backoff", backoff))

	return nil
}
