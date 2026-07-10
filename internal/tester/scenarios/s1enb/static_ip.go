// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"net/netip"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/spf13/pflag"
)

const (
	staticIPv4IMSI = "001017271246702"
	staticIPv4Pin  = "10.45.11.11"
)

type staticIPParams struct {
	ExpectedIP string
}

func bindStaticIPFlags(fs *pflag.FlagSet) any {
	p := &staticIPParams{}
	fs.StringVar(&p.ExpectedIP, "expected-ip", "", "address the pinned subscriber must be assigned")

	return p
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/static_ip",
		BindFlags: bindStaticIPFlags,
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runS1ENBStaticIP(ctx, env, params.(*staticIPParams), staticIPv4IMSI, false)
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return staticIPFixture(staticIPv4IMSI, staticIPv4Pin)
		},
	})
}

func staticIPFixture(imsi, pin string) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(imsi, "")},
		StaticIPs:   []scenarios.StaticIPSpec{{IMSI: imsi, DataNetwork: scenarios.DefaultDNN, Address: pin}},
		ExtraArgs:   []string{"--expected-ip", pin},
	}
}

// runS1ENBStaticIP attaches a UE for a subscriber that has a pinned address
// and asserts the EPS attach assigns exactly that address.
func runS1ENBStaticIP(_ context.Context, env scenarios.Env, p *staticIPParams, imsi string, ipv6 bool) error {
	if p.ExpectedIP == "" {
		return fmt.Errorf("--expected-ip is required")
	}

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

	ue := e.NewUE(imsi, k, opc)
	if ipv6 {
		ue.RequestPDNType(eps.PDNTypeIPv6)
	}

	res, err := e.Attach(ue, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	got := res.UEIPv4
	if ipv6 {
		got = res.UEIPv6
	}

	if err := assertPinnedAddress(got, p.ExpectedIP); err != nil {
		return err
	}

	return e.Detach(ue, res.MMEUES1APID, res.ENBUES1APID, 10*time.Second)
}

// assertPinnedAddress compares the assigned and pinned addresses by value so
// that textual formatting differences do not cause spurious failures.
func assertPinnedAddress(got, want string) error {
	gotAddr, err := netip.ParseAddr(got)
	if err != nil {
		return fmt.Errorf("parse assigned address %q: %w", got, err)
	}

	wantAddr, err := netip.ParseAddr(want)
	if err != nil {
		return fmt.Errorf("parse pinned address %q: %w", want, err)
	}

	if gotAddr != wantAddr {
		return fmt.Errorf("UE was assigned %q, expected pinned address %q", got, want)
	}

	return nil
}
