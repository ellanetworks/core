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

type RegistrationSuccessMultipleDataNetworks struct{}

func (RegistrationSuccessMultipleDataNetworks) Meta() engine.Meta {
	return engine.Meta{
		ID:      "ue/registration_success_multiple_data_networks",
		Summary: "UE parallel registration success test validating the Registration Request and Authentication procedures with multiple data networks",
		Timeout: 60 * time.Second,
	}
}

func (t RegistrationSuccessMultipleDataNetworks) Run(ctx context.Context, env engine.Env) error {
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
			ProfileName:    "profile1",
		},
		{
			Imsi:           "001017271246548",
			Key:            DefaultKey,
			SequenceNumber: DefaultSequenceNumber,
			OPc:            DefaultOPC,
			ProfileName:    "profile2",
		},
		{
			Imsi:           "001017271246549",
			Key:            DefaultKey,
			SequenceNumber: DefaultSequenceNumber,
			OPc:            DefaultOPC,
			ProfileName:    "profile3",
		},
		{
			Imsi:           "001017271246550",
			Key:            DefaultKey,
			SequenceNumber: DefaultSequenceNumber,
			OPc:            DefaultOPC,
			ProfileName:    "profile4",
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
			{
				Name:           "profile1",
				UeAmbrUplink:   DefaultProfileUeAmbrUplink,
				UeAmbrDownlink: DefaultProfileUeAmbrDownlink,
			},
			{
				Name:           "profile2",
				UeAmbrUplink:   DefaultProfileUeAmbrUplink,
				UeAmbrDownlink: DefaultProfileUeAmbrDownlink,
			},
			{
				Name:           "profile3",
				UeAmbrUplink:   DefaultProfileUeAmbrUplink,
				UeAmbrDownlink: DefaultProfileUeAmbrDownlink,
			},
			{
				Name:           "profile4",
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
				IPPool: "10.45.0.0/22",
				DNS:    "8.8.8.8",
				Mtu:    1500,
			},
			{
				Name:   "dnn1",
				IPPool: "10.46.0.0/22",
				DNS:    "8.8.4.4",
				Mtu:    1500,
			},
			{
				Name:   "dnn2",
				IPPool: "10.47.0.0/22",
				DNS:    "8.8.2.2",
				Mtu:    1500,
			},
			{
				Name:   "dnn3",
				IPPool: "10.48.0.0/22",
				DNS:    "8.8.1.1",
				Mtu:    1500,
			},
			{
				Name:   "dnn4",
				IPPool: "10.49.0.0/22",
				DNS:    "8.8.0.0",
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
				Name:                "policy1",
				ProfileName:         "profile1",
				SliceName:           DefaultSliceName,
				SessionAmbrUplink:   "100 Mbps",
				SessionAmbrDownlink: "100 Mbps",
				Var5qi:              9,
				Arp:                 15,
				DataNetworkName:     "dnn1",
			},
			{
				Name:                "policy2",
				ProfileName:         "profile2",
				SliceName:           DefaultSliceName,
				SessionAmbrUplink:   "100 Mbps",
				SessionAmbrDownlink: "100 Mbps",
				Var5qi:              9,
				Arp:                 15,
				DataNetworkName:     "dnn2",
			},
			{
				Name:                "policy3",
				ProfileName:         "profile3",
				SliceName:           DefaultSliceName,
				SessionAmbrUplink:   "100 Mbps",
				SessionAmbrDownlink: "100 Mbps",
				Var5qi:              9,
				Arp:                 15,
				DataNetworkName:     "dnn3",
			},
			{
				Name:                "policy4",
				ProfileName:         "profile4",
				SliceName:           DefaultSliceName,
				SessionAmbrUplink:   "100 Mbps",
				SessionAmbrDownlink: "100 Mbps",
				Var5qi:              9,
				Arp:                 15,
				DataNetworkName:     "dnn4",
			},
		},
		Subscribers: subs,
	})

	err := ellaCoreEnv.Create(ctx)
	if err != nil {
		return fmt.Errorf("could not create EllaCore environment: %v", err)
	}

	logger.Logger.Debug("Created EllaCore environment")

	dnns := []string{DefaultDNN, "dnn1", "dnn2", "dnn3", "dnn4"}

	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:         GNBID,
		MCC:           DefaultMCC,
		MNC:           DefaultMNC,
		SST:           DefaultSST,
		SD:            DefaultSD,
		DNN:           dnns[0],
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

	eg := errgroup.Group{}

	for i := range 5 {
		func() {
			eg.Go(func() error {
				ranUENGAPID := RANUENGAPID + int64(i)

				network, err := netip.ParsePrefix(fmt.Sprintf("10.%d.0.0/22", 45+i))
				if err != nil {
					return fmt.Errorf("failed to parse UE IP subnet: %v", err)
				}

				exp := &validate.ExpectedPDUSessionEstablishmentAccept{
					PDUSessionID:               PDUSessionID,
					PDUSessionType:             PDUSessionType,
					UeIPSubnet:                 network,
					Dnn:                        dnns[i],
					Sst:                        DefaultSST,
					Sd:                         DefaultSD,
					MaximumBitRateUplinkMbps:   100,
					MaximumBitRateDownlinkMbps: 100,
					Qfi:                        1,
					FiveQI:                     9,
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
