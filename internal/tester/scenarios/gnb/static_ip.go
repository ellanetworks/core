// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/spf13/pflag"
)

const (
	staticIPv4IMSI = "001017271246700"
	staticIPv4Pin  = "10.45.9.9"
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
		Name:      "gnb/static_ip",
		BindFlags: bindStaticIPFlags,
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runStaticIP(ctx, env, params.(*staticIPParams), staticIPv4IMSI, false)
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

// runStaticIP registers a UE for a subscriber that has a pinned address and
// asserts the PDU session establishment assigns exactly that address.
func runStaticIP(_ context.Context, env scenarios.Env, p *staticIPParams, imsi string, ipv6 bool) error {
	if p.ExpectedIP == "" {
		return fmt.Errorf("--expected-ip is required")
	}

	subs, err := buildSubscribers(1, imsi)
	if err != nil {
		return fmt.Errorf("build subscriber: %v", err)
	}

	sub := subs[0]

	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	newUE, err := newDefaultUE(gNodeB, sub.IMSI[5:], sub.Key, sub.OPc, sub.SequenceNumber, env.PDUSessionType())
	if err != nil {
		return fmt.Errorf("create UE: %v", err)
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	if _, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	}); err != nil {
		return fmt.Errorf("initial registration: %v", err)
	}

	session, err := newUE.WaitForPDUSession(scenarios.DefaultPDUSessionID, 5*time.Second)
	if err != nil {
		return fmt.Errorf("wait for PDU session: %v", err)
	}

	got := session.UEIP
	if ipv6 {
		got = session.UEIPV6
	}

	if err := assertPinnedAddress(got, p.ExpectedIP); err != nil {
		return err
	}

	return procedure.Deregistration(&procedure.DeregistrationOpts{
		UE:          newUE,
		AMFUENGAPID: gNodeB.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID: ranUENGAPID,
	})
}

// assertPinnedAddress compares the assigned and pinned addresses by value so
// that differences in textual formatting (e.g. IPv6 zero-compression) do not
// cause spurious failures.
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
