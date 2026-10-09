// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/validate"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/spf13/pflag"
)

const (
	pcscfAddressesIMSI         = "001017271246900"
	pcscfAddressesInternetIMSI = "001017271246901"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/pcscf_addresses",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runPCSCFAddresses(ctx, env)
		},
		Fixture: fixturePCSCFAddresses,
	})
}

func fixturePCSCFAddresses(env scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Profiles: []scenarios.ProfileSpec{{
			Name:           scenarios.IMSProfileName,
			UeAmbrUplink:   scenarios.DefaultProfileUeAmbrUplink,
			UeAmbrDownlink: scenarios.DefaultProfileUeAmbrDownlink,
		}},
		DataNetworks: []scenarios.DataNetworkSpec{scenarios.IMSDataNetwork(env)},
		Policies: []scenarios.PolicySpec{
			scenarios.IMSPolicy(scenarios.IMSProfileName),
			{
				Name:                scenarios.IMSInternetPolicyName,
				ProfileName:         scenarios.IMSProfileName,
				SliceName:           scenarios.DefaultSliceName,
				DataNetworkName:     scenarios.DefaultDNN,
				SessionAmbrUplink:   "100 Mbps",
				SessionAmbrDownlink: "100 Mbps",
				Var5qi:              9,
				Arp:                 15,
			},
		},
		Subscribers: []scenarios.SubscriberSpec{
			scenarios.DefaultSubscriberWith(pcscfAddressesIMSI, scenarios.IMSProfileName),
			scenarios.DefaultSubscriberWith(pcscfAddressesInternetIMSI, scenarios.IMSProfileName),
		},
		PCSCFAddresses: scenarios.PCSCFAddresses,
	}
}

func runPCSCFAddresses(_ context.Context, env scenarios.Env) error {
	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	cases := []struct {
		imsi    string
		dnn     string
		subnet  string
		fiveQI  uint8
		pcscf   []netip.Addr
		ranUEID int64
	}{
		{pcscfAddressesIMSI, scenarios.IMSDNN, scenarios.IMSUEIPv4Pool, scenarios.IMSPolicy5QI, scenarios.ExpectedPCSCFAddresses(env), scenarios.DefaultRANUENGAPID},
		{pcscfAddressesInternetIMSI, scenarios.DefaultDNN, scenarios.DefaultUEIPv4Pool, 9, nil, scenarios.DefaultRANUENGAPID + 1},
	}

	for _, tc := range cases {
		sub := subscriber{IMSI: tc.imsi, Key: scenarios.DefaultKey, SequenceNumber: scenarios.DefaultSequenceNumber, OPc: scenarios.DefaultOPC, ProfileName: scenarios.IMSProfileName}

		newUE, err := newSubscriberUE(gNodeB, sub, tc.dnn, env.PDUSessionType())
		if err != nil {
			return err
		}

		gNodeB.AddUE(tc.ranUEID, newUE)

		registration, err := gNodeB.Register(newUE, tc.ranUEID, scenarios.DefaultPDUSessionID, registrationTimeout)
		if err != nil {
			return fmt.Errorf("registration on %s failed: %v", tc.dnn, err)
		}

		err = validate.PDUSessionEstablishmentAccept(registration.Session.Accept, &validate.ExpectedPDUSessionEstablishmentAccept{
			PDUSessionID:               scenarios.DefaultPDUSessionID,
			PDUSessionType:             fgs.PDUSessionType(env.PDUSessionType()),
			UeIPSubnet:                 netip.MustParsePrefix(tc.subnet),
			Dnn:                        tc.dnn,
			Sst:                        scenarios.DefaultSST,
			Sd:                         scenarios.DefaultSD,
			MaximumBitRateUplinkMbps:   100,
			MaximumBitRateDownlinkMbps: 100,
			Qfi:                        1,
			FiveQI:                     tc.fiveQI,
		})
		if err != nil {
			return fmt.Errorf("%s: %v", tc.dnn, err)
		}

		acc, err := fgs.ParsePDUSessionEstablishmentAccept(registration.Session.Accept)
		if err != nil {
			return fmt.Errorf("%s: parse PDU Session Establishment Accept: %v", tc.dnn, err)
		}

		var got []netip.Addr
		if acc.ExtendedPCO != nil {
			got = acc.ExtendedPCO.PCSCFAddresses()
		}

		if err := scenarios.CheckPCSCFAddresses(got, tc.pcscf); err != nil {
			return fmt.Errorf("%s: %v", tc.dnn, err)
		}

		if err := gNodeB.Deregister(newUE, tc.ranUEID, releaseTimeout); err != nil {
			return fmt.Errorf("%s: deregistration failed: %v", tc.dnn, err)
		}
	}

	return nil
}
