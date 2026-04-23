package ue

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/validate"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "ue/registration_success_multiple_data_networks",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runRegistrationSuccessMultipleDataNetworks(ctx, env, params)
		},
		Fixture: fixtureRegistrationSuccessMultipleDataNetworks,
	})
}

func fixtureRegistrationSuccessMultipleDataNetworks() scenarios.FixtureSpec {
	profiles := []scenarios.ProfileSpec{
		{Name: "profile1", UeAmbrUplink: scenarios.DefaultProfileUeAmbrUplink, UeAmbrDownlink: scenarios.DefaultProfileUeAmbrDownlink},
		{Name: "profile2", UeAmbrUplink: scenarios.DefaultProfileUeAmbrUplink, UeAmbrDownlink: scenarios.DefaultProfileUeAmbrDownlink},
		{Name: "profile3", UeAmbrUplink: scenarios.DefaultProfileUeAmbrUplink, UeAmbrDownlink: scenarios.DefaultProfileUeAmbrDownlink},
		{Name: "profile4", UeAmbrUplink: scenarios.DefaultProfileUeAmbrUplink, UeAmbrDownlink: scenarios.DefaultProfileUeAmbrDownlink},
	}

	dns := []scenarios.DataNetworkSpec{
		{Name: "dnn1", IPPool: "10.46.0.0/16", DNS: scenarios.DefaultDNS, MTU: scenarios.DefaultMTU},
		{Name: "dnn2", IPPool: "10.47.0.0/16", DNS: scenarios.DefaultDNS, MTU: scenarios.DefaultMTU},
		{Name: "dnn3", IPPool: "10.48.0.0/16", DNS: scenarios.DefaultDNS, MTU: scenarios.DefaultMTU},
		{Name: "dnn4", IPPool: "10.49.0.0/16", DNS: scenarios.DefaultDNS, MTU: scenarios.DefaultMTU},
	}

	policies := []scenarios.PolicySpec{
		{Name: "policy1", ProfileName: "profile1", SliceName: scenarios.DefaultSliceName, DataNetworkName: "dnn1", SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: 9, Arp: 15},
		{Name: "policy2", ProfileName: "profile2", SliceName: scenarios.DefaultSliceName, DataNetworkName: "dnn2", SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: 9, Arp: 15},
		{Name: "policy3", ProfileName: "profile3", SliceName: scenarios.DefaultSliceName, DataNetworkName: "dnn3", SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: 9, Arp: 15},
		{Name: "policy4", ProfileName: "profile4", SliceName: scenarios.DefaultSliceName, DataNetworkName: "dnn4", SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: 9, Arp: 15},
	}

	subs := []scenarios.SubscriberSpec{
		scenarios.DefaultSubscriberWith("001017271246546", ""),
		scenarios.DefaultSubscriberWith("001017271246547", "profile1"),
		scenarios.DefaultSubscriberWith("001017271246548", "profile2"),
		scenarios.DefaultSubscriberWith("001017271246549", "profile3"),
		scenarios.DefaultSubscriberWith("001017271246550", "profile4"),
	}

	return scenarios.FixtureSpec{
		Profiles:     profiles,
		DataNetworks: dns,
		Policies:     policies,
		Subscribers:  subs,
	}
}

func runRegistrationSuccessMultipleDataNetworks(_ context.Context, env scenarios.Env, _ any) error {
	subs := []subscriber{
		{IMSI: "001017271246546", Key: scenarios.DefaultKey, SequenceNumber: scenarios.DefaultSequenceNumber, OPc: scenarios.DefaultOPC, ProfileName: scenarios.DefaultProfileName},
		{IMSI: "001017271246547", Key: scenarios.DefaultKey, SequenceNumber: scenarios.DefaultSequenceNumber, OPc: scenarios.DefaultOPC, ProfileName: "profile1"},
		{IMSI: "001017271246548", Key: scenarios.DefaultKey, SequenceNumber: scenarios.DefaultSequenceNumber, OPc: scenarios.DefaultOPC, ProfileName: "profile2"},
		{IMSI: "001017271246549", Key: scenarios.DefaultKey, SequenceNumber: scenarios.DefaultSequenceNumber, OPc: scenarios.DefaultOPC, ProfileName: "profile3"},
		{IMSI: "001017271246550", Key: scenarios.DefaultKey, SequenceNumber: scenarios.DefaultSequenceNumber, OPc: scenarios.DefaultOPC, ProfileName: "profile4"},
	}

	dnns := []string{scenarios.DefaultDNN, "dnn1", "dnn2", "dnn3", "dnn4"}

	g := env.FirstGNB()

	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:           scenarios.DefaultGNBID,
		MCC:             scenarios.DefaultMCC,
		MNC:             scenarios.DefaultMNC,
		SST:             scenarios.DefaultSST,
		SD:              scenarios.DefaultSD,
		DNN:             dnns[0],
		TAC:             scenarios.DefaultTAC,
		Name:            "Ella-Core-Tester",
		CoreN2Addresses: env.CoreN2Addresses,
		GnbN2Address:    g.N2Address,
		GnbN3Address:    g.N3Address,
	})
	if err != nil {
		return fmt.Errorf("error starting gNB: %v", err)
	}

	defer gNodeB.Close()

	_, err = gNodeB.WaitForMessage(ngapType.NGAPPDUPresentSuccessfulOutcome, ngapType.SuccessfulOutcomePresentNGSetupResponse, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive SCTP frame: %v", err)
	}

	eg := errgroup.Group{}

	for i := range 5 {
		func() {
			eg.Go(func() error {
				ranUENGAPID := int64(scenarios.DefaultRANUENGAPID) + int64(i)

				network, err := netip.ParsePrefix(fmt.Sprintf("10.%d.0.0/22", 45+i))
				if err != nil {
					return fmt.Errorf("failed to parse UE IP subnet: %v", err)
				}

				exp := &validate.ExpectedPDUSessionEstablishmentAccept{
					PDUSessionID:               scenarios.DefaultPDUSessionID,
					PDUSessionType:             PDUSessionType,
					UeIPSubnet:                 network,
					Dnn:                        dnns[i],
					Sst:                        scenarios.DefaultSST,
					Sd:                         scenarios.DefaultSD,
					MaximumBitRateUplinkMbps:   100,
					MaximumBitRateDownlinkMbps: 100,
					Qfi:                        1,
					FiveQI:                     9,
				}

				return ueRegistrationTest(ranUENGAPID, gNodeB, subs[i], dnns[i], exp)
			})
		}()
	}

	err = eg.Wait()
	if err != nil {
		return fmt.Errorf("error during UE registrations: %v", err)
	}

	return nil
}
