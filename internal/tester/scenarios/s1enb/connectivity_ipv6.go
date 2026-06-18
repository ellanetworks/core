// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/spf13/pflag"
)

const (
	connIPv6IMSI     = "001017271246607"
	connIPv6TunIface = "s1enbtunv60"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/connectivity_ipv6",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBConnectivityIPv6,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(connIPv6IMSI, "")},
			}
		},
	})
}

// runS1ENBConnectivityIPv6 attaches a UE with PDN type IPv6, builds an IPv6-only
// GTP-U tunnel (the link-local from the PDN IID, promoted to a global address by
// the UPF Router Advertisement), and verifies user-plane connectivity by pinging
// the N6 destination over IPv6 — the 4G counterpart of ue/connectivity_ipv6.
func runS1ENBConnectivityIPv6(ctx context.Context, env scenarios.Env, _ any) error {
	s1mme, err := s1mmeAddress(env.FirstCore())
	if err != nil {
		return err
	}

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	enbID, err := strconv.ParseUint(scenarios.DefaultGNBID, 16, 32)
	if err != nil {
		return fmt.Errorf("parse eNB ID %q: %w", scenarios.DefaultGNBID, err)
	}

	g := env.FirstGNB()

	e, err := s1enb.Start(&s1enb.StartOpts{
		ENBID: uint32(enbID), MCC: scenarios.DefaultMCC, MNC: scenarios.DefaultMNC, TAC: scenarios.DefaultTAC,
		Name: s1enbName, CoreS1MMEAddress: s1mme,
		ENBAddress: g.N2Address, ENBN3Address: g.N3Address, EnableDatapath: true,
	})
	if err != nil {
		return fmt.Errorf("start eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(connIPv6IMSI, k, opc)
	ue.RequestPDNType(eps.PDNTypeIPv6)

	res, err := e.Attach(ue, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	if res.PDNType != eps.PDNTypeIPv6 {
		return fmt.Errorf("expected negotiated PDN type IPv6 (%d), got %d", eps.PDNTypeIPv6, res.PDNType)
	}

	if res.UEIPv6 == "" {
		return fmt.Errorf("IPv6 attach assigned no IPv6 interface identifier")
	}

	if err := e.AddTunnel(&s1enb.TunnelOpts{
		UEIPv6:           res.UEIPv6 + "/64",
		UpfAddress:       res.UpfAddress,
		ULTEID:           res.ULTEID,
		DLTEID:           res.DLTEID,
		TunInterfaceName: connIPv6TunIface,
	}); err != nil {
		return fmt.Errorf("add GTP tunnel: %w", err)
	}

	defer e.CloseTunnel(res.DLTEID)

	// Wait for the UPF Router Advertisement to give the TUN a global IPv6 address.
	if err := s1enb.WaitForULAAddr(connIPv6TunIface, scenarios.DefaultUEIPv6Pool, 5*time.Second); err != nil {
		return fmt.Errorf("await SLAAC address: %w", err)
	}

	cmd := exec.CommandContext(ctx, "ping6", "-I", connIPv6TunIface, scenarios.DefaultPingDestinationV6, "-c", "3", "-W", "2") // #nosec G204 -- fixed test constants
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ping6 %s via %s failed: %v\n%s", scenarios.DefaultPingDestinationV6, connIPv6TunIface, err, string(out))
	}

	return nil
}
