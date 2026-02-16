package ue

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/core-tester/internal/gnb"
	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/ellanetworks/core/core-tester/internal/tests/engine"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils/core"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils/validate"
	"github.com/free5gc/ngap/ngapType"
	"golang.org/x/sync/errgroup"
)

const (
	NumSubscribersParallel = 150
)

type RegistrationSuccess150Parallel struct{}

func (RegistrationSuccess150Parallel) Meta() engine.Meta {
	return engine.Meta{
		ID:      "ue/registration_success_150_parallel",
		Summary: "UE parallel registration success test validating the Registration Request and Authentication procedures with 150 UEs",
		Timeout: 60 * time.Second,
	}
}

func (t RegistrationSuccess150Parallel) Run(ctx context.Context, env engine.Env) error {
	subs, err := buildSubscriberConfig(NumSubscribersParallel, testStartIMSI)
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

	eg := errgroup.Group{}

	for i := range NumSubscribersParallel {
		func() {
			eg.Go(func() error {
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

				return ueRegistrationTest(ranUENGAPID, gNodeB, subs[i], DefaultDNN, exp)
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
