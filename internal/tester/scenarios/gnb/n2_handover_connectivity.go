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
	"github.com/ellanetworks/core/internal/tester/scenarios/common"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const (
	n2HandoverConnIMSI        = "001017271246591"
	n2HandoverTunInterface    = "ellan2ho0"
	n2HandoverTargetTunPrefix = "ellan2hot0"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/n2_handover_connectivity",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runN2HandoverConnectivity,
		Fixture:   fixtureN2HandoverConnectivity,
	})
}

func fixtureN2HandoverConnectivity(_ scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{
			scenarios.DefaultSubscriberWith(n2HandoverConnIMSI, ""),
		},
	}
}

func runN2HandoverConnectivity(_ context.Context, env scenarios.Env, _ any) error {
	if len(env.GNBs) < 2 {
		return fmt.Errorf("n2_handover_connectivity requires at least 2 gNBs, got %d", len(env.GNBs))
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

	if _, err := sourceGNB.WaitForMessage(
		ngapType.NGAPPDUPresentSuccessfulOutcome,
		ngapType.SuccessfulOutcomePresentNGSetupResponse,
		2*time.Second,
	); err != nil {
		return fmt.Errorf("source gNB: wait NGSetupResponse: %w", err)
	}

	targetGNB, err := gnb.Start(&gnb.StartOpts{
		GnbID:           "000002",
		MCC:             scenarios.DefaultMCC,
		MNC:             scenarios.DefaultMNC,
		SST:             scenarios.DefaultSST,
		SD:              scenarios.DefaultSD,
		DNN:             scenarios.DefaultDNN,
		TAC:             scenarios.DefaultTAC,
		Name:            "Target-gNB",
		CoreN2Addresses: env.CoreN2Addresses,
		GnbN2Address:    targetGNBSpec.N2Address,
		GnbN3Address:    targetGNBSpec.N3Address,
	})
	if err != nil {
		return fmt.Errorf("start target gNB: %w", err)
	}

	defer targetGNB.Close()

	if _, err := targetGNB.WaitForMessage(
		ngapType.NGAPPDUPresentSuccessfulOutcome,
		ngapType.SuccessfulOutcomePresentNGSetupResponse,
		2*time.Second,
	); err != nil {
		return fmt.Errorf("target gNB: wait NGSetupResponse: %w", err)
	}

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	newUE, err := newDefaultUE(sourceGNB, n2HandoverConnIMSI[5:], scenarios.DefaultKey, scenarios.DefaultOPC, scenarios.DefaultSequenceNumber, scenarios.DefaultPDUSessionTypeIPv4)
	if err != nil {
		return fmt.Errorf("create UE: %w", err)
	}

	sourceGNB.AddUE(ranUENGAPID, newUE)

	_, err = procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	})
	if err != nil {
		return fmt.Errorf("initial registration: %w", err)
	}

	amfUENGAPID := sourceGNB.GetAMFUENGAPID(ranUENGAPID)

	uePDUSession, err := newUE.WaitForPDUSession(scenarios.DefaultPDUSessionID, 5*time.Second)
	if err != nil {
		return fmt.Errorf("wait PDU session: %w", err)
	}

	gnbPDUSession, err := sourceGNB.WaitForPDUSession(ranUENGAPID, int64(scenarios.DefaultPDUSessionID), 5*time.Second)
	if err != nil {
		return fmt.Errorf("source gNB: wait PDU session: %w", err)
	}

	ueIP := newUE.GetPDUSession(scenarios.DefaultPDUSessionID).UEIP + "/16"

	_, err = sourceGNB.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             ueIP,
		UpfIP:            gnbPDUSession.UpfAddress,
		TunInterfaceName: n2HandoverTunInterface,
		ULteid:           gnbPDUSession.ULTeid,
		DLteid:           gnbPDUSession.DLTeid,
		MTU:              uePDUSession.MTU,
		QFI:              newUE.GetPDUSession(scenarios.DefaultPDUSessionID).QFI,
	})
	if err != nil {
		return fmt.Errorf("create source GTP tunnel: %w", err)
	}

	pingDest := env.PingDestination()
	if err := probe.Run(context.Background(), probe.ICMP, n2HandoverTunInterface, pingDest, scenarios.DefaultProbePort, false); err != nil {
		return fmt.Errorf("ping before handover failed: %w", err)
	}

	logger.Logger.Info("Ping successful before handover", zap.String("dest", pingDest))

	err = sourceGNB.SendHandoverRequired(&gnb.HandoverRequiredOpts{
		AMFUENGAPID:  amfUENGAPID,
		RANUENGAPID:  ranUENGAPID,
		HandoverType: int64(ngapType.HandoverTypePresentIntra5gs),
		TargetGnbID:  "000002",
		PDUSessions: []gnb.HandoverRequiredPDUSession{
			{PDUSessionID: int64(scenarios.DefaultPDUSessionID)},
		},
	})
	if err != nil {
		return fmt.Errorf("send HandoverRequired: %w", err)
	}

	hoReqFrame, err := targetGNB.WaitForMessage(
		ngapType.NGAPPDUPresentInitiatingMessage,
		ngapType.InitiatingMessagePresentHandoverRequest,
		5*time.Second,
	)
	if err != nil {
		return fmt.Errorf("target gNB: wait HandoverRequest: %w", err)
	}

	targetAmfUENGAPID, err := common.ExtractAmfUeNgapIDFromHandoverRequest(hoReqFrame.Data)
	if err != nil {
		return fmt.Errorf("extract AMF UE NGAP ID from HandoverRequest: %w", err)
	}

	targetRanUENGAPID := int64(100)
	targetN3IP := netip.MustParseAddr(targetGNBSpec.N3Address)
	targetDLTEID := uint32(9000)

	err = targetGNB.SendHandoverRequestAcknowledge(&gnb.HandoverRequestAcknowledgeOpts{
		AMFUENGAPID: targetAmfUENGAPID,
		RANUENGAPID: targetRanUENGAPID,
		PDUSessions: []gnb.HandoverAdmittedPDUSession{
			{
				PDUSessionID: int64(scenarios.DefaultPDUSessionID),
				DLTeid:       targetDLTEID,
				DLIP:         targetN3IP,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("send HandoverRequestAcknowledge: %w", err)
	}

	_, err = sourceGNB.WaitForMessage(
		ngapType.NGAPPDUPresentSuccessfulOutcome,
		ngapType.SuccessfulOutcomePresentHandoverCommand,
		5*time.Second,
	)
	if err != nil {
		return fmt.Errorf("source gNB: wait HandoverCommand: %w", err)
	}

	err = targetGNB.SendHandoverNotify(&gnb.HandoverNotifyOpts{
		AMFUENGAPID: targetAmfUENGAPID,
		RANUENGAPID: targetRanUENGAPID,
	})
	if err != nil {
		return fmt.Errorf("send HandoverNotify: %w", err)
	}

	_, err = sourceGNB.WaitForMessage(
		ngapType.NGAPPDUPresentInitiatingMessage,
		ngapType.InitiatingMessagePresentUEContextReleaseCommand,
		5*time.Second,
	)
	if err != nil {
		return fmt.Errorf("source gNB: wait UEContextReleaseCommand: %w", err)
	}

	_ = sourceGNB.CloseTunnel(gnbPDUSession.DLTeid)

	time.Sleep(500 * time.Millisecond)

	_, err = targetGNB.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             ueIP,
		UpfIP:            gnbPDUSession.UpfAddress,
		TunInterfaceName: n2HandoverTargetTunPrefix,
		ULteid:           gnbPDUSession.ULTeid,
		DLteid:           targetDLTEID,
		MTU:              uePDUSession.MTU,
		QFI:              newUE.GetPDUSession(scenarios.DefaultPDUSessionID).QFI,
	})
	if err != nil {
		return fmt.Errorf("create target GTP tunnel: %w", err)
	}

	// TS 23.502 §4.9.1.3.3: the UPF is updated with the new AN tunnel during
	// handover completion, so downlink must flow through the target gNB.
	if err := probe.Run(context.Background(), probe.ICMP, n2HandoverTargetTunPrefix, pingDest, scenarios.DefaultProbePort, false); err != nil {
		return fmt.Errorf("ping after N2 handover FAILED (UPF not updated with new AN tunnel info): %w", err)
	}

	logger.Logger.Info("Ping successful after N2 handover", zap.String("dest", pingDest))

	return nil
}
