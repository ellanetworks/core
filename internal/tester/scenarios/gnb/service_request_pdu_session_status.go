// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"github.com/spf13/pflag"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/service_request_pdu_session_status",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runServiceRequestPDUSessionStatus(ctx, env, params)
		},
		Fixture: fixtureConnectivityMultiPDUSession,
	})
}

func runServiceRequestPDUSessionStatus(ctx context.Context, env scenarios.Env, _ any) error {
	const (
		dnn1 = scenarios.DefaultDNN
		dnn2 = "enterprise"

		slice2SST = int32(1)
		slice2SD  = "204060"

		keptPDUSessionID    uint8 = 1
		droppedPDUSessionID uint8 = 2
	)

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)
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
		return fmt.Errorf("error starting gNB: %w", err)
	}

	defer gNodeB.Close()

	if _, err := gNodeB.WaitForMessage(gnb.Successful, ngap.ProcNGSetup, scenarios.NGSetupTimeout); err != nil {
		return fmt.Errorf("did not receive NG Setup Response: %w", err)
	}

	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:         gNodeB,
		PDUSessionID:   keptPDUSessionID,
		PDUSessionType: fgs.PDUSessionType(env.PDUSessionType()),
		Msin:           "017271246546",
		K:              scenarios.DefaultKey,
		OpC:            scenarios.DefaultOPC,
		Amf:            scenarios.DefaultAMF,
		Sqn:            scenarios.DefaultSequenceNumber,
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
			Integrity: testutil.IntegrityAlgorithms{Nia2: true},
			Ciphering: testutil.CipheringAlgorithms{Nea0: true, Nea2: true},
		}),
	})
	if err != nil {
		return fmt.Errorf("could not create UE: %w", err)
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	if _, err := gNodeB.Register(newUE, ranUENGAPID, keptPDUSessionID, registrationTimeout); err != nil {
		return fmt.Errorf("initial registration procedure failed: %w", err)
	}

	slice2Snssai := models.Snssai{Sst: slice2SST, Sd: slice2SD}
	if _, err := gNodeB.EstablishPDUSession(newUE, ranUENGAPID, droppedPDUSessionID, dnn2, slice2Snssai, registrationTimeout); err != nil {
		return fmt.Errorf("could not establish PDU session %d: %w", droppedPDUSessionID, err)
	}

	sessions := []uint8{keptPDUSessionID, droppedPDUSessionID}
	if err := gNodeB.ReleaseContext(newUE, ranUENGAPID, sessions, gnb.CauseUserInactivity, releaseTimeout); err != nil {
		return fmt.Errorf("release the connection before the service request: %w", err)
	}

	sr, err := gNodeB.ServiceRequest(newUE, ranUENGAPID, keptPDUSessionID, registrationTimeout)
	if err != nil {
		return fmt.Errorf("service request reporting PDU session %d inactive: %w", droppedPDUSessionID, err)
	}

	if err := assertReportedPDUSessionStatus(sr.PDUSessionStatus, keptPDUSessionID, droppedPDUSessionID); err != nil {
		return err
	}

	tun := gtpInterfaceNamePrefix + "srs0"

	ueIP := sr.Session.UEIPv4 + env.UIPrefix()
	if env.IPFamily() == scenarios.IPv6Only {
		ueIP = sr.Session.UEIPv6 + env.UIPrefix()
	}

	err = gNodeB.AddTunnel(&gnb.TunnelOpts{
		UEIPv4:           ueIP,
		UpfAddress:       sr.Session.UpfAddress,
		TunInterfaceName: tun,
		ULTEID:           sr.Session.ULTEID,
		DLTEID:           sr.Session.DLTEID,
		MTU:              sr.Session.MTU,
		QFI:              sr.Session.QFI,
	})
	if err != nil {
		return fmt.Errorf("could not create GTP tunnel for the surviving session: %w", err)
	}

	defer gNodeB.CloseTunnel(sr.Session.DLTEID)

	if err := probe.Run(ctx, probe.ICMP, tun, env.PingDestination(), scenarios.DefaultProbePort, env.PingCommand() == "ping6"); err != nil {
		return fmt.Errorf("ping via %s (DNN %s) failed after the service request: %w", tun, dnn1, err)
	}

	return gNodeB.Deregister(newUE, ranUENGAPID, releaseTimeout)
}

func assertReportedPDUSessionStatus(status *fgs.PSIBitmap, kept, dropped uint8) error {
	if status == nil {
		return fmt.Errorf("the SERVICE ACCEPT carried no PDU session status, so the UE cannot tell which session the network kept (TS 24.501 §5.6.1.4.1)")
	}

	if !status.PSI[kept] {
		return fmt.Errorf("PDU session status reports session %d inactive, want it active: the UE asked for its user plane back", kept)
	}

	if status.PSI[dropped] {
		return fmt.Errorf("PDU session status reports session %d active, want it inactive: the UE reported it inactive and the AMF must locally release it (TS 24.501 §5.6.1.4.1)", dropped)
	}

	return nil
}
