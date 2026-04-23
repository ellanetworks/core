package ue

import (
	"context"
	"fmt"
	"net/netip"
	"os/exec"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/ellanetworks/core/internal/tester/testutil/validate"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/free5gc/nas"
	"github.com/free5gc/ngap/ngapType"
	"github.com/free5gc/openapi/models"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "ue/connectivity_multi_pdu_session",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runConnectivityMultiPDUSession(ctx, env, params)
		},
		Fixture: fixtureConnectivityMultiPDUSession,
	})
}

func fixtureConnectivityMultiPDUSession() scenarios.FixtureSpec {
	// Scenario validates UE-AMBR at 500 Mbps, distinct from the baseline
	// default profile (100 Mbps). The fixture declares its own profile
	// "multi-pdu-profile" (500 Mbps) and two policies pinning that profile
	// to (default slice, default DN) for PDU session 1 and to
	// (enterprise slice, enterprise DN) for PDU session 2.
	return scenarios.FixtureSpec{
		Profiles: []scenarios.ProfileSpec{
			{Name: "multi-pdu-profile", UeAmbrUplink: "500 Mbps", UeAmbrDownlink: "500 Mbps"},
		},
		Slices: []scenarios.SliceSpec{
			{Name: "enterprise-slice", SST: 1, SD: "204060"},
		},
		DataNetworks: []scenarios.DataNetworkSpec{
			{Name: "enterprise", IPPool: "10.46.0.0/16", DNS: "8.8.4.4", MTU: scenarios.DefaultMTU},
		},
		Policies: []scenarios.PolicySpec{
			{
				Name:                "multi-pdu-default",
				ProfileName:         "multi-pdu-profile",
				SliceName:           scenarios.DefaultSliceName,
				DataNetworkName:     scenarios.DefaultDNN,
				SessionAmbrUplink:   "100 Mbps",
				SessionAmbrDownlink: "100 Mbps",
				Var5qi:              9,
				Arp:                 15,
			},
			{
				Name:                "multi-pdu-enterprise",
				ProfileName:         "multi-pdu-profile",
				SliceName:           "enterprise-slice",
				DataNetworkName:     "enterprise",
				SessionAmbrUplink:   "30 Mbps",
				SessionAmbrDownlink: "60 Mbps",
				Var5qi:              7,
				Arp:                 15,
			},
		},
		Subscribers: []scenarios.SubscriberSpec{
			scenarios.DefaultSubscriberWith("001017271246546", "multi-pdu-profile"),
		},
		AssertUsageForIMSIs: []string{"001017271246546"},
	}
}

func runConnectivityMultiPDUSession(ctx context.Context, env scenarios.Env, _ any) error {
	const (
		dnn1    = scenarios.DefaultDNN
		dnn2    = "enterprise"
		ipPool1 = "10.45.0.0/16"
		ipPool2 = "10.46.0.0/16"

		slice2SST = int32(1)
		slice2SD  = "204060"

		pduSessionID1 uint8 = 1
		pduSessionID2 uint8 = 2
	)

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	sub := subscriber{
		IMSI:           "001017271246546",
		Key:            scenarios.DefaultKey,
		SequenceNumber: scenarios.DefaultSequenceNumber,
		OPc:            scenarios.DefaultOPC,
		ProfileName:    scenarios.DefaultProfileName,
	}

	g := env.FirstGNB()

	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:         scenarios.DefaultGNBID,
		MCC:           scenarios.DefaultMCC,
		MNC:           scenarios.DefaultMNC,
		SST:           scenarios.DefaultSST,
		SD:            scenarios.DefaultSD,
		DNN:           dnn1,
		TAC:           scenarios.DefaultTAC,
		Name:          "Ella-Core-Tester",
		CoreN2Address: env.FirstCore(),
		GnbN2Address:  g.N2Address,
		GnbN3Address:  g.N3Address,
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

	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:         gNodeB,
		PDUSessionID:   pduSessionID1,
		PDUSessionType: PDUSessionType,
		Msin:           sub.IMSI[5:],
		K:              sub.Key,
		OpC:            sub.OPc,
		Amf:            scenarios.DefaultAMF,
		Sqn:            sub.SequenceNumber,
		Mcc:            scenarios.DefaultMCC,
		Mnc:            scenarios.DefaultMNC,
		HomeNetworkPublicKey: sidf.HomeNetworkPublicKey{
			ProtectionScheme: sidf.NullScheme,
			PublicKeyID:      "0",
		},
		RoutingIndicator: scenarios.DefaultRoutingIndicator,
		DNN:              dnn1,
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

	gNodeB.AddUE(ranUENGAPID, newUE)

	network1, err := netip.ParsePrefix(ipPool1)
	if err != nil {
		return fmt.Errorf("failed to parse IP pool 1: %v", err)
	}

	network2, err := netip.ParsePrefix(ipPool2)
	if err != nil {
		return fmt.Errorf("failed to parse IP pool 2: %v", err)
	}

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
		Sst:                        scenarios.DefaultSST,
		Sd:                         scenarios.DefaultSD,
		MaximumBitRateUplinkMbps:   100,
		MaximumBitRateDownlinkMbps: 100,
		Qfi:                        1,
		FiveQI:                     9,
	})
	if err != nil {
		return fmt.Errorf("PDU session 1 NAS validation failed: %v", err)
	}

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

	time.Sleep(50 * time.Millisecond)

	logger.Logger.Debug(
		"Established PDU session 2",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.String("DNN", dnn2),
		zap.Uint8("PDU Session ID", pduSessionID2),
	)

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

	tun1 := gtpInterfaceNamePrefix + "mp0"
	tun2 := gtpInterfaceNamePrefix + "mp1"

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

	cmd := exec.CommandContext(ctx, "ping", "-I", tun1, scenarios.DefaultPingDestination, "-c", "3", "-W", "1") // #nosec G204

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ping via %s (DNN %s, session 1) failed: %v\noutput:\n%s", tun1, dnn1, err, string(out))
	}

	logger.Logger.Debug("Ping successful on PDU session 1",
		zap.String("DNN", dnn1),
		zap.String("interface", tun1),
		zap.String("destination", scenarios.DefaultPingDestination),
	)

	cmd = exec.CommandContext(ctx, "ping", "-I", tun2, scenarios.DefaultPingDestination, "-c", "3", "-W", "1") // #nosec G204

	out, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ping via %s (DNN %s, session 2) failed: %v\noutput:\n%s", tun2, dnn2, err, string(out))
	}

	logger.Logger.Debug("Ping successful on PDU session 2",
		zap.String("DNN", dnn2),
		zap.String("interface", tun2),
		zap.String("destination", scenarios.DefaultPingDestination),
	)

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
