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
	})
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
		GnbID:         scenarios.DefaultGNBID,
		MCC:           scenarios.DefaultMCC,
		MNC:           scenarios.DefaultMNC,
		SST:           scenarios.DefaultSST,
		SD:            scenarios.DefaultSD,
		DNN:           dnns[0],
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
