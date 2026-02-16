package ue

import (
	"context"
	"fmt"
	"net/netip"
	"strconv"
	"time"

	"github.com/ellanetworks/core/core-tester/internal/gnb"
	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/ellanetworks/core/core-tester/internal/tests/engine"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils/core"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils/procedure"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils/validate"
	"github.com/ellanetworks/core/core-tester/internal/ue"
	"github.com/ellanetworks/core/core-tester/internal/ue/sidf"
	"github.com/free5gc/ngap/ngapType"
)

const (
	NumSubscribersSequential = 50
	testStartIMSI            = "001017271246546"
)

type RegistrationSuccess50Sequential struct{}

func (RegistrationSuccess50Sequential) Meta() engine.Meta {
	return engine.Meta{
		ID:      "ue/registration_success_50_sequential",
		Summary: "UE sequential registration success test validating the Registration Request and Authentication procedures with 50 UEs",
		Timeout: 60 * time.Second,
	}
}

func computeIMSI(baseIMSI string, increment int) (string, error) {
	intBaseImsi, err := strconv.Atoi(baseIMSI)
	if err != nil {
		return "", fmt.Errorf("failed to convert base IMSI to int: %v", err)
	}

	newIMSI := intBaseImsi + increment

	return fmt.Sprintf("%015d", newIMSI), nil
}

func buildSubscriberConfig(numSubscribers int, startIMSI string) ([]core.SubscriberConfig, error) {
	subs := []core.SubscriberConfig{}

	for i := range numSubscribers {
		imsi, err := computeIMSI(startIMSI, i)
		if err != nil {
			return nil, fmt.Errorf("failed to compute IMSI: %v", err)
		}

		subs = append(subs, core.SubscriberConfig{
			Imsi:           imsi,
			Key:            DefaultKey,
			SequenceNumber: DefaultSequenceNumber,
			OPc:            DefaultOPC,
			PolicyName:     DefaultPolicyName,
		})
	}

	return subs, nil
}

func (t RegistrationSuccess50Sequential) Run(ctx context.Context, env engine.Env) error {
	subs, err := buildSubscriberConfig(NumSubscribersSequential, testStartIMSI)
	if err != nil {
		return fmt.Errorf("could not build subscriber config: %v", err)
	}

	ellaCoreEnv := core.NewEllaCoreEnv(env.EllaCoreClient, core.EllaCoreConfig{
		Operator: core.OperatorConfig{
			ID: core.OperatorID{
				MCC: DefaultMCC,
				MNC: DefaultMNC,
			},
			Slice: core.OperatorSlice{
				SST: DefaultSST,
				SD:  DefaultSD,
			},
			Tracking: core.OperatorTracking{
				SupportedTACs: []string{DefaultTAC},
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
				Name:            DefaultPolicyName,
				BitrateUplink:   "100 Mbps",
				BitrateDownlink: "100 Mbps",
				Var5qi:          9,
				Arp:             15,
				DataNetworkName: DefaultDNN,
			},
		},
		Subscribers: subs,
	})

	err = ellaCoreEnv.Create(ctx)
	if err != nil {
		return fmt.Errorf("could not create EllaCore environment: %v", err)
	}

	logger.Logger.Debug("Created EllaCore environment")

	gNodeB, err := gnb.Start(
		GNBID,
		DefaultMCC,
		DefaultMNC,
		DefaultSST,
		DefaultSD,
		DefaultDNN,
		DefaultTAC,
		"Ella-Core-Tester",
		env.Config.EllaCore.N2Address,
		env.Config.Gnb.N2Address,
		env.Config.Gnb.N3Address,
	)
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

	for i := range NumSubscribersSequential {
		ranUENGAPID := RANUENGAPID + int64(i)

		exp := &validate.ExpectedPDUSessionEstablishmentAccept{
			PDUSessionID:               PDUSessionID,
			PDUSessionType:             PDUSessionType,
			UeIPSubnet:                 network,
			Dnn:                        DefaultDNN,
			Sst:                        DefaultSST,
			Sd:                         DefaultSD,
			MaximumBitRateUplinkMbps:   100,
			MaximumBitRateDownlinkMbps: 100,
			Qfi:                        1,
			FiveQI:                     9,
		}

		err := ueRegistrationTest(ranUENGAPID, gNodeB, subs[i], DefaultDNN, exp)
		if err != nil {
			return fmt.Errorf("UE registration test failed for subscriber %d: %v", i, err)
		}
	}

	err = ellaCoreEnv.Delete(ctx)
	if err != nil {
		return fmt.Errorf("could not delete EllaCore environment: %v", err)
	}

	logger.Logger.Debug("Deleted EllaCore environment")

	return nil
}

func ueRegistrationTest(ranUENGAPID int64, gNodeB *gnb.GnodeB, subscriber core.SubscriberConfig, dnn string, expectedPDUSessionAccept *validate.ExpectedPDUSessionEstablishmentAccept) error {
	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:         gNodeB,
		PDUSessionID:   PDUSessionID,
		PDUSessionType: PDUSessionType,
		Msin:           subscriber.Imsi[5:],
		K:              subscriber.Key,
		OpC:            subscriber.OPc,
		Amf:            "80000000000000000000000000000000",
		Sqn:            DefaultSequenceNumber,
		Mcc:            DefaultMCC,
		Mnc:            DefaultMNC,
		HomeNetworkPublicKey: sidf.HomeNetworkPublicKey{
			ProtectionScheme: sidf.NullScheme,
			PublicKeyID:      "0",
		},
		RoutingIndicator: "0000",
		DNN:              dnn,
		Sst:              DefaultSST,
		Sd:               DefaultSD,
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
		return fmt.Errorf("could not create UE: %v", err)
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	pduSessAcceptMsg, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID: ranUENGAPID,
		UE:          newUE,
	})
	if err != nil {
		return fmt.Errorf("initial registration procedure failed for subscriber %v: %v", newUE.UeSecurity.Msin, err)
	}

	err = validate.PDUSessionEstablishmentAccept(pduSessAcceptMsg, expectedPDUSessionAccept)
	if err != nil {
		return fmt.Errorf("PDUSessionResourceSetupRequest validation failed: %v", err)
	}

	// Cleanup
	err = procedure.Deregistration(&procedure.DeregistrationOpts{
		UE:          newUE,
		AMFUENGAPID: gNodeB.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID: ranUENGAPID,
	})
	if err != nil {
		return fmt.Errorf("DeregistrationProcedure failed: %v", err)
	}

	return nil
}
