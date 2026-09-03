// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
)

const initialContextSetupIMSI = "001017271246622"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/initial_context_setup",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBInitialContextSetup,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(initialContextSetupIMSI, "")},
			}
		},
	})
}

func runS1ENBInitialContextSetup(_ context.Context, env scenarios.Env, _ any) error {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(initialContextSetupIMSI, k, opc)
	ue.RequestPDNType(env.PDUSessionType())

	res, err := e.Attach(ue, attachTimeout)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	if err := assertInitialContextSetupSecurity(ue, res); err != nil {
		return err
	}

	exp := familyExpect(env, scenarios.DefaultDNN, scenarios.DefaultUEIPv4Pool)
	exp.UEAmbrDownlinkBps = 100 * mbpsToBps
	exp.UEAmbrUplinkBps = 100 * mbpsToBps
	exp.ARP = 15

	return assertAttach(res, exp)
}

func assertInitialContextSetupSecurity(ue *s1enb.UE, res *s1enb.AttachResult) error {
	kenb, err := ue.KeNB()
	if err != nil {
		return err
	}

	if res.SecurityKey != s1ap.SecurityKey(kenb) {
		return fmt.Errorf("initial Context Setup Security Key does not match the K_eNB the UE derived from K_ASME and its uplink NAS COUNT; access-stratum security would fail on a real UE")
	}

	want := ue.S1APSecurityCapabilities()

	if res.UESecurityCaps.EncryptionAlgorithms != want.EncryptionAlgorithms {
		return fmt.Errorf("initial Context Setup encryption algorithms = %#04x, want the %#04x the UE advertised",
			res.UESecurityCaps.EncryptionAlgorithms, want.EncryptionAlgorithms)
	}

	if res.UESecurityCaps.IntegrityProtectionAlgorithms != want.IntegrityProtectionAlgorithms {
		return fmt.Errorf("initial Context Setup integrity algorithms = %#04x, want the %#04x the UE advertised",
			res.UESecurityCaps.IntegrityProtectionAlgorithms, want.IntegrityProtectionAlgorithms)
	}

	return nil
}
