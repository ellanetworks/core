package ue

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/tests/engine"
	"github.com/ellanetworks/core/internal/tester/tests/tests/utils/core"
	"github.com/ellanetworks/core/internal/tester/tests/tests/utils/validate"
	"github.com/free5gc/ngap/ngapType"
	"golang.org/x/sync/errgroup"
)

type RegistrationSuccessMultiplePoliciesPerProfile struct{}

func (RegistrationSuccessMultiplePoliciesPerProfile) Meta() engine.Meta {
	return engine.Meta{
		ID:      "ue/registration_success_multiple_policies_per_profile",
		Summary: "UE registration with multiple policies on the same profile, each targeting a different data network",
		Timeout: 60 * time.Second,
	}
}

func (t RegistrationSuccessMultiplePoliciesPerProfile) Run(ctx context.Context, env engine.Env) error {
	const (
		dnn2       = "enterprise"
		ipPool1    = "10.45.0.0/16"
		ipPool2    = "10.46.0.0/16"
		policyName = "enterprise"
	)

	subs := []core.SubscriberConfig{
		{
			Imsi:           "001017271246546",
			Key:            DefaultKey,
			SequenceNumber: DefaultSequenceNumber,
			OPc:            DefaultOPC,
			ProfileName:    DefaultProfileName,
		},
		{
			Imsi:           "001017271246547",
			Key:            DefaultKey,
			SequenceNumber: DefaultSequenceNumber,
			OPc:            DefaultOPC,
			ProfileName:    DefaultProfileName,
		},
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
				IPPool: ipPool1,
				DNS:    "8.8.8.8",
				Mtu:    1500,
			},
			{
				Name:   dnn2,
				IPPool: ipPool2,
				DNS:    "8.8.4.4",
				Mtu:    1500,
			},
		},
		Policies: []core.PolicyConfig{
			{
				Name:                DefaultPolicyName,
				ProfileName:         DefaultProfileName,
				SliceName:           DefaultSliceName,
				SessionAmbrUplink:   "100 Mbps",
				SessionAmbrDownlink: "100 Mbps",
				Var5qi:              9,
				Arp:                 15,
				DataNetworkName:     DefaultDNN,
			},
			{
				Name:                policyName,
				ProfileName:         DefaultProfileName,
				SliceName:           DefaultSliceName,
				SessionAmbrUplink:   "30 Mbps",
				SessionAmbrDownlink: "60 Mbps",
				Var5qi:              7,
				Arp:                 15,
				DataNetworkName:     dnn2,
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
		SST:           DefaultSST,
		SD:            DefaultSD,
		DNN:           DefaultDNN,
		TAC:           DefaultTAC,
		Name:          "Ella-Core-Tester",
		CoreN2Address: env.Config.EllaCore.N2Address,
		GnbN2Address:  env.Config.Gnb.N2Address,
		GnbN3Address:  env.Config.Gnb.N3Address,
	})
	if err != nil {
		return fmt.Errorf("error starting gNB: %v", err)
	}

	defer gNodeB.Close()

	_, err = gNodeB.WaitForMessage(ngapType.NGAPPDUPresentSuccessfulOutcome, ngapType.SuccessfulOutcomePresentNGSetupResponse, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive SCTP frame: %v", err)
	}

	network1, err := netip.ParsePrefix(ipPool1)
	if err != nil {
		return fmt.Errorf("failed to parse UE IP subnet: %v", err)
	}

	network2, err := netip.ParsePrefix(ipPool2)
	if err != nil {
		return fmt.Errorf("failed to parse UE IP subnet: %v", err)
	}

	dnns := []string{DefaultDNN, dnn2}
	networks := []netip.Prefix{network1, network2}
	uplinkMbps := []uint64{100, 30}
	downlinkMbps := []uint64{100, 60}
	fiveQIs := []uint8{9, 7}

	eg := errgroup.Group{}

	for i := range subs {
		func() {
			eg.Go(func() error {
				ranUENGAPID := RANUENGAPID + int64(i)
				exp := &validate.ExpectedPDUSessionEstablishmentAccept{
					PDUSessionID:               PDUSessionID,
					PDUSessionType:             PDUSessionType,
					UeIPSubnet:                 networks[i],
					Dnn:                        dnns[i],
					Sst:                        DefaultSST,
					Sd:                         DefaultSD,
					MaximumBitRateUplinkMbps:   uplinkMbps[i],
					MaximumBitRateDownlinkMbps: downlinkMbps[i],
					Qfi:                        1,
					FiveQI:                     fiveQIs[i],
				}

				return ueRegistrationTest(ranUENGAPID, gNodeB, subs[i], dnns[i], exp)
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
