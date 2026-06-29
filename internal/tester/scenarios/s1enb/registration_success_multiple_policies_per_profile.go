// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
)

const (
	ppProfile        = "s1enb-pp-profile"
	ppEnterpriseDNN  = "ppenterprise"
	ppEnterprisePool = "10.54.0.0/16"
	ppEnterprisePoo6 = "fd54::/48"
	ppDefaultIMSI    = "001017271246670"
	ppEnterpriseIMSI = "001017271246671"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/registration_success_multiple_policies_per_profile",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBMultiplePoliciesPerProfile,
		Fixture:   fixtureS1ENBMultiplePoliciesPerProfile,
	})
}

// One profile with two policies on distinct data networks; the enterprise policy
// is reachable only by a UE that requests its APN at attach.
func fixtureS1ENBMultiplePoliciesPerProfile(env scenarios.Env) scenarios.FixtureSpec {
	enterprise := scenarios.DataNetworkSpec{
		Name:     ppEnterpriseDNN,
		IPv4Pool: ppEnterprisePool,
		DNS:      scenarios.DefaultDNS,
		MTU:      scenarios.DefaultMTU,
	}
	if env.HasIPv6() {
		enterprise.IPv6Pool = ppEnterprisePoo6
	}

	return scenarios.FixtureSpec{
		Profiles: []scenarios.ProfileSpec{
			{Name: ppProfile, UeAmbrUplink: scenarios.DefaultProfileUeAmbrUplink, UeAmbrDownlink: scenarios.DefaultProfileUeAmbrDownlink},
		},
		DataNetworks: []scenarios.DataNetworkSpec{enterprise},
		Policies: []scenarios.PolicySpec{
			// First policy → the profile's default (internet).
			{
				Name: "s1enb-pp-default", ProfileName: ppProfile, SliceName: scenarios.DefaultSliceName,
				DataNetworkName: scenarios.DefaultDNN, SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps",
				Var5qi: 9, Arp: 15,
			},
			{
				Name: "s1enb-pp-enterprise", ProfileName: ppProfile, SliceName: scenarios.DefaultSliceName,
				DataNetworkName: ppEnterpriseDNN, SessionAmbrUplink: "30 Mbps", SessionAmbrDownlink: "60 Mbps",
				Var5qi: 7, Arp: 15,
			},
		},
		Subscribers: []scenarios.SubscriberSpec{
			scenarios.DefaultSubscriberWith(ppDefaultIMSI, ppProfile),
			scenarios.DefaultSubscriberWith(ppEnterpriseIMSI, ppProfile),
		},
	}
}

// Per-attach APN selection (TS 24.301 §6.5.1.3): a UE requesting no APN lands on
// the profile's default policy, one requesting the enterprise APN lands on that
// non-default policy.
func runS1ENBMultiplePoliciesPerProfile(_ context.Context, env scenarios.Env, _ any) error {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ueDefault := e.NewUE(ppDefaultIMSI, k, opc)
	ueDefault.RequestPDNType(env.PDUSessionType())

	resDefault, err := e.Attach(ueDefault, 15*time.Second)
	if err != nil {
		return fmt.Errorf("default-APN attach: %w", err)
	}

	if err := assertAttach(resDefault, familyExpect(env, scenarios.DefaultDNN, scenarios.DefaultUEIPv4Pool)); err != nil {
		return fmt.Errorf("default-APN UE: %w", err)
	}

	ueEnterprise := e.NewUE(ppEnterpriseIMSI, k, opc)
	ueEnterprise.RequestPDNType(env.PDUSessionType())
	ueEnterprise.RequestAPN(ppEnterpriseDNN)

	resEnterprise, err := e.Attach(ueEnterprise, 15*time.Second)
	if err != nil {
		return fmt.Errorf("enterprise-APN attach: %w", err)
	}

	expEnterprise := familyExpect(env, ppEnterpriseDNN, ppEnterprisePool)
	expEnterprise.QCI = 7
	expEnterprise.SessAmbrUplinkBps = 30 * mbpsToBps
	expEnterprise.SessAmbrDownlinkBps = 60 * mbpsToBps

	if err := assertAttach(resEnterprise, expEnterprise); err != nil {
		return fmt.Errorf("enterprise-APN UE: %w", err)
	}

	return nil
}
