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
		Name:      "ue/registration_success_multiple_policies_per_profile",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runRegistrationSuccessMultiplePoliciesPerProfile(ctx, env, params)
		},
		Fixture: fixtureRegistrationSuccessMultiplePoliciesPerProfile,
	})
}

func fixtureRegistrationSuccessMultiplePoliciesPerProfile() scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		DataNetworks: []scenarios.DataNetworkSpec{
			{Name: "enterprise", IPPool: "10.46.0.0/16", DNS: "8.8.4.4", MTU: scenarios.DefaultMTU},
		},
		Policies: []scenarios.PolicySpec{
			{
				Name:                "enterprise",
				ProfileName:         scenarios.DefaultProfileName,
				SliceName:           scenarios.DefaultSliceName,
				DataNetworkName:     "enterprise",
				SessionAmbrUplink:   "30 Mbps",
				SessionAmbrDownlink: "60 Mbps",
				Var5qi:              7,
				Arp:                 15,
			},
		},
		Subscribers: []scenarios.SubscriberSpec{
			scenarios.DefaultSubscriberWith("001017271246546", ""),
			scenarios.DefaultSubscriberWith("001017271246547", ""),
		},
	}
}

func runRegistrationSuccessMultiplePoliciesPerProfile(_ context.Context, env scenarios.Env, _ any) error {
	const (
		dnn2    = "enterprise"
		ipPool1 = "10.45.0.0/16"
		ipPool2 = "10.46.0.0/16"
	)

	subs := []subscriber{
		{IMSI: "001017271246546", Key: scenarios.DefaultKey, SequenceNumber: scenarios.DefaultSequenceNumber, OPc: scenarios.DefaultOPC, ProfileName: scenarios.DefaultProfileName},
		{IMSI: "001017271246547", Key: scenarios.DefaultKey, SequenceNumber: scenarios.DefaultSequenceNumber, OPc: scenarios.DefaultOPC, ProfileName: scenarios.DefaultProfileName},
	}

	g := env.FirstGNB()

	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:           scenarios.DefaultGNBID,
		MCC:             scenarios.DefaultMCC,
		MNC:             scenarios.DefaultMNC,
		SST:             scenarios.DefaultSST,
		SD:              scenarios.DefaultSD,
		DNN:             scenarios.DefaultDNN,
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

	network1, err := netip.ParsePrefix(ipPool1)
	if err != nil {
		return fmt.Errorf("failed to parse UE IP subnet: %v", err)
	}

	network2, err := netip.ParsePrefix(ipPool2)
	if err != nil {
		return fmt.Errorf("failed to parse UE IP subnet: %v", err)
	}

	dnns := []string{scenarios.DefaultDNN, dnn2}
	networks := []netip.Prefix{network1, network2}
	uplinkMbps := []uint64{100, 30}
	downlinkMbps := []uint64{100, 60}
	fiveQIs := []uint8{9, 7}

	eg := errgroup.Group{}

	for i := range subs {
		func() {
			eg.Go(func() error {
				ranUENGAPID := int64(scenarios.DefaultRANUENGAPID) + int64(i)
				exp := &validate.ExpectedPDUSessionEstablishmentAccept{
					PDUSessionID:               scenarios.DefaultPDUSessionID,
					PDUSessionType:             PDUSessionType,
					UeIPSubnet:                 networks[i],
					Dnn:                        dnns[i],
					Sst:                        scenarios.DefaultSST,
					Sd:                         scenarios.DefaultSD,
					MaximumBitRateUplinkMbps:   uplinkMbps[i],
					MaximumBitRateDownlinkMbps: downlinkMbps[i],
					Qfi:                        1,
					FiveQI:                     fiveQIs[i],
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
