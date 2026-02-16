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
			PolicyName:     "policy0",
		},
		{
			Imsi:           "001017271246547",
			Key:            DefaultKey,
			SequenceNumber: DefaultSequenceNumber,
			OPc:            DefaultOPC,
			PolicyName:     "policy1",
		},
		{
			Imsi:           "001017271246548",
			Key:            DefaultKey,
			SequenceNumber: DefaultSequenceNumber,
			OPc:            DefaultOPC,
			PolicyName:     "policy2",
		},
		{
			Imsi:           "001017271246549",
			Key:            DefaultKey,
			SequenceNumber: DefaultSequenceNumber,
			OPc:            DefaultOPC,
			PolicyName:     "policy3",
		},
		{
			Imsi:           "001017271246550",
			Key:            DefaultKey,
			SequenceNumber: DefaultSequenceNumber,
			OPc:            DefaultOPC,
			PolicyName:     "policy4",
		},
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
				Name:   "dnn0",
				IPPool: "10.45.0.0/16",
				DNS:    "8.8.8.8",
				Mtu:    1500,
			},
			{
				Name:   "dnn1",
				IPPool: "10.46.0.0/16",
				DNS:    "8.8.4.4",
				Mtu:    1500,
			},
			{
				Name:   "dnn2",
				IPPool: "10.47.0.0/16",
				DNS:    "8.8.2.2",
				Mtu:    1500,
			},
			{
				Name:   "dnn3",
				IPPool: "10.48.0.0/16",
				DNS:    "8.8.1.1",
				Mtu:    1500,
			},
			{
				Name:   "dnn4",
				IPPool: "10.49.0.0/16",
				DNS:    "8.8.0.0",
				Mtu:    1500,
			},
		},
		Policies: []core.PolicyConfig{
			{
				Name:            "policy0",
				BitrateUplink:   "10 Mbps",
				BitrateDownlink: "50 Mbps",
				Var5qi:          5,
				Arp:             15,
				DataNetworkName: "dnn0",
			},
			{
				Name:            "policy1",
				BitrateUplink:   "20 Mbps",
				BitrateDownlink: "100 Mbps",
				Var5qi:          6,
				Arp:             15,
				DataNetworkName: "dnn1",
			},
			{
				Name:            "policy2",
				BitrateUplink:   "30 Mbps",
				BitrateDownlink: "150 Mbps",
				Var5qi:          7,
				Arp:             15,
				DataNetworkName: "dnn2",
			},
			{
				Name:            "policy3",
				BitrateUplink:   "40 Mbps",
				BitrateDownlink: "200 Mbps",
				Var5qi:          8,
				Arp:             15,
				DataNetworkName: "dnn3",
			},
			{
				Name:            "policy4",
				BitrateUplink:   "50 Mbps",
				BitrateDownlink: "250 Mbps",
				Var5qi:          9,
				Arp:             15,
				DataNetworkName: "dnn4",
			},
		},
		Subscribers: subs,
	})

	err := ellaCoreEnv.Create(ctx)
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
		"dnn0",
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

	eg := errgroup.Group{}

	for i := range 5 {
		func() {
			eg.Go(func() error {
				ranUENGAPID := RANUENGAPID + int64(i)

				network, err := netip.ParsePrefix(fmt.Sprintf("10.4%d.0.0/16", 5+i))
				if err != nil {
					return fmt.Errorf("failed to parse UE IP subnet: %v", err)
				}

				exp := &validate.ExpectedPDUSessionEstablishmentAccept{
					PDUSessionID:               PDUSessionID,
					PDUSessionType:             PDUSessionType,
					UeIPSubnet:                 network,
					Dnn:                        fmt.Sprintf("dnn%d", i),
					Sst:                        DefaultSST,
					Sd:                         DefaultSD,
					MaximumBitRateUplinkMbps:   10 * uint64(i+1),
					MaximumBitRateDownlinkMbps: 50 * uint64(i+1),
					Qfi:                        1,
					FiveQI:                     5 + uint8(i),
				}

				return ueRegistrationTest(ranUENGAPID, gNodeB, subs[i], fmt.Sprintf("dnn%d", i), exp)
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
