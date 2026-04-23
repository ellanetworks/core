package enb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/enb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/tests/engine"
	"github.com/ellanetworks/core/internal/tester/tests/tests/utils"
	"github.com/ellanetworks/core/internal/tester/tests/tests/utils/core"
	"github.com/ellanetworks/core/internal/tester/tests/tests/utils/procedure"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"golang.org/x/sync/errgroup"
)

const (
	NumMultiUERegistration = 8
)

type MultiUERegistration struct{}

func (MultiUERegistration) Meta() engine.Meta {
	return engine.Meta{
		ID:      "enb/multi_ue_registration",
		Summary: "ng-eNB multi-UE registration test with 8 UEs registering concurrently",
		Timeout: 30 * time.Second,
	}
}

func (t MultiUERegistration) Run(ctx context.Context, env engine.Env) error {
	subs, err := buildSubscriberConfig(NumMultiUERegistration, testStartIMSI)
	if err != nil {
		return fmt.Errorf("could not build subscriber config: %v", err)
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
		},
		Slices: []core.SliceConfig{
			{
				Name: DefaultSliceName,
				SST:  DefaultSST,
				SD:   DefaultSD,
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
				SliceName:           DefaultSliceName,
				SessionAmbrUplink:   DefaultPolicySessionAmbrUplink,
				SessionAmbrDownlink: DefaultPolicySessionAmbrDownlink,
				Var5qi:              9,
				Arp:                 15,
				DataNetworkName:     DefaultDNN,
			},
		},
		Subscribers: subs,
	})

	err = ellaCoreEnv.Create(ctx)
	if err != nil {
		return fmt.Errorf("could not create EllaCore environment: %v", err)
	}

	logger.Logger.Debug("Created EllaCore environment")

	ngeNB, err := enb.Start(
		DefaultEnbID,
		DefaultMCC,
		DefaultMNC,
		DefaultSST,
		DefaultSD,
		DefaultDNN,
		DefaultTAC,
		"Ella-Core-Tester-ENB",
		env.Config.EllaCore.N2Address,
		env.Config.Gnb.N2Address,
		env.Config.Gnb.N3Address,
	)
	if err != nil {
		return fmt.Errorf("error starting eNB: %v", err)
	}

	defer ngeNB.Close()

	_, err = ngeNB.WaitForMessage(ngapType.NGAPPDUPresentSuccessfulOutcome, ngapType.SuccessfulOutcomePresentNGSetupResponse, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive SCTP frame: %v", err)
	}

	eg := errgroup.Group{}

	for i := range NumMultiUERegistration {
		func() {
			eg.Go(func() error {
				ranUENGAPID := DefaultRanUENGAPID + int64(i)

				return runMultiUERegistrationTest(ranUENGAPID, ngeNB, subs[i])
			})
		}()
	}

	err = eg.Wait()
	if err != nil {
		return fmt.Errorf("error during UE registrations: %v", err)
	}

	err = ellaCoreEnv.Delete(ctx)
	if err != nil {
		return fmt.Errorf("could not delete EllaCore environment: %v", err)
	}

	logger.Logger.Debug("Deleted EllaCore environment")

	return nil
}

func runMultiUERegistrationTest(
	ranUENGAPID int64,
	ngeNB *enb.NgeNB,
	subscriber core.SubscriberConfig,
) error {
	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:         ngeNB.GnodeB,
		PDUSessionID:   DefaultPDUSessionID,
		PDUSessionType: nasMessage.PDUSessionTypeIPv4,
		Msin:           subscriber.Imsi[5:],
		K:              subscriber.Key,
		OpC:            subscriber.OPc,
		Amf:            "80000000000000000000000000000000",
		Sqn:            subscriber.SequenceNumber,
		Mcc:            DefaultMCC,
		Mnc:            DefaultMNC,
		HomeNetworkPublicKey: sidf.HomeNetworkPublicKey{
			ProtectionScheme: sidf.NullScheme,
			PublicKeyID:      "0",
		},
		RoutingIndicator: "0000",
		DNN:              DefaultDNN,
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

	ngeNB.AddUE(ranUENGAPID, newUE)

	_, err = procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: DefaultPDUSessionID,
		UE:           newUE,
	})
	if err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	err = procedure.Deregistration(&procedure.DeregistrationOpts{
		UE:          newUE,
		AMFUENGAPID: ngeNB.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID: ranUENGAPID,
	})
	if err != nil {
		return fmt.Errorf("deregistration procedure failed: %v", err)
	}

	return nil
}
