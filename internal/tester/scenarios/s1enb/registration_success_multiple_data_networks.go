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
	multiDNCount    = 4
	multiDNBaseIMSI = "001017271246640"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/registration_success_multiple_data_networks",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBMultipleDataNetworks,
		Fixture:   fixtureS1ENBMultipleDataNetworks,
	})
}

func multiDNName(i int) string   { return fmt.Sprintf("mdn%d", i) }
func multiDNPool(i int) string   { return fmt.Sprintf("10.%d.0.0/16", 50+i) }
func multiDNPoolV6(i int) string { return fmt.Sprintf("fd5%d::/48", i) }

// fixtureS1ENBMultipleDataNetworks provisions N subscribers, each bound to its
// own profile/policy on a distinct data network with its own IP pool(s), so each
// attach must land on a different network. IPv6 pools are added when the env has
// IPv6 so the scenario works across address families.
func fixtureS1ENBMultipleDataNetworks(env scenarios.Env) scenarios.FixtureSpec {
	dns := make([]scenarios.DataNetworkSpec, 0, multiDNCount)
	profiles := make([]scenarios.ProfileSpec, 0, multiDNCount)
	policies := make([]scenarios.PolicySpec, 0, multiDNCount)
	subs := make([]scenarios.SubscriberSpec, 0, multiDNCount)

	for i := range multiDNCount {
		profileName := fmt.Sprintf("mdnprofile%d", i)

		dn := scenarios.DataNetworkSpec{
			Name:     multiDNName(i),
			IPv4Pool: multiDNPool(i),
			DNS:      scenarios.DefaultDNS,
			MTU:      scenarios.DefaultMTU,
		}
		if env.HasIPv6() {
			dn.IPv6Pool = multiDNPoolV6(i)
		}

		dns = append(dns, dn)
		profiles = append(profiles, scenarios.ProfileSpec{
			Name:           profileName,
			UeAmbrUplink:   scenarios.DefaultProfileUeAmbrUplink,
			UeAmbrDownlink: scenarios.DefaultProfileUeAmbrDownlink,
		})
		policies = append(policies, scenarios.PolicySpec{
			Name:                fmt.Sprintf("mdnpolicy%d", i),
			ProfileName:         profileName,
			SliceName:           scenarios.DefaultSliceName,
			DataNetworkName:     multiDNName(i),
			SessionAmbrUplink:   "100 Mbps",
			SessionAmbrDownlink: "100 Mbps",
			Var5qi:              9,
			Arp:                 15,
		})
		subs = append(subs, scenarios.DefaultSubscriberWith(nthIMSI(multiDNBaseIMSI, i), profileName))
	}

	return scenarios.FixtureSpec{DataNetworks: dns, Profiles: profiles, Policies: policies, Subscribers: subs}
}

// runS1ENBMultipleDataNetworks attaches each subscriber on one eNB and asserts
// the default bearer lands on its data network: the matching APN and a UE IPv4
// from that network's pool (TS 23.401). Sequential because the eNB sim does not
// demux attach responses per UE.
func runS1ENBMultipleDataNetworks(_ context.Context, env scenarios.Env, _ any) error {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	for i := range multiDNCount {
		imsi := nthIMSI(multiDNBaseIMSI, i)

		ue := e.NewUE(imsi, k, opc)
		ue.RequestPDNType(env.PDUSessionType())

		res, err := e.Attach(ue, 15*time.Second)
		if err != nil {
			return fmt.Errorf("attach %d/%d (imsi %s): %w", i+1, multiDNCount, imsi, err)
		}

		if err := assertAttach(res, familyExpect(env, multiDNName(i), multiDNPool(i))); err != nil {
			return fmt.Errorf("imsi %s: %w", imsi, err)
		}
	}

	return nil
}
