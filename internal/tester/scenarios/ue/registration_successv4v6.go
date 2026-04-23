package ue

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "ue/registration_success_v4v6",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runRegistrationSuccessV4V6(ctx, env, params)
		},
	})
}

func runRegistrationSuccessV4V6(_ context.Context, env scenarios.Env, _ any) error {
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

	_, err = gNodeB.WaitForMessage(ngapType.NGAPPDUPresentSuccessfulOutcome, ngapType.SuccessfulOutcomePresentNGSetupResponse, 1*time.Second)
	if err != nil {
		return fmt.Errorf("did not receive SCTP frame: %v", err)
	}

	newUE, err := ue.NewUE(&ue.UEOpts{
		PDUSessionID:   scenarios.DefaultPDUSessionID,
		PDUSessionType: nasMessage.PDUSessionTypeIPv4IPv6,
		GnodeB:         gNodeB,
		Msin:           scenarios.DefaultIMSI[5:],
		K:              scenarios.DefaultKey,
		OpC:            scenarios.DefaultOPC,
		Amf:            scenarios.DefaultAMF,
		Sqn:            scenarios.DefaultSequenceNumber,
		Mcc:            scenarios.DefaultMCC,
		Mnc:            scenarios.DefaultMNC,
		HomeNetworkPublicKey: sidf.HomeNetworkPublicKey{
			ProtectionScheme: sidf.NullScheme,
			PublicKeyID:      "0",
		},
		RoutingIndicator: scenarios.DefaultRoutingIndicator,
		DNN:              scenarios.DefaultDNN,
		Sst:              scenarios.DefaultSST,
		Sd:               scenarios.DefaultSD,
		IMEISV:           scenarios.DefaultIMEISV,
		UeSecurityCapability: testutil.GetUESecurityCapability(&testutil.UeSecurityCapability{
			Integrity: testutil.IntegrityAlgorithms{
				Nia2: true,
			},
			Ciphering: testutil.CipheringAlgorithms{
				Nea0: true,
				Nea2: true,
			},
		}),
	})
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	gNodeB.AddUE(int64(scenarios.DefaultRANUENGAPID), newUE)

	err = runInitialRegistration(&initialRegistrationOpts{
		RANUENGAPID:            int64(scenarios.DefaultRANUENGAPID),
		PDUSessionID:           scenarios.DefaultPDUSessionID,
		ExpectedPDUSessionType: nasMessage.PDUSessionTypeIPv4,
		UE:                     newUE,
		GnodeB:                 gNodeB,
	})
	if err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	err = procedure.Deregistration(&procedure.DeregistrationOpts{
		UE:          newUE,
		AMFUENGAPID: gNodeB.GetAMFUENGAPID(int64(scenarios.DefaultRANUENGAPID)),
		RANUENGAPID: int64(scenarios.DefaultRANUENGAPID),
	})
	if err != nil {
		return fmt.Errorf("DeregistrationProcedure failed: %v", err)
	}

	return nil
}
