// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/testutil/validate"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/connectivity_multi_pdu_session",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runConnectivityMultiPDUSession(ctx, env, params)
		},
		Fixture: fixtureConnectivityMultiPDUSession,
	})
}

func fixtureConnectivityMultiPDUSession(env scenarios.Env) scenarios.FixtureSpec {
	// Validates UE-AMBR at 500 Mbps, distinct from the baseline default profile (100 Mbps).
	enterpriseIPPool := "10.46.0.0/16"
	enterpriseDNS := "8.8.4.4"

	if env.IPFamily() == scenarios.IPv6Only {
		enterpriseIPPool = "fd46::/48"
		enterpriseDNS = scenarios.DefaultDNSv6
	}

	return scenarios.FixtureSpec{
		Profiles: []scenarios.ProfileSpec{
			{Name: "multi-pdu-profile", UeAmbrUplink: "500 Mbps", UeAmbrDownlink: "500 Mbps"},
		},
		Slices: []scenarios.SliceSpec{
			{Name: "enterprise-slice", SST: 1, SD: "204060"},
		},
		DataNetworks: []scenarios.DataNetworkSpec{
			{Name: "enterprise", IPv4Pool: enterpriseIPPool, DNS: enterpriseDNS, MTU: scenarios.DefaultMTU},
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
		dnn1 = scenarios.DefaultDNN
		dnn2 = "enterprise"

		slice2SST = int32(1)
		slice2SD  = "204060"

		pduSessionID1 uint8 = 1
		pduSessionID2 uint8 = 2
	)

	ipFamily := env.IPFamily()

	var ipPool1, ipPool2 string
	if ipFamily == scenarios.IPv6Only {
		ipPool1 = "fd45::/48"
		ipPool2 = "fd46::/48"
	} else {
		ipPool1 = "10.45.0.0/16"
		ipPool2 = "10.46.0.0/16"
	}

	pingDest := env.PingDestination()
	pingCmd := env.PingCommand()
	pduSessionType := env.PDUSessionType()

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
		GnbID:           scenarios.DefaultGNBID,
		MCC:             scenarios.DefaultMCC,
		MNC:             scenarios.DefaultMNC,
		SST:             scenarios.DefaultSST,
		SD:              scenarios.DefaultSD,
		DNN:             dnn1,
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

	_, err = gNodeB.WaitForMessage(gnb.Successful, ngap.ProcNGSetup, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive NG Setup Response: %v", err)
	}

	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:         gNodeB,
		PDUSessionID:   pduSessionID1,
		PDUSessionType: fgs.PDUSessionType(pduSessionType),
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

	registration, err := gNodeB.Register(newUE, ranUENGAPID, pduSessionID1, registrationTimeout)
	if err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	session1 := registration.Session

	err = validate.PDUSessionEstablishmentAccept(session1.Accept, &validate.ExpectedPDUSessionEstablishmentAccept{
		PDUSessionID:               fgs.PDUSessionID(pduSessionID1),
		PDUSessionType:             fgs.PDUSessionType(pduSessionType),
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

	slice2Snssai := models.Snssai{Sst: slice2SST, Sd: slice2SD}

	session2, err := gNodeB.EstablishPDUSession(newUE, ranUENGAPID, pduSessionID2, dnn2, slice2Snssai, registrationTimeout)
	if err != nil {
		return fmt.Errorf("could not establish PDU session 2: %v", err)
	}

	err = validate.PDUSessionEstablishmentAccept(session2.Accept, &validate.ExpectedPDUSessionEstablishmentAccept{
		PDUSessionID:               fgs.PDUSessionID(pduSessionID2),
		PDUSessionType:             fgs.PDUSessionType(pduSessionType),
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

	logger.Logger.Debug(
		"Established PDU session 2",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.String("DNN", dnn2),
		zap.Uint8("PDU Session ID", pduSessionID2),
	)

	err = validate.PDUSessionInformation(session1, &validate.ExpectedPDUSessionInformation{
		FiveQI: 9,
		ARP:    15,
		QFI:    1,
	})
	if err != nil {
		return fmt.Errorf("NGAP QoS validation failed for PDU session 1: %v", err)
	}

	err = validate.PDUSessionInformation(*session2, &validate.ExpectedPDUSessionInformation{
		FiveQI: 7,
		ARP:    15,
		QFI:    1,
	})
	if err != nil {
		return fmt.Errorf("NGAP QoS validation failed for PDU session 2: %v", err)
	}

	tun1 := gtpInterfaceNamePrefix + "mp0"
	tun2 := gtpInterfaceNamePrefix + "mp1"

	var ueIP1, ueIP2 string
	if ipFamily == scenarios.IPv6Only {
		ueIP1 = session1.UEIPv6 + env.UIPrefix()
		ueIP2 = session2.UEIPv6 + env.UIPrefix()
	} else {
		ueIP1 = session1.UEIPv4 + env.UIPrefix()
		ueIP2 = session2.UEIPv4 + env.UIPrefix()
	}

	err = gNodeB.AddTunnel(&gnb.TunnelOpts{
		UEIPv4:           ueIP1,
		UpfAddress:       session1.UpfAddress,
		TunInterfaceName: tun1,
		ULTEID:           session1.ULTEID,
		DLTEID:           session1.DLTEID,
		MTU:              session1.MTU,
		QFI:              session1.QFI,
	})
	if err != nil {
		return fmt.Errorf("could not create GTP tunnel for session 1: %v", err)
	}

	logger.GnbLogger.Debug("Created GTP tunnel for PDU session 1",
		zap.String("interface", tun1),
		zap.String("UE IP", ueIP1),
	)

	err = gNodeB.AddTunnel(&gnb.TunnelOpts{
		UEIPv4:           ueIP2,
		UpfAddress:       session2.UpfAddress,
		TunInterfaceName: tun2,
		ULTEID:           session2.ULTEID,
		DLTEID:           session2.DLTEID,
		MTU:              session2.MTU,
		QFI:              session2.QFI,
	})
	if err != nil {
		return fmt.Errorf("could not create GTP tunnel for session 2: %v", err)
	}

	logger.GnbLogger.Debug("Created GTP tunnel for PDU session 2",
		zap.String("interface", tun2),
		zap.String("UE IP", ueIP2),
	)

	if err := probe.Run(ctx, probe.ICMP, tun1, pingDest, scenarios.DefaultProbePort, pingCmd == "ping6"); err != nil {
		return fmt.Errorf("ping via %s (DNN %s, session 1) failed: %w", tun1, dnn1, err)
	}

	logger.Logger.Debug("Ping successful on PDU session 1",
		zap.String("DNN", dnn1),
		zap.String("interface", tun1),
		zap.String("destination", pingDest),
	)

	if err := probe.Run(ctx, probe.ICMP, tun2, pingDest, scenarios.DefaultProbePort, pingCmd == "ping6"); err != nil {
		return fmt.Errorf("ping via %s (DNN %s, session 2) failed: %w", tun2, dnn2, err)
	}

	logger.Logger.Debug("Ping successful on PDU session 2",
		zap.String("DNN", dnn2),
		zap.String("interface", tun2),
		zap.String("destination", pingDest),
	)

	gNodeB.CloseTunnel(session1.DLTEID)

	gNodeB.CloseTunnel(session2.DLTEID)

	err = gNodeB.Deregister(newUE, ranUENGAPID, releaseTimeout)
	if err != nil {
		return fmt.Errorf("deregistration failed: %v", err)
	}

	logger.Logger.Debug("Deregistered UE after multi-PDU-session test")

	return nil
}
