package ue

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/tests/engine"
	"github.com/ellanetworks/core/internal/tester/tests/tests/utils"
	"github.com/ellanetworks/core/internal/tester/tests/tests/utils/core"
	"github.com/ellanetworks/core/internal/tester/tests/tests/utils/procedure"
	"github.com/ellanetworks/core/internal/tester/tests/tests/utils/validate"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
)

type RegistrationSuccessMultipleSlices struct{}

func (RegistrationSuccessMultipleSlices) Meta() engine.Meta {
	return engine.Meta{
		ID:      "ue/registration_success_multiple_slices",
		Summary: "UE registration validating per-subscriber Allowed NSSAI derived from profile policies across multiple slices",
		Timeout: 60 * time.Second,
	}
}

func (t RegistrationSuccessMultipleSlices) Run(ctx context.Context, env engine.Env) error {
	const (
		slice1Name = DefaultSliceName
		slice1SST  = DefaultSST
		slice1SD   = DefaultSD

		slice2Name = "enterprise-slice"
		slice2SST  = int32(1)
		slice2SD   = "204060"
	)

	type subTest struct {
		subscriber                core.SubscriberConfig
		expectedSST               int32
		expectedSD                string
		expectedSessionAmbrUpMbps uint64
		expectedSessionAmbrDnMbps uint64
		expectedFiveQI            uint8
		expectedQfi               uint8
	}

	cases := []subTest{
		{
			subscriber: core.SubscriberConfig{
				Imsi:           "001017271246546",
				Key:            DefaultKey,
				SequenceNumber: DefaultSequenceNumber,
				OPc:            DefaultOPC,
				ProfileName:    DefaultProfileName,
			},
			expectedSST:               slice1SST,
			expectedSD:                slice1SD,
			expectedSessionAmbrUpMbps: 100,
			expectedSessionAmbrDnMbps: 100,
			expectedFiveQI:            9,
			expectedQfi:               1,
		},
		{
			subscriber: core.SubscriberConfig{
				Imsi:           "001017271246547",
				Key:            DefaultKey,
				SequenceNumber: DefaultSequenceNumber,
				OPc:            DefaultOPC,
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

	subs := make([]core.SubscriberConfig, len(cases))
	for i := range cases {
		subs[i] = cases[i].subscriber
	}

	ellaCoreEnv := core.NewEllaCoreEnv(env.EllaCoreClient, core.EllaCoreConfig{
		Operator: core.OperatorConfig{
			ID: core.OperatorID{
				MCC: DefaultMCC,
				MNC: DefaultMNC,
			},
			Tracking: core.OperatorTracking{
				SupportedTACs: []string{DefaultTAC},
			},
		},
		Profiles: []core.ProfileConfig{
			{
				Name:           DefaultProfileName,
				UeAmbrUplink:   DefaultProfileUeAmbrUplink,
				UeAmbrDownlink: DefaultProfileUeAmbrDownlink,
			},
			{
				Name:           "enterprise-profile",
				UeAmbrUplink:   DefaultProfileUeAmbrUplink,
				UeAmbrDownlink: DefaultProfileUeAmbrDownlink,
			},
		},
		Slices: []core.SliceConfig{
			{
				Name: slice1Name,
				SST:  slice1SST,
				SD:   slice1SD,
			},
			{
				Name: slice2Name,
				SST:  slice2SST,
				SD:   slice2SD,
			},
		},
		DataNetworks: []core.DataNetworkConfig{
			{
				Name:   DefaultDNN,
				IPPool: "10.45.0.0/16",
				DNS:    "8.8.8.8",
				Mtu:    1500,
			},
		},
		Policies: []core.PolicyConfig{
			{
				Name:                DefaultPolicyName,
				ProfileName:         DefaultProfileName,
				SliceName:           slice1Name,
				SessionAmbrUplink:   "100 Mbps",
				SessionAmbrDownlink: "100 Mbps",
				Var5qi:              9,
				Arp:                 15,
				DataNetworkName:     DefaultDNN,
			},
			{
				Name:                "enterprise-policy",
				ProfileName:         "enterprise-profile",
				SliceName:           slice2Name,
				SessionAmbrUplink:   "50 Mbps",
				SessionAmbrDownlink: "50 Mbps",
				Var5qi:              7,
				Arp:                 15,
				DataNetworkName:     DefaultDNN,
			},
		},
		Subscribers: subs,
	})

	err := ellaCoreEnv.Create(ctx)
	if err != nil {
		return fmt.Errorf("could not create EllaCore environment: %v", err)
	}

	logger.Logger.Debug("Created EllaCore environment")

	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:         GNBID,
		MCC:           DefaultMCC,
		MNC:           DefaultMNC,
		SST:           slice1SST,
		SD:            slice1SD,
		DNN:           DefaultDNN,
		TAC:           DefaultTAC,
		Name:          "Ella-Core-Tester",
		CoreN2Address: env.Config.EllaCore.N2Address,
		GnbN2Address:  env.Config.Gnb.N2Address,
		GnbN3Address:  env.Config.Gnb.N3Address,
		Slices: []gnb.SliceOpt{
			{Sst: slice1SST, Sd: slice1SD},
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
		ranUENGAPID := RANUENGAPID + int64(i)

		newUE, err := ue.NewUE(&ue.UEOpts{
			GnodeB:         gNodeB,
			PDUSessionID:   PDUSessionID,
			PDUSessionType: PDUSessionType,
			Msin:           tc.subscriber.Imsi[5:],
			K:              tc.subscriber.Key,
			OpC:            tc.subscriber.OPc,
			Amf:            "80000000000000000000000000000000",
			Sqn:            tc.subscriber.SequenceNumber,
			Mcc:            DefaultMCC,
			Mnc:            DefaultMNC,
			HomeNetworkPublicKey: sidf.HomeNetworkPublicKey{
				ProtectionScheme: sidf.NullScheme,
				PublicKeyID:      "0",
			},
			RoutingIndicator: "0000",
			DNN:              DefaultDNN,
			Sst:              tc.expectedSST,
			Sd:               tc.expectedSD,
			IMEISV:           "3569380356438091",
			UeSecurityCapability: utils.GetUESecurityCapability(&utils.UeSecurityCapability{
				Integrity: utils.IntegrityAlgorithms{
					Nia2: true,
				},
				Ciphering: utils.CipheringAlgorithms{
					Nea0: true,
					Nea2: true,
				},
			}),
		})
		if err != nil {
			return fmt.Errorf("could not create UE %d: %v", i, err)
		}

		gNodeB.AddUE(ranUENGAPID, newUE)

		// Step 1: Send Registration Request
		err = newUE.SendRegistrationRequest(ranUENGAPID, nasMessage.RegistrationType5GSInitialRegistration)
		if err != nil {
			return fmt.Errorf("could not send Registration Request for UE %d: %v", i, err)
		}

		// Step 2: Authentication
		_, err = newUE.WaitForNASGMMMessage(nas.MsgTypeAuthenticationRequest, 5*time.Second)
		if err != nil {
			return fmt.Errorf("did not receive Authentication Request for UE %d: %v", i, err)
		}

		// Step 3: Security Mode Command
		_, err = newUE.WaitForNASGMMMessage(nas.MsgTypeSecurityModeCommand, 5*time.Second)
		if err != nil {
			return fmt.Errorf("did not receive Security Mode Command for UE %d: %v", i, err)
		}

		// Step 4: Registration Accept — validate per-subscriber Allowed NSSAI
		nasMsg, err := newUE.WaitForNASGMMMessage(nas.MsgTypeRegistrationAccept, 5*time.Second)
		if err != nil {
			return fmt.Errorf("did not receive Registration Accept for UE %d: %v", i, err)
		}

		err = validate.RegistrationAccept(&validate.RegistrationAcceptOpts{
			NASMsg: nasMsg,
			UE:     newUE,
			Sst:    tc.expectedSST,
			Sd:     tc.expectedSD,
			Mcc:    DefaultMCC,
			Mnc:    DefaultMNC,
			ExpectedSlices: []validate.ExpectedSlice{
				{Sst: tc.expectedSST, Sd: tc.expectedSD},
			},
		})
		if err != nil {
			return fmt.Errorf("registration accept validation failed for UE %d: %v", i, err)
		}

		// Step 5: PDU Session Establishment Accept
		pduMsg, err := newUE.WaitForNASGSMMessage(nas.MsgTypePDUSessionEstablishmentAccept, 5*time.Second)
		if err != nil {
			return fmt.Errorf("did not receive PDU Session Establishment Accept for UE %d: %v", i, err)
		}

		err = validate.PDUSessionEstablishmentAccept(pduMsg, &validate.ExpectedPDUSessionEstablishmentAccept{
			PDUSessionID:               PDUSessionID,
			PDUSessionType:             PDUSessionType,
			UeIPSubnet:                 network,
			Dnn:                        DefaultDNN,
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

		// Cleanup: Deregistration
		err = procedure.Deregistration(&procedure.DeregistrationOpts{
			UE:          newUE,
			AMFUENGAPID: gNodeB.GetAMFUENGAPID(ranUENGAPID),
			RANUENGAPID: ranUENGAPID,
		})
		if err != nil {
			return fmt.Errorf("deregistration failed for UE %d: %v", i, err)
		}
	}

	err = ellaCoreEnv.Delete(ctx)
	if err != nil {
		return fmt.Errorf("could not delete EllaCore environment: %v", err)
	}

	logger.Logger.Debug("Deleted EllaCore environment")

	return nil
}
