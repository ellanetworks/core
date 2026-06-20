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
	multiPolicyCount    = 5
	multiPolicyBaseIMSI = "001017271246630"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/registration_success_multiple_policies",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBMultiplePolicies,
		Fixture:   fixtureS1ENBMultiplePolicies,
	})
}

// fixtureS1ENBMultiplePolicies provisions N subscribers, each bound to its own
// profile/policy pair (profile0/policy0 .. profileN-1/policyN-1) on the baseline
// slice and data network, with policy i carrying QCI 5+i.
func fixtureS1ENBMultiplePolicies(_ scenarios.Env) scenarios.FixtureSpec {
	profiles := make([]scenarios.ProfileSpec, 0, multiPolicyCount)
	policies := make([]scenarios.PolicySpec, 0, multiPolicyCount)
	subs := make([]scenarios.SubscriberSpec, 0, multiPolicyCount)

	for i := range multiPolicyCount {
		profileName := fmt.Sprintf("profile%d", i)
		policyName := fmt.Sprintf("policy%d", i)

		profiles = append(profiles, scenarios.ProfileSpec{
			Name:           profileName,
			UeAmbrUplink:   scenarios.DefaultProfileUeAmbrUplink,
			UeAmbrDownlink: scenarios.DefaultProfileUeAmbrDownlink,
		})
		policies = append(policies, scenarios.PolicySpec{
			Name:                policyName,
			ProfileName:         profileName,
			SliceName:           scenarios.DefaultSliceName,
			DataNetworkName:     scenarios.DefaultDNN,
			SessionAmbrUplink:   fmt.Sprintf("%d Mbps", 10*(i+1)),
			SessionAmbrDownlink: fmt.Sprintf("%d Mbps", 50*(i+1)),
			Var5qi:              int32(5 + i),
			Arp:                 15,
		})
		subs = append(subs, scenarios.DefaultSubscriberWith(nthIMSI(multiPolicyBaseIMSI, i), profileName))
	}

	return scenarios.FixtureSpec{Profiles: profiles, Policies: policies, Subscribers: subs}
}

// runS1ENBMultiplePolicies attaches each subscriber on one eNB and asserts the
// default bearer carries its policy's QCI (5+i), per-policy Session-AMBR
// (10·(i+1)/50·(i+1) Mbps), and the baseline signaled fields (IP∈pool, APN, PDN
// type). Sequential because the eNB sim does not demux attach responses per UE.
func runS1ENBMultiplePolicies(_ context.Context, env scenarios.Env, _ any) error {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	for i := range multiPolicyCount {
		imsi := nthIMSI(multiPolicyBaseIMSI, i)

		res, err := e.Attach(e.NewUE(imsi, k, opc), 15*time.Second)
		if err != nil {
			return fmt.Errorf("attach %d/%d (imsi %s): %w", i+1, multiPolicyCount, imsi, err)
		}

		exp := defaultExpectedAttach()
		exp.QCI = byte(5 + i)
		exp.SessAmbrUplinkBps = uint64(10*(i+1)) * mbpsToBps
		exp.SessAmbrDownlinkBps = uint64(50*(i+1)) * mbpsToBps

		if err := assertAttach(res, exp); err != nil {
			return fmt.Errorf("imsi %s: %w", imsi, err)
		}
	}

	return nil
}
