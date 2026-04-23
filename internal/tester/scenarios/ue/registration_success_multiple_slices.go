package ue

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
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "ue/registration_success_multiple_slices",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runRegistrationSuccessMultipleSlices(ctx, env, params)
		},
		Fixture: fixtureRegistrationSuccessMultipleSlices,
	})
}

func fixtureRegistrationSuccessMultipleSlices() scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Profiles: []scenarios.ProfileSpec{
			{
				Name:           "enterprise-profile",
				UeAmbrUplink:   scenarios.DefaultProfileUeAmbrUplink,
				UeAmbrDownlink: scenarios.DefaultProfileUeAmbrDownlink,
			},
		},
		Slices: []scenarios.SliceSpec{
			{Name: "enterprise-slice", SST: 1, SD: "204060"},
		},
		Policies: []scenarios.PolicySpec{
			{
				Name:                "enterprise",
				ProfileName:         "enterprise-profile",
				SliceName:           "enterprise-slice",
				DataNetworkName:     scenarios.DefaultDNN,
				SessionAmbrUplink:   "50 Mbps",
				SessionAmbrDownlink: "50 Mbps",
				Var5qi:              7,
				Arp:                 15,
			},
		},
		Subscribers: []scenarios.SubscriberSpec{
			scenarios.DefaultSubscriberWith("001017271246546", ""),
			scenarios.DefaultSubscriberWith("001017271246547", "enterprise-profile"),
		},
	}
}

