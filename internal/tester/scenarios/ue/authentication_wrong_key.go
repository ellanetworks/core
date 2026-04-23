package ue

import (
	"context"
	"fmt"
	"reflect"
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
		Name:      "ue/authentication/wrong_key",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runAuthenticationWrongKey(ctx, env, params)
		},
		Fixture: fixtureAuthenticationWrongKey,
	})
}

func fixtureAuthenticationWrongKey() scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
	}
}

func runAuthenticationWrongKey(_ context.Context, env scenarios.Env, _ any) error {
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
		GnbN3Address:  "0.0.0.0",
	})
	if err != nil {
		return fmt.Errorf("error starting gNB: %v", err)
	}

	defer gNodeB.Close()

	_, err = gNodeB.WaitForMessage(ngapType.NGAPPDUPresentSuccessfulOutcome, ngapType.SuccessfulOutcomePresentNGSetupResponse, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("timeout waiting for NGSetupComplete: %v", err)
	}

	newUE, err := ue.NewUE(&ue.UEOpts{
		PDUSessionType: PDUSessionType,
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

	err = sendAuthenticationResponseWithWrongKey(int64(scenarios.DefaultRANUENGAPID), newUE)
	if err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	return nil
}

func sendAuthenticationResponseWithWrongKey(ranUENGAPID int64, u *ue.UE) error {
	// The SNN will be used to derive wrong keys.
	u.UeSecurity.Snn = "an unreasonable serving network name"

	err := u.SendRegistrationRequest(ranUENGAPID, nasMessage.RegistrationType5GSInitialRegistration)
	if err != nil {
		return fmt.Errorf("could not build Registration Request NAS PDU: %v", err)
	}

	msg, err := u.WaitForNASGMMMessage(nas.MsgTypeAuthenticationReject, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive Authentication Reject: %v", err)
	}

	return validateAuthenticationReject(msg)
}

func validateAuthenticationReject(nasMsg *nas.Message) error {
	if nasMsg == nil {
		return fmt.Errorf("NAS PDU is nil")
	}

	if nasMsg.GmmMessage == nil {
		return fmt.Errorf("NAS message is not a GMM message")
	}

	if nasMsg.GmmMessage.GetMessageType() != nas.MsgTypeAuthenticationReject {
		return fmt.Errorf("NAS message type is not Authentication Reject (%d), got (%d)", nas.MsgTypeAuthenticationReject, nasMsg.GmmMessage.GetMessageType())
	}

	if reflect.ValueOf(nasMsg.AuthenticationReject.ExtendedProtocolDiscriminator).IsZero() {
		return fmt.Errorf("extended protocol is missing")
	}

	if nasMsg.AuthenticationReject.GetExtendedProtocolDiscriminator() != 126 {
		return fmt.Errorf("extended protocol not the expected value")
	}

	if nasMsg.AuthenticationReject.GetSecurityHeaderType() != 0 {
		return fmt.Errorf("security header type not the expected value")
	}

	if nasMsg.AuthenticationReject.GetSpareHalfOctet() != 0 {
		return fmt.Errorf("spare half octet not the expected value")
	}

	if reflect.ValueOf(nasMsg.AuthenticationReject.AuthenticationRejectMessageIdentity).IsZero() {
		return fmt.Errorf("message type is missing")
	}

	return nil
}
