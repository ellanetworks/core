package ue

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "ue/registration_reject/unknown_ue",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runRegistrationRejectUnknownUE(ctx, env, params)
		},
		Fixture: fixtureRegistrationRejectUnknownUE,
	})
}

func fixtureRegistrationRejectUnknownUE() scenarios.FixtureSpec {
	return scenarios.FixtureSpec{}
}

func runRegistrationRejectUnknownUE(_ context.Context, env scenarios.Env, _ any) error {
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
		return fmt.Errorf("timeout waiting for NGSetupComplete: %v", err)
	}

	secCap := testutil.UeSecurityCapability{
		Integrity: testutil.IntegrityAlgorithms{
			Nia2: true,
		},
		Ciphering: testutil.CipheringAlgorithms{
			Nea0: true,
			Nea2: true,
		},
	}

	newUEOpts := &ue.UEOpts{
		GnodeB:         gNodeB,
		PDUSessionType: PDUSessionType,
		Msin:           "1234567890",
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
		RoutingIndicator:     scenarios.DefaultRoutingIndicator,
		DNN:                  scenarios.DefaultDNN,
		Sst:                  scenarios.DefaultSST,
		Sd:                   scenarios.DefaultSD,
		UeSecurityCapability: testutil.GetUESecurityCapability(&secCap),
	}

	newUE, err := ue.NewUE(newUEOpts)
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	gNodeB.AddUE(int64(scenarios.DefaultRANUENGAPID), newUE)

	err = newUE.SendRegistrationRequest(int64(scenarios.DefaultRANUENGAPID), nasMessage.RegistrationType5GSInitialRegistration)
	if err != nil {
		return fmt.Errorf("could not send Registration Request: %v", err)
	}

	msg, err := newUE.WaitForNASGMMMessage(nas.MsgTypeRegistrationReject, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive Registration Reject: %v", err)
	}

	err = validateRegistrationReject(msg, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
	if err != nil {
		return fmt.Errorf("NAS PDU validation failed: %v", err)
	}

	return nil
}
