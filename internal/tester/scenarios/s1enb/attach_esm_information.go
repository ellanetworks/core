// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
)

const (
	esmInfoProfile = "s1enb-esminfo-profile"
	esmInfoDNN     = "esminfo"
	esmInfoPool    = "10.55.0.0/16"
	esmInfoPoolV6  = "fd55::/48"
	esmInfoIMSI    = "001017271246618"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/attach_esm_information",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBAttachESMInformation,
		Fixture:   fixtureS1ENBAttachESMInformation,
	})
}

func fixtureS1ENBAttachESMInformation(env scenarios.Env) scenarios.FixtureSpec {
	dn := scenarios.DataNetworkSpec{
		Name:     esmInfoDNN,
		IPv4Pool: esmInfoPool,
		DNS:      scenarios.DefaultDNS,
		MTU:      scenarios.DefaultMTU,
	}
	if env.HasIPv6() {
		dn.IPv6Pool = esmInfoPoolV6
	}

	return scenarios.FixtureSpec{
		Profiles: []scenarios.ProfileSpec{
			{Name: esmInfoProfile, UeAmbrUplink: scenarios.DefaultProfileUeAmbrUplink, UeAmbrDownlink: scenarios.DefaultProfileUeAmbrDownlink},
		},
		DataNetworks: []scenarios.DataNetworkSpec{dn},
		Policies: []scenarios.PolicySpec{
			{
				Name: "s1enb-esminfo-default", ProfileName: esmInfoProfile, SliceName: scenarios.DefaultSliceName,
				DataNetworkName: scenarios.DefaultDNN, SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps",
				Var5qi: 9, Arp: 15,
			},
			{
				Name: "s1enb-esminfo-deferred", ProfileName: esmInfoProfile, SliceName: scenarios.DefaultSliceName,
				DataNetworkName: esmInfoDNN, SessionAmbrUplink: "20 Mbps", SessionAmbrDownlink: "40 Mbps",
				Var5qi: 6, Arp: 15,
			},
		},
		Subscribers: []scenarios.SubscriberSpec{
			scenarios.DefaultSubscriberWith(esmInfoIMSI, esmInfoProfile),
		},
	}
}

func runS1ENBAttachESMInformation(_ context.Context, env scenarios.Env, _ any) error {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(esmInfoIMSI, k, opc)
	ue.RequestPDNType(env.PDUSessionType())
	ue.RequestAPN(esmInfoDNN)
	ue.DeferESMInformation()

	res, err := e.Attach(ue, attachTimeout)
	if err != nil {
		return fmt.Errorf("attach with deferred ESM information: %w", err)
	}

	exp := familyExpect(env, esmInfoDNN, esmInfoPool)
	exp.QCI = 6
	exp.SessAmbrUplinkBps = 20 * mbpsToBps
	exp.SessAmbrDownlinkBps = 40 * mbpsToBps

	return assertAttach(res, exp)
}
