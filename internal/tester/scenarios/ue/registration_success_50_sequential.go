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
)

const (
	numSubscribersSequential = 50
	sequentialStartIMSI      = "001017271246546"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "ue/registration_success_50_sequential",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runRegistrationSuccess50Sequential(ctx, env, params)
		},
	})
}

func runRegistrationSuccess50Sequential(_ context.Context, env scenarios.Env, _ any) error {
	subs, err := buildSubscribers(numSubscribersSequential, sequentialStartIMSI)
	if err != nil {
		return fmt.Errorf("could not build subscriber config: %v", err)
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

	for i := range numSubscribersSequential {
		ranUENGAPID := int64(scenarios.DefaultRANUENGAPID) + int64(i)

		exp := &validate.ExpectedPDUSessionEstablishmentAccept{
			PDUSessionID:               scenarios.DefaultPDUSessionID,
			PDUSessionType:             PDUSessionType,
			UeIPSubnet:                 network,
			Dnn:                        scenarios.DefaultDNN,
			Sst:                        scenarios.DefaultSST,
			Sd:                         scenarios.DefaultSD,
			MaximumBitRateUplinkMbps:   100,
			MaximumBitRateDownlinkMbps: 100,
			Qfi:                        1,
			FiveQI:                     9,
		}

		err := ueRegistrationTest(ranUENGAPID, gNodeB, subs[i], scenarios.DefaultDNN, exp)
		if err != nil {
			return fmt.Errorf("UE registration test failed for subscriber %d: %v", i, err)
		}
	}

	return nil
}
