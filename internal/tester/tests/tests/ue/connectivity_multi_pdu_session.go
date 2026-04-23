package ue

import (
	"context"
	"fmt"
	"net/netip"
	"os/exec"
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
	"github.com/free5gc/ngap/ngapType"
	"github.com/free5gc/openapi/models"
	"go.uber.org/zap"
)

type ConnectivityMultiPDUSession struct{}

func (ConnectivityMultiPDUSession) Meta() engine.Meta {
	return engine.Meta{
		ID:      "ue/connectivity_multi_pdu_session",
		Summary: "Single UE establishes two PDU sessions on different DNNs and slices, validates data plane connectivity on both",
		Timeout: 60 * time.Second,
	}
}

func (t ConnectivityMultiPDUSession) Run(ctx context.Context, env engine.Env) error {
	const (
		dnn1    = DefaultDNN
		dnn2    = "enterprise"
		ipPool1 = "10.45.0.0/16"
		ipPool2 = "10.46.0.0/16"

		slice1Name = DefaultSliceName
		slice1SST  = DefaultSST
		slice1SD   = DefaultSD

		slice2Name = "enterprise-slice"
		slice2SST  = int32(1)
		slice2SD   = "204060"

		pduSessionID1 uint8 = 1
		pduSessionID2 uint8 = 2

		ranUENGAPID = int64(RANUENGAPID)
	)

	subscriber := core.SubscriberConfig{
		Imsi:           "001017271246546",
		Key:            DefaultKey,
		SequenceNumber: DefaultSequenceNumber,
		OPc:            DefaultOPC,
		ProfileName:    DefaultProfileName,
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
				Name: DefaultProfileName,
				// UE AMBR deliberately set to 500 Mbps (different from Session AMBRs of
				// 100/100 and 30/60) to verify ella-core does not confuse UE-level and
				// session-level AMBR.
				UeAmbrUplink:   "500 Mbps",
				UeAmbrDownlink: "500 Mbps",
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
				Name:   dnn1,
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
				SliceName:           slice1Name,
				SessionAmbrUplink:   "100 Mbps",
				SessionAmbrDownlink: "100 Mbps",
				Var5qi:              9,
				Arp:                 15,
				DataNetworkName:     dnn1,
			},
			{
				Name:                "enterprise",
				ProfileName:         DefaultProfileName,
				SliceName:           slice2Name,
				SessionAmbrUplink:   "30 Mbps",
				SessionAmbrDownlink: "60 Mbps",
				Var5qi:              7,
				Arp:                 15,
				DataNetworkName:     dnn2,
			},
		},
		Subscribers: []core.SubscriberConfig{subscriber},
	})

	err := ellaCoreEnv.Create(ctx)
	if err != nil {
		return fmt.Errorf("could not create EllaCore environment: %v", err)
	}

	defer func() {
		if delErr := ellaCoreEnv.Delete(ctx); delErr != nil {
			logger.Logger.Error("could not delete EllaCore environment", zap.Error(delErr))
		}
	}()

	logger.Logger.Debug("Created EllaCore environment")

	// Start gNodeB with both slices advertised
	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:         GNBID,
		MCC:           DefaultMCC,
		MNC:           DefaultMNC,
		SST:           slice1SST,
		SD:            slice1SD,
		DNN:           dnn1,
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

	// Create UE with first DNN — the initial registration establishes PDU session 1
	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:         gNodeB,
		PDUSessionID:   pduSessionID1,
		PDUSessionType: PDUSessionType,
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
		DNN:              dnn1,
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

	network1, err := netip.ParsePrefix(ipPool1)
	if err != nil {
		return fmt.Errorf("failed to parse IP pool 1: %v", err)
	}

	network2, err := netip.ParsePrefix(ipPool2)
	if err != nil {
		return fmt.Errorf("failed to parse IP pool 2: %v", err)
	}

	// Step 1: Initial Registration — establishes PDU session 1 on dnn1
	pduAccept1, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: pduSessionID1,
		UE:           newUE,
	})
	if err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	err = validate.PDUSessionEstablishmentAccept(pduAccept1, &validate.ExpectedPDUSessionEstablishmentAccept{
		PDUSessionID:               pduSessionID1,
		PDUSessionType:             PDUSessionType,
		UeIPSubnet:                 network1,
		Dnn:                        dnn1,
		Sst:                        DefaultSST,
		Sd:                         DefaultSD,
		MaximumBitRateUplinkMbps:   100,
		MaximumBitRateDownlinkMbps: 100,
		Qfi:                        1,
		FiveQI:                     9,
	})
	if err != nil {
		return fmt.Errorf("PDU session 1 NAS validation failed: %v", err)
	}

	// Validate UE-level AMBR (profile-level, distinct from session AMBR)
	ueAmbr := gNodeB.GetUEAmbr(ranUENGAPID)

	err = validate.UEAmbr(ueAmbr, &validate.ExpectedUEAmbr{
		UplinkBps:   500_000_000,
		DownlinkBps: 500_000_000,
	})
	if err != nil {
		return fmt.Errorf("UE AMBR validation failed: %v", err)
	}

	logger.Logger.Debug(
		"Completed Initial Registration (PDU session 1)",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.String("DNN", dnn1),
		zap.Uint8("PDU Session ID", pduSessionID1),
	)

	// Step 2: Establish second PDU session on dnn2 with slice2
	amfUENGAPID := gNodeB.GetAMFUENGAPID(ranUENGAPID)

	slice2Snssai := models.Snssai{Sst: slice2SST, Sd: slice2SD}

	err = newUE.SendPDUSessionEstablishmentRequest(amfUENGAPID, ranUENGAPID, pduSessionID2, dnn2, slice2Snssai)
	if err != nil {
		return fmt.Errorf("could not send PDU Session Establishment Request for session 2: %v", err)
	}

	pduAccept2, err := newUE.WaitForNASGSMMessage(nas.MsgTypePDUSessionEstablishmentAccept, 5*time.Second)
	if err != nil {
		return fmt.Errorf("did not receive PDU Session Establishment Accept for session 2: %v", err)
	}

	err = validate.PDUSessionEstablishmentAccept(pduAccept2, &validate.ExpectedPDUSessionEstablishmentAccept{
		PDUSessionID:               pduSessionID2,
		PDUSessionType:             PDUSessionType,
		UeIPSubnet:                 network2,
		Dnn:                        dnn2,
		Sst:                        slice2SST,
		Sd:                         slice2SD,
		MaximumBitRateUplinkMbps:   30,
		MaximumBitRateDownlinkMbps: 60,
		Qfi:                        1,
		FiveQI:                     7,
	})
	if err != nil {
		return fmt.Errorf("PDU session 2 NAS validation failed: %v", err)
	}

	_, err = newUE.WaitForPDUSession(pduSessionID2, 5*time.Second)
	if err != nil {
		return fmt.Errorf("timeout waiting for PDU session 2: %v", err)
	}

	// Allow gNB to process the PDU Session Resource Setup for session 2
	time.Sleep(50 * time.Millisecond)

	logger.Logger.Debug(
		"Established PDU session 2",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.String("DNN", dnn2),
		zap.Uint8("PDU Session ID", pduSessionID2),
	)

	// Step 3: Get session info and create tunnels for both sessions
	uePDU1 := newUE.GetPDUSession(pduSessionID1)
	uePDU2 := newUE.GetPDUSession(pduSessionID2)

	gnbPDU1, err := gNodeB.WaitForPDUSession(ranUENGAPID, int64(pduSessionID1), 5*time.Second)
	if err != nil {
		return fmt.Errorf("could not get gNB PDU session 1: %v", err)
	}

	err = validate.PDUSessionInformation(gnbPDU1, &validate.ExpectedPDUSessionInformation{
		FiveQi: 9,
		PriArp: 15,
		QFI:    1,
	})
	if err != nil {
		return fmt.Errorf("NGAP QoS validation failed for PDU session 1: %v", err)
	}

	gnbPDU2, err := gNodeB.WaitForPDUSession(ranUENGAPID, int64(pduSessionID2), 5*time.Second)
	if err != nil {
		return fmt.Errorf("could not get gNB PDU session 2: %v", err)
	}

	err = validate.PDUSessionInformation(gnbPDU2, &validate.ExpectedPDUSessionInformation{
		FiveQi: 7,
		PriArp: 15,
		QFI:    1,
	})
	if err != nil {
		return fmt.Errorf("NGAP QoS validation failed for PDU session 2: %v", err)
	}

	tun1 := GTPInterfaceNamePrefix + "mp0"
	tun2 := GTPInterfaceNamePrefix + "mp1"

	ueIP1 := uePDU1.UEIP + "/16"
	ueIP2 := uePDU2.UEIP + "/16"

	_, err = gNodeB.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             ueIP1,
		UpfIP:            gnbPDU1.UpfAddress,
		TunInterfaceName: tun1,
		ULteid:           gnbPDU1.ULTeid,
		DLteid:           gnbPDU1.DLTeid,
		MTU:              uePDU1.MTU,
		QFI:              uePDU1.QFI,
	})
	if err != nil {
		return fmt.Errorf("could not create GTP tunnel for session 1: %v", err)
	}

	logger.GnbLogger.Debug("Created GTP tunnel for PDU session 1",
		zap.String("interface", tun1),
		zap.String("UE IP", ueIP1),
	)

	_, err = gNodeB.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             ueIP2,
		UpfIP:            gnbPDU2.UpfAddress,
		TunInterfaceName: tun2,
		ULteid:           gnbPDU2.ULTeid,
		DLteid:           gnbPDU2.DLTeid,
		MTU:              uePDU2.MTU,
		QFI:              uePDU2.QFI,
	})
	if err != nil {
		return fmt.Errorf("could not create GTP tunnel for session 2: %v", err)
	}

	logger.GnbLogger.Debug("Created GTP tunnel for PDU session 2",
		zap.String("interface", tun2),
		zap.String("UE IP", ueIP2),
	)

	// Step 4: Ping via both tunnels
	cmd := exec.CommandContext(ctx, "ping", "-I", tun1, env.Config.PingDestination, "-c", "3", "-W", "1") // #nosec G204 -- ping is fixed; tun1 is internally derived; PingDestination is test config

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ping via %s (DNN %s, session 1) failed: %v\noutput:\n%s", tun1, dnn1, err, string(out))
	}

	logger.Logger.Debug("Ping successful on PDU session 1",
		zap.String("DNN", dnn1),
		zap.String("interface", tun1),
		zap.String("destination", env.Config.PingDestination),
	)

	cmd = exec.CommandContext(ctx, "ping", "-I", tun2, env.Config.PingDestination, "-c", "3", "-W", "1") // #nosec G204 -- ping is fixed; tun2 is internally derived; PingDestination is test config

	out, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ping via %s (DNN %s, session 2) failed: %v\noutput:\n%s", tun2, dnn2, err, string(out))
	}

	logger.Logger.Debug("Ping successful on PDU session 2",
		zap.String("DNN", dnn2),
		zap.String("interface", tun2),
		zap.String("destination", env.Config.PingDestination),
	)

	// Step 5: Cleanup tunnels and deregister
	err = gNodeB.CloseTunnel(gnbPDU1.DLTeid)
	if err != nil {
		return fmt.Errorf("could not close GTP tunnel for session 1: %v", err)
	}

	err = gNodeB.CloseTunnel(gnbPDU2.DLTeid)
	if err != nil {
		return fmt.Errorf("could not close GTP tunnel for session 2: %v", err)
	}

	err = procedure.Deregistration(&procedure.DeregistrationOpts{
		UE:          newUE,
		AMFUENGAPID: amfUENGAPID,
		RANUENGAPID: ranUENGAPID,
	})
	if err != nil {
		return fmt.Errorf("deregistration failed: %v", err)
	}

	logger.Logger.Debug("Deregistered UE after multi-PDU-session test")

	return nil
}
