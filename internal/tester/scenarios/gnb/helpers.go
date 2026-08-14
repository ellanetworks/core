// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/ngap"
)

func wantsIPv6Probe(env scenarios.Env) bool {
	return env.IPFamily() == scenarios.IPv6Only
}

func handoverTunnelAddress(env scenarios.Env, session ue.PDUSessionInfo) string {
	if wantsIPv6Probe(env) {
		return session.UEIPV6 + "/64"
	}

	return session.UEIP + "/16"
}

func awaitHandoverTunnelReady(env scenarios.Env, iface string) error {
	if !wantsIPv6Probe(env) {
		time.Sleep(500 * time.Millisecond)

		return nil
	}

	if err := gnb.WaitForULAAddr(iface, scenarios.DefaultUEIPv6Pool, 5*time.Second); err != nil {
		return fmt.Errorf("timeout waiting for a ULA address on %s: %w", iface, err)
	}

	return nil
}

func handoverProbe(ctx context.Context, env scenarios.Env, iface string) error {
	return probe.Run(ctx, probe.ICMP, iface, env.PingDestination(), scenarios.DefaultProbePort, wantsIPv6Probe(env))
}

func startGNB(env scenarios.Env) (*gnb.GnodeB, error) {
	g := env.FirstGNB()

	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:           scenarios.DefaultGNBID,
		MCC:             scenarios.DefaultMCC,
		MNC:             scenarios.DefaultMNC,
		SST:             scenarios.DefaultSST,
		SD:              scenarios.DefaultSD,
		DNN:             scenarios.DefaultDNN,
		TAC:             scenarios.DefaultTAC,
		Name:            "Ella-Core-Tester",
		CoreN2Addresses: env.CoreN2Addresses,
		GnbN2Address:    g.N2Address,
		GnbN3Address:    g.N3Address,
	})
	if err != nil {
		return nil, fmt.Errorf("start gNB: %w", err)
	}

	if _, err := gNodeB.WaitForMessage(gnb.Successful, ngap.ProcNGSetup, 200*time.Millisecond); err != nil {
		gNodeB.Close()

		return nil, fmt.Errorf("await NG Setup Response: %w", err)
	}

	return gNodeB, nil
}
