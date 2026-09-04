// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

const s1enbName = "Ella-Core-Tester-S1eNB"

func startENB(env scenarios.Env) (*s1enb.ENB, error) {
	return startENBOpts(env, false)
}

// startENBWithDatapath starts an eNB with the S1-U datapath enabled, for
// scenarios that move user-plane packets rather than only signalling.
func startENBWithDatapath(env scenarios.Env) (*s1enb.ENB, error) {
	return startENBOpts(env, true)
}

func startENBOpts(env scenarios.Env, datapath bool) (*s1enb.ENB, error) {
	s1mme, err := s1mmeAddress(env.FirstCore())
	if err != nil {
		return nil, err
	}

	enbID, err := strconv.ParseUint(scenarios.DefaultGNBID, 16, 32)
	if err != nil {
		return nil, fmt.Errorf("parse eNB ID %q: %w", scenarios.DefaultGNBID, err)
	}

	g := env.FirstGNB()

	return s1enb.Start(&s1enb.StartOpts{
		ENBID:            uint32(enbID),
		MCC:              scenarios.DefaultMCC,
		MNC:              scenarios.DefaultMNC,
		TAC:              scenarios.DefaultTAC,
		Name:             s1enbName,
		CoreS1MMEAddress: s1mme,
		ENBAddress:       g.N2Address,
		ENBN3Address:     g.N3Address,
		EnableDatapath:   datapath,
	})
}

func wantsIPv6Probe(env scenarios.Env) bool {
	return env.IPFamily() == scenarios.IPv6Only
}

func handoverTunnelOpts(env scenarios.Env, res *s1enb.AttachResult, dlTEID uint32, iface string) (*s1enb.TunnelOpts, error) {
	opts := &s1enb.TunnelOpts{
		UpfAddress: res.UpfAddress, ULTEID: res.ULTEID, DLTEID: dlTEID, TunInterfaceName: iface,
	}

	if wantsIPv6Probe(env) {
		if res.UEIPv6 == "" {
			return nil, fmt.Errorf("attach assigned no IPv6 address")
		}

		opts.UEIPv6 = res.UEIPv6 + "/64"

		return opts, nil
	}

	if res.UEIPv4 == "" {
		return nil, fmt.Errorf("attach assigned no IPv4 address")
	}

	opts.UEIPv4 = res.UEIPv4 + "/16"

	return opts, nil
}

func awaitHandoverTunnelReady(env scenarios.Env, iface string) error {
	if !wantsIPv6Probe(env) {
		time.Sleep(500 * time.Millisecond)

		return nil
	}

	if err := s1enb.WaitForULAAddr(iface, scenarios.DefaultUEIPv6Pool, 5*time.Second); err != nil {
		return fmt.Errorf("timeout waiting for a ULA address on %s: %w", iface, err)
	}

	return nil
}

func handoverProbe(ctx context.Context, env scenarios.Env, iface string) error {
	return probe.Run(ctx, probe.ICMP, iface, env.PingDestination(), scenarios.DefaultProbePort, wantsIPv6Probe(env))
}
