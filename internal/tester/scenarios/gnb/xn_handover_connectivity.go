// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	ngaplib "github.com/ellanetworks/core/ngap"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const (
	xnHandoverConnIMSI       = "001017271246593"
	xnHandoverSourceTun      = "ellaxnho0"
	xnHandoverTargetTun      = "ellaxnhot0"
	xnHandoverConnRANID      = int64(201)
	xnHandoverConnTargetTEID = uint32(9201)
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/xn_handover_connectivity",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runXnHandoverConnectivity,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{
					scenarios.DefaultSubscriberWith(xnHandoverConnIMSI, ""),
				},
				AssertUsageForIMSIs: []string{xnHandoverConnIMSI},
			}
		},
	})
}

func runXnHandoverConnectivity(ctx context.Context, env scenarios.Env, _ any) error {
	if len(env.GNBs) < 2 {
		return fmt.Errorf("xn_handover_connectivity requires at least 2 gNBs, got %d", len(env.GNBs))
	}

	sourceGNBSpec := env.GNBs[0]
	targetGNBSpec := env.GNBs[1]

	sourceGNB, err := gnb.Start(&gnb.StartOpts{
		GnbID:           "000001",
		MCC:             scenarios.DefaultMCC,
		MNC:             scenarios.DefaultMNC,
		SST:             scenarios.DefaultSST,
		SD:              scenarios.DefaultSD,
		DNN:             scenarios.DefaultDNN,
		TAC:             scenarios.DefaultTAC,
		Name:            "Source-gNB",
		CoreN2Addresses: env.CoreN2Addresses,
		GnbN2Address:    sourceGNBSpec.N2Address,
		GnbN3Address:    sourceGNBSpec.N3Address,
	})
	if err != nil {
		return fmt.Errorf("start source gNB: %w", err)
	}

	defer sourceGNB.Close()

	if _, err := sourceGNB.WaitForMessage(gnb.Successful, ngaplib.ProcNGSetup, 2*time.Second); err != nil {
		return fmt.Errorf("source gNB: wait NGSetupResponse: %w", err)
	}

	targetGNB, err := startXnTargetGNB(env, targetGNBSpec.N2Address, targetGNBSpec.N3Address)
	if err != nil {
		return err
	}

	defer targetGNB.Close()

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	newUE, err := newDefaultUE(sourceGNB, xnHandoverConnIMSI[5:], scenarios.DefaultKey, scenarios.DefaultOPC, scenarios.DefaultSequenceNumber, scenarios.DefaultPDUSessionTypeIPv4)
	if err != nil {
		return fmt.Errorf("create UE: %w", err)
	}

	sourceGNB.AddUE(ranUENGAPID, newUE)

	if _, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	}); err != nil {
		return fmt.Errorf("initial registration: %w", err)
	}

	sourceAmfUENGAPID := sourceGNB.GetAMFUENGAPID(ranUENGAPID)

	uePDUSession, err := newUE.WaitForPDUSession(scenarios.DefaultPDUSessionID, 5*time.Second)
	if err != nil {
		return fmt.Errorf("wait PDU session: %w", err)
	}

	gnbPDUSession, err := sourceGNB.WaitForPDUSession(ranUENGAPID, int64(scenarios.DefaultPDUSessionID), 5*time.Second)
	if err != nil {
		return fmt.Errorf("source gNB: wait PDU session: %w", err)
	}

	ueIP := newUE.GetPDUSession(scenarios.DefaultPDUSessionID).UEIP + "/16"
	qfi := newUE.GetPDUSession(scenarios.DefaultPDUSessionID).QFI

	if _, err := sourceGNB.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             ueIP,
		UpfIP:            gnbPDUSession.UpfAddress,
		TunInterfaceName: xnHandoverSourceTun,
		ULteid:           gnbPDUSession.ULTeid,
		DLteid:           gnbPDUSession.DLTeid,
		MTU:              uePDUSession.MTU,
		QFI:              qfi,
	}); err != nil {
		return fmt.Errorf("create source GTP tunnel: %w", err)
	}

	pingDest := env.PingDestination()
	if err := probe.Run(ctx, probe.ICMP, xnHandoverSourceTun, pingDest, scenarios.DefaultProbePort, false); err != nil {
		return fmt.Errorf("ping before the Xn handover failed: %w", err)
	}

	logger.Logger.Info("Ping successful before the Xn handover", zap.String("dest", pingDest))

	targetN3IP, err := netip.ParseAddr(targetGNBSpec.N3Address)
	if err != nil {
		return fmt.Errorf("parse target N3 address: %w", err)
	}

	if _, err := xnPathSwitch(targetGNB, &xnPathSwitchOpts{
		SourceAMFUENGAPID: sourceAmfUENGAPID,
		TargetRANUENGAPID: xnHandoverConnRANID,
		TargetN3IP:        targetN3IP,
		TargetDLTEID:      xnHandoverConnTargetTEID,
	}); err != nil {
		return err
	}

	_ = sourceGNB.CloseTunnel(gnbPDUSession.DLTeid)

	if _, err := targetGNB.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             ueIP,
		UpfIP:            gnbPDUSession.UpfAddress,
		TunInterfaceName: xnHandoverTargetTun,
		ULteid:           gnbPDUSession.ULTeid,
		DLteid:           xnHandoverConnTargetTEID,
		MTU:              uePDUSession.MTU,
		QFI:              qfi,
	}); err != nil {
		return fmt.Errorf("create target GTP tunnel: %w", err)
	}

	time.Sleep(500 * time.Millisecond)

	if err := probe.Run(ctx, probe.ICMP, xnHandoverTargetTun, pingDest, scenarios.DefaultProbePort, false); err != nil {
		return fmt.Errorf("ping after the Xn handover FAILED (UPF downlink not switched to the target gNB): %w", err)
	}

	logger.Logger.Info("Ping successful after the Xn handover", zap.String("dest", pingDest))

	return nil
}
