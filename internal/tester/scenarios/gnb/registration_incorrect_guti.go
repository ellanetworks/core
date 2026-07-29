// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/ellanetworks/core/internal/tester/testutil/validate"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/registration/incorrect_guti",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runRegistrationIncorrectGUTI(ctx, env, params)
		},
		Fixture: fixtureRegistrationIncorrectGUTI,
	})
}

func fixtureRegistrationIncorrectGUTI(env scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
	}
}

func runRegistrationIncorrectGUTI(_ context.Context, env scenarios.Env, _ any) error {
	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	// A 5G-GUTI the core never issued (TS 24.501 §9.11.3.4).
	guti := fgs.GUTIIdentity(fgs.GUTI{
		PLMN: nas.PLMN{MCC: "000", MNC: "00"}, AMFRegionID: 205, AMFSetID: 1018, AMFPointer: 1,
		TMSI: [4]byte{0x21, 0x43, 0x65, 0x84},
	})

	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:         gNodeB,
		PDUSessionID:   scenarios.DefaultPDUSessionID,
		PDUSessionType: fgs.PDUSessionType(env.PDUSessionType()),
		Guti:           &guti,
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
		ExpectedPDUSessionType: env.PDUSessionType(),
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
	err := opts.UE.SendRegistrationRequest(opts.RANUENGAPID, uint8(fgs.RegistrationTypeInitial))
	if err != nil {
		return fmt.Errorf("could not send Registration Request NAS PDU: %v", err)
	}

	nasMsg, err := opts.UE.WaitForNASGMMMessage(uint8(fgs.MsgIdentityRequest), 1*time.Second)
	if err != nil {
		return fmt.Errorf("did not receive Identity Request: %v", err)
	}

	err = validateIdentityRequest(nasMsg)
	if err != nil {
		return fmt.Errorf("NAS PDU validation failed: %v", err)
	}

	nasMsg, err = opts.UE.WaitForNASGMMMessage(uint8(fgs.MsgRegistrationAccept), 1*time.Second)
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

	msg, err := opts.UE.WaitForNASGSMMessage(uint8(fgs.MsgPDUSessionEstablishmentAccept), 500*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive PDU Session Establishment Accept: %v", err)
	}

	network, err := netip.ParsePrefix("10.45.0.0/16")
	if err != nil {
		return fmt.Errorf("failed to parse UE IP subnet: %v", err)
	}

	err = validate.PDUSessionEstablishmentAccept(msg, &validate.ExpectedPDUSessionEstablishmentAccept{
		PDUSessionID:               fgs.PDUSessionID(opts.PDUSessionID),
		PDUSessionType:             fgs.PDUSessionType(opts.ExpectedPDUSessionType),
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

func validateIdentityRequest(plain []byte) error {
	_, err := testutil.ExpectNAS[*fgs.IdentityRequest](plain)

	return err
}