func runRegistrationSuccessMultipleSlices(_ context.Context, env scenarios.Env, _ any) error {
	const (
		slice2SST = int32(1)
		slice2SD  = "204060"
	)

	type subTest struct {
		sub                       subscriber
		expectedSST               int32
		expectedSD                string
		expectedSessionAmbrUpMbps uint64
		expectedSessionAmbrDnMbps uint64
		expectedFiveQI            uint8
		expectedQfi               uint8
	}

	cases := []subTest{
		{
			sub: subscriber{
				IMSI:           "001017271246546",
				Key:            scenarios.DefaultKey,
				SequenceNumber: scenarios.DefaultSequenceNumber,
				OPc:            scenarios.DefaultOPC,
				ProfileName:    scenarios.DefaultProfileName,
			},
			expectedSST:               scenarios.DefaultSST,
			expectedSD:                scenarios.DefaultSD,
			expectedSessionAmbrUpMbps: 100,
			expectedSessionAmbrDnMbps: 100,
			expectedFiveQI:            9,
			expectedQfi:               1,
		},
		{
			sub: subscriber{
				IMSI:           "001017271246547",
				Key:            scenarios.DefaultKey,
				SequenceNumber: scenarios.DefaultSequenceNumber,
				OPc:            scenarios.DefaultOPC,
				ProfileName:    "enterprise-profile",
			},
			expectedSST:               slice2SST,
			expectedSD:                slice2SD,
			expectedSessionAmbrUpMbps: 50,
			expectedSessionAmbrDnMbps: 50,
			expectedFiveQI:            7,
			expectedQfi:               1,
		},
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
		Slices: []gnb.SliceOpt{
			{Sst: scenarios.DefaultSST, Sd: scenarios.DefaultSD},
			{Sst: slice2SST, Sd: slice2SD},
		},
	})
	if err != nil {
		return fmt.Errorf("error starting gNB: %v", err)
	}

	defer gNodeB.Close()

	_, err = gNodeB.WaitForMessage(ngapType.NGAPPDUPresentSuccessfulOutcome, ngapType.SuccessfulOutcomePresentNGSetupResponse, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive NG Setup Response: %v", err)
	}

	network, err := netip.ParsePrefix("10.45.0.0/16")
	if err != nil {
		return fmt.Errorf("failed to parse UE IP subnet: %v", err)
	}

	for i, tc := range cases {
		ranUENGAPID := int64(scenarios.DefaultRANUENGAPID) + int64(i)

		newUE, err := ue.NewUE(&ue.UEOpts{
			GnodeB:         gNodeB,
			PDUSessionID:   scenarios.DefaultPDUSessionID,
			PDUSessionType: PDUSessionType,
			Msin:           tc.sub.IMSI[5:],
			K:              tc.sub.Key,
			OpC:            tc.sub.OPc,
			Amf:            scenarios.DefaultAMF,
			Sqn:            tc.sub.SequenceNumber,
			Mcc:            scenarios.DefaultMCC,
			Mnc:            scenarios.DefaultMNC,
			HomeNetworkPublicKey: sidf.HomeNetworkPublicKey{
				ProtectionScheme: sidf.NullScheme,
				PublicKeyID:      "0",
			},
			RoutingIndicator: scenarios.DefaultRoutingIndicator,
			DNN:              scenarios.DefaultDNN,
			Sst:              tc.expectedSST,
			Sd:               tc.expectedSD,
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
			return fmt.Errorf("could not create UE %d: %v", i, err)
		}

		gNodeB.AddUE(ranUENGAPID, newUE)

		err = newUE.SendRegistrationRequest(ranUENGAPID, nasMessage.RegistrationType5GSInitialRegistration)
		if err != nil {
			return fmt.Errorf("could not send Registration Request for UE %d: %v", i, err)
		}

		_, err = newUE.WaitForNASGMMMessage(nas.MsgTypeAuthenticationRequest, 5*time.Second)
		if err != nil {
			return fmt.Errorf("did not receive Authentication Request for UE %d: %v", i, err)
		}

		_, err = newUE.WaitForNASGMMMessage(nas.MsgTypeSecurityModeCommand, 5*time.Second)
		if err != nil {
			return fmt.Errorf("did not receive Security Mode Command for UE %d: %v", i, err)
		}

		nasMsg, err := newUE.WaitForNASGMMMessage(nas.MsgTypeRegistrationAccept, 5*time.Second)
		if err != nil {
			return fmt.Errorf("did not receive Registration Accept for UE %d: %v", i, err)
		}

		err = validate.RegistrationAccept(&validate.RegistrationAcceptOpts{
			NASMsg: nasMsg,
			UE:     newUE,
			Sst:    tc.expectedSST,
			Sd:     tc.expectedSD,
			Mcc:    scenarios.DefaultMCC,
			Mnc:    scenarios.DefaultMNC,
			ExpectedSlices: []validate.ExpectedSlice{
				{Sst: tc.expectedSST, Sd: tc.expectedSD},
			},
		})
		if err != nil {
			return fmt.Errorf("registration accept validation failed for UE %d: %v", i, err)
		}

		pduMsg, err := newUE.WaitForNASGSMMessage(nas.MsgTypePDUSessionEstablishmentAccept, 5*time.Second)
		if err != nil {
			return fmt.Errorf("did not receive PDU Session Establishment Accept for UE %d: %v", i, err)
		}

		err = validate.PDUSessionEstablishmentAccept(pduMsg, &validate.ExpectedPDUSessionEstablishmentAccept{
			PDUSessionID:               scenarios.DefaultPDUSessionID,
			PDUSessionType:             PDUSessionType,
			UeIPSubnet:                 network,
			Dnn:                        scenarios.DefaultDNN,
			Sst:                        tc.expectedSST,
			Sd:                         tc.expectedSD,
			MaximumBitRateUplinkMbps:   tc.expectedSessionAmbrUpMbps,
			MaximumBitRateDownlinkMbps: tc.expectedSessionAmbrDnMbps,
			FiveQI:                     tc.expectedFiveQI,
			Qfi:                        tc.expectedQfi,
		})
		if err != nil {
			return fmt.Errorf("PDU Session validation failed for UE %d: %v", i, err)
		}

		err = procedure.Deregistration(&procedure.DeregistrationOpts{
			UE:          newUE,
			AMFUENGAPID: gNodeB.GetAMFUENGAPID(ranUENGAPID),
			RANUENGAPID: ranUENGAPID,
		})
		if err != nil {
			return fmt.Errorf("deregistration failed for UE %d: %v", i, err)
		}
	}

	return nil
}
