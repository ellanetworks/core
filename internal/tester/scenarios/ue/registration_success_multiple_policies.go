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
		Name:      "ue/registration_success_multiple_policies",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runRegistrationSuccessMultiplePolicies(ctx, env, params)
		},
		Fixture: fixtureRegistrationSuccessMultiplePolicies,
	})
}

func fixtureRegistrationSuccessMultiplePolicies() scenarios.FixtureSpec {
	// Scenario expects UE i to see Session AMBR = (10*(i+1), 50*(i+1)) Mbps
	// with 5qi = 5+i, for i=0..4. All 5 UEs are fixture-scoped: each has its
	// own profile/policy pair (profile0/policy0 .. profile4/policy4) bound to
	// the baseline default slice and DN.
	profiles := make([]scenarios.ProfileSpec, 0, 5)
	policies := make([]scenarios.PolicySpec, 0, 5)
	subs := make([]scenarios.SubscriberSpec, 0, 5)

	for i := 0; i <= 4; i++ {
		profileName := fmt.Sprintf("profile%d", i)
		policyName := fmt.Sprintf("policy%d", i)
		imsi := fmt.Sprintf("00101727124654%d", 6+i)

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
		subs = append(subs, scenarios.DefaultSubscriberWith(imsi, profileName))
	}

	return scenarios.FixtureSpec{
		Profiles:    profiles,
		Policies:    policies,
		Subscribers: subs,
	}
}

func runRegistrationSuccessMultiplePolicies(_ context.Context, env scenarios.Env, _ any) error {
	subs := []subscriber{
		{IMSI: "001017271246546", Key: scenarios.DefaultKey, SequenceNumber: scenarios.DefaultSequenceNumber, OPc: scenarios.DefaultOPC, ProfileName: scenarios.DefaultProfileName},
		{IMSI: "001017271246547", Key: scenarios.DefaultKey, SequenceNumber: scenarios.DefaultSequenceNumber, OPc: scenarios.DefaultOPC, ProfileName: "profile1"},
		{IMSI: "001017271246548", Key: scenarios.DefaultKey, SequenceNumber: scenarios.DefaultSequenceNumber, OPc: scenarios.DefaultOPC, ProfileName: "profile2"},
		{IMSI: "001017271246549", Key: scenarios.DefaultKey, SequenceNumber: scenarios.DefaultSequenceNumber, OPc: scenarios.DefaultOPC, ProfileName: "profile3"},
		{IMSI: "001017271246550", Key: scenarios.DefaultKey, SequenceNumber: scenarios.DefaultSequenceNumber, OPc: scenarios.DefaultOPC, ProfileName: "profile4"},
	}

	g := env.FirstGNB()

	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:         scenarios.DefaultGNBID,
		MCC:           scenarios.DefaultMCC,
		MNC:           scenarios.DefaultMNC,
		SST:           scenarios.DefaultSST,
		SD:            scenarios.DefaultSD,
		DNN:           scenarios.DefaultDNN,
		TAC:           scenarios.DefaultTAC,
		Name:          "Ella-Core-Tester",
		CoreN2Address: env.FirstCore(),
		GnbN2Address:  g.N2Address,
		GnbN3Address:  g.N3Address,
	})
	if err != nil {
		return fmt.Errorf("error starting gNB: %v", err)
	}

	defer gNodeB.Close()

	_, err = gNodeB.WaitForMessage(ngapType.NGAPPDUPresentSuccessfulOutcome, ngapType.SuccessfulOutcomePresentNGSetupResponse, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive SCTP frame: %v", err)
	}

	network, err := netip.ParsePrefix("10.45.0.0/16")
	if err != nil {
		return fmt.Errorf("failed to parse UE IP subnet: %v", err)
	}

	eg := errgroup.Group{}

	for i := range subs {
		func() {
			eg.Go(func() error {
				ranUENGAPID := int64(scenarios.DefaultRANUENGAPID) + int64(i)
				exp := &validate.ExpectedPDUSessionEstablishmentAccept{
					PDUSessionID:               scenarios.DefaultPDUSessionID,
					PDUSessionType:             PDUSessionType,
					UeIPSubnet:                 network,
					Dnn:                        scenarios.DefaultDNN,
					Sst:                        scenarios.DefaultSST,
					Sd:                         scenarios.DefaultSD,
					MaximumBitRateUplinkMbps:   10 * uint64(i+1),
					MaximumBitRateDownlinkMbps: 50 * uint64(i+1),
					Qfi:                        1,
					FiveQI:                     5 + uint8(i),
				}

				return ueRegistrationTest(ranUENGAPID, gNodeB, subs[i], scenarios.DefaultDNN, exp)
			})
		}()
	}

	err = eg.Wait()
	if err != nil {
		return fmt.Errorf("error during UE registrations: %v", err)
	}

	return nil
}
