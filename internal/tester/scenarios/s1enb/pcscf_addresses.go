// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
)

const (
	pcscfAddressesIMSI         = "001017271246910"
	pcscfAddressesInternetIMSI = "001017271246911"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/pcscf_addresses",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBPCSCFAddresses,
		Fixture:   fixtureS1ENBPCSCFAddresses,
	})
}

func fixtureS1ENBPCSCFAddresses(env scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Profiles: []scenarios.ProfileSpec{{
			Name:           scenarios.IMSProfileName,
			UeAmbrUplink:   scenarios.DefaultProfileUeAmbrUplink,
			UeAmbrDownlink: scenarios.DefaultProfileUeAmbrDownlink,
		}},
		DataNetworks: []scenarios.DataNetworkSpec{scenarios.IMSDataNetwork(env)},
		Policies:     []scenarios.PolicySpec{scenarios.IMSPolicy(scenarios.IMSProfileName)},
		Subscribers: []scenarios.SubscriberSpec{
			scenarios.DefaultSubscriberWith(pcscfAddressesIMSI, scenarios.IMSProfileName),
			scenarios.DefaultSubscriberWith(pcscfAddressesInternetIMSI, ""),
		},
		PCSCFAddresses: scenarios.PCSCFAddresses,
	}
}

func runS1ENBPCSCFAddresses(_ context.Context, env scenarios.Env, _ any) error {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ims := familyExpect(env, scenarios.IMSDNN, scenarios.IMSUEIPv4Pool)
	ims.QCI = scenarios.IMSPolicy5QI

	cases := []struct {
		imsi   string
		expect expectedAttach
		pcscf  []netip.Addr
	}{
		{pcscfAddressesIMSI, ims, scenarios.ExpectedPCSCFAddresses(env)},
		{pcscfAddressesInternetIMSI, familyExpect(env, scenarios.DefaultDNN, scenarios.DefaultUEIPv4Pool), nil},
	}

	for _, tc := range cases {
		ue := e.NewUE(tc.imsi, k, opc)
		ue.RequestPDNType(env.PDUSessionType())
		ue.RequestPCSCFAddresses()

		res, err := e.Attach(ue, attachTimeout)
		if err != nil {
			return fmt.Errorf("attach (imsi %s): %w", tc.imsi, err)
		}

		if err := assertAttach(res, tc.expect); err != nil {
			return fmt.Errorf("imsi %s: %w", tc.imsi, err)
		}

		if err := scenarios.CheckPCSCFAddresses(res.PCSCF, tc.pcscf); err != nil {
			return fmt.Errorf("imsi %s: %w", tc.imsi, err)
		}
	}

	return nil
}
