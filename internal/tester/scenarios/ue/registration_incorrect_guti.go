package ue

import (
	"context"
	"fmt"
	"net/netip"
	"reflect"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/ellanetworks/core/internal/tester/testutil/validate"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "ue/registration/incorrect_guti",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runRegistrationIncorrectGUTI(ctx, env, params)
		},
		Fixture: fixtureRegistrationIncorrectGUTI,
	})
}

func fixtureRegistrationIncorrectGUTI() scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
	}
}

func runRegistrationIncorrectGUTI(_ context.Context, env scenarios.Env, _ any) error {
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

	guti := &nasType.GUTI5G{}
	guti.SetAMFRegionID(205)
	guti.SetAMFSetID(1018)
	guti.SetAMFPointer(1)
	guti.SetTMSI5G([4]uint8{0x21, 0x43, 0x65, 0x84})
	guti.SetLen(11)
	guti.SetTypeOfIdentity(nasMessage.MobileIdentity5GSType5gGuti)
	guti.SetIei(nasMessage.RegistrationAcceptGUTI5GType)

	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:         gNodeB,
		PDUSessionID:   scenarios.DefaultPDUSessionID,
		PDUSessionType: PDUSessionType,
		Guti:           guti,
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

	err = runInitialRegistrationWithIdentityRequest(&initialRegistrationWithIdentityRequestOpts{
		Mcc:                    scenarios.DefaultMCC,
		Mnc:                    scenarios.DefaultMNC,
		Sst:                    scenarios.DefaultSST,
		Sd:                     scenarios.DefaultSD,
		DNN:                    scenarios.DefaultDNN,
		RANUENGAPID:            int64(scenarios.DefaultRANUENGAPID),
		PDUSessionID:           scenarios.DefaultPDUSessionID,
		ExpectedPDUSessionType: PDUSessionType,
		UE:                     newUE,
		GnodeB:                 gNodeB,
	})
	if err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	_, err = gNodeB.WaitForMessage(ngapType.NGAPPDUPresentInitiatingMessage, ngapType.InitiatingMessagePresentPDUSessionResourceSetupRequest, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive SCTP frame: %v", err)
	}

	_, err = gNodeB.WaitForMessage(ngapType.NGAPPDUPresentInitiatingMessage, ngapType.InitiatingMessagePresentPDUSessionResourceSetupRequest, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive SCTP frame: %v", err)
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

type initialRegistrationWithIdentityRequestOpts struct {
	Mcc                    string
	Mnc                    string
	Sst                    int32
	Sd                     string
	DNN                    string
	RANUENGAPID            int64
	PDUSessionID           uint8
	ExpectedPDUSessionType uint8
	UE                     *ue.UE
	GnodeB                 *gnb.GnodeB
}

func runInitialRegistrationWithIdentityRequest(opts *initialRegistrationWithIdentityRequestOpts) error {
	err := opts.UE.SendRegistrationRequest(opts.RANUENGAPID, nasMessage.RegistrationType5GSInitialRegistration)
	if err != nil {
		return fmt.Errorf("could not send Registration Request NAS PDU: %v", err)
	}

	nasMsg, err := opts.UE.WaitForNASGMMMessage(nas.MsgTypeIdentityRequest, 1*time.Second)
	if err != nil {
		return fmt.Errorf("did not receive Identity Request: %v", err)
	}

	err = validateIdentityRequest(nasMsg)
	if err != nil {
		return fmt.Errorf("NAS PDU validation failed: %v", err)
	}

	nasMsg, err = opts.UE.WaitForNASGMMMessage(nas.MsgTypeRegistrationAccept, 1*time.Second)
	if err != nil {
		return fmt.Errorf("did not receive Registration Accept: %v", err)
	}

	err = validate.RegistrationAccept(&validate.RegistrationAcceptOpts{
		NASMsg: nasMsg,
		UE:     opts.UE,
		Sst:    opts.Sst,
		Sd:     opts.Sd,
		Mcc:    opts.Mcc,
		Mnc:    opts.Mnc,
	})
	if err != nil {
		return fmt.Errorf("validation failed for registration accept: %v", err)
	}

	err = opts.UE.SendPDUSessionEstablishmentRequest(opts.GnodeB.GetAMFUENGAPID(opts.RANUENGAPID), opts.RANUENGAPID, opts.UE.PDUSessionID, opts.UE.DNN, opts.UE.Snssai)
	if err != nil {
		return fmt.Errorf("could not build PDU Session Establishment Request NAS PDU: %v", err)
	}

	msg, err := opts.UE.WaitForNASGSMMessage(nas.MsgTypePDUSessionEstablishmentAccept, 500*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive PDU Session Establishment Accept: %v", err)
	}

	network, err := netip.ParsePrefix("10.45.0.0/16")
	if err != nil {
		return fmt.Errorf("failed to parse UE IP subnet: %v", err)
	}

	err = validate.PDUSessionEstablishmentAccept(msg, &validate.ExpectedPDUSessionEstablishmentAccept{
		PDUSessionID:               opts.PDUSessionID,
		PDUSessionType:             opts.ExpectedPDUSessionType,
		UeIPSubnet:                 network,
		Dnn:                        opts.DNN,
		Sst:                        opts.Sst,
		Sd:                         opts.Sd,
		MaximumBitRateUplinkMbps:   100,
		MaximumBitRateDownlinkMbps: 100,
		Qfi:                        1,
		FiveQI:                     9,
	})
	if err != nil {
		return fmt.Errorf("PDUSessionResourceSetupRequest validation failed: %v", err)
	}

	return nil
}

func validateIdentityRequest(nasMsg *nas.Message) error {
	if nasMsg == nil {
		return fmt.Errorf("NAS message is nil")
	}

	if reflect.ValueOf(nasMsg.IdentityRequest.ExtendedProtocolDiscriminator).IsZero() {
		return fmt.Errorf("extended protocol is missing")
	}

	if nasMsg.IdentityRequest.GetExtendedProtocolDiscriminator() != 126 {
		return fmt.Errorf("extended protocol not the expected value")
	}

	if nasMsg.IdentityRequest.GetSpareHalfOctet() != 0 {
		return fmt.Errorf("spare half octet not the expected value")
	}

	if nasMsg.IdentityRequest.GetSecurityHeaderType() != 0 {
		return fmt.Errorf("security header type not the expected value")
	}

	if reflect.ValueOf(nasMsg.IdentityRequest.IdentityRequestMessageIdentity).IsZero() {
		return fmt.Errorf("message type is missing")
	}

	if nasMsg.IdentityRequestMessageIdentity.GetMessageType() != 91 {
		return fmt.Errorf("message type not the expected value")
	}

	if reflect.ValueOf(nasMsg.IdentityRequest.SpareHalfOctetAndIdentityType).IsZero() {
		return fmt.Errorf("spare half octet and identity type is missing")
	}

	return nil
}
