// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

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
		Name:      "ue/n2_handover_connectivity",
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

// runN2HandoverConnectivity exercises the N2 handover procedure and then
// verifies that data plane connectivity survives after handover.
func runN2HandoverConnectivity(_ context.Context, env scenarios.Env, _ any) error {
	if len(env.GNBs) < 2 {
		return fmt.Errorf("n2_handover_connectivity requires at least 2 gNBs, got %d", len(env.GNBs))
	}

	sourceGNBSpec := env.GNBs[0]
	targetGNBSpec := env.GNBs[1]

	// Start source gNB.
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

	// Start target gNB.
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

	// Register UE on source gNB.
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

	// Create GTP tunnel on source gNB and verify connectivity.
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

	// Verify connectivity before handover.
	pingDest := env.PingDestination()
	cmd := exec.CommandContext(context.Background(), "ping", "-I", n2HandoverTunInterface, pingDest, "-c", "3", "-W", "1") // #nosec G204

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ping before handover failed: %v\noutput:\n%s", err, string(out))
	}

	logger.Logger.Info("Ping successful before handover", zap.String("dest", pingDest))

	// === Execute N2 Handover ===

	// Source gNB → AMF: HandoverRequired
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

	// AMF → Target gNB: HandoverRequest
	hoReqFrame, err := targetGNB.WaitForMessage(
		ngapType.NGAPPDUPresentInitiatingMessage,
		ngapType.InitiatingMessagePresentHandoverRequest,
		5*time.Second,
	)
	if err != nil {
		return fmt.Errorf("target gNB: wait HandoverRequest: %w", err)
	}

	// Decode the HandoverRequest to extract the AMF UE NGAP ID assigned for target.
	targetAmfUENGAPID, err := common.ExtractAmfUeNgapIDFromHandoverRequest(hoReqFrame.Data)
	if err != nil {
		return fmt.Errorf("extract AMF UE NGAP ID from HandoverRequest: %w", err)
	}

	// Target gNB → AMF: HandoverRequestAcknowledge
	// The target gNB reports its new DL tunnel endpoint.
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

	// AMF → Source gNB: HandoverCommand
	_, err = sourceGNB.WaitForMessage(
		ngapType.NGAPPDUPresentSuccessfulOutcome,
		ngapType.SuccessfulOutcomePresentHandoverCommand,
		5*time.Second,
	)
	if err != nil {
		return fmt.Errorf("source gNB: wait HandoverCommand: %w", err)
	}

	// Target gNB → AMF: HandoverNotify
	err = targetGNB.SendHandoverNotify(&gnb.HandoverNotifyOpts{
		AMFUENGAPID: targetAmfUENGAPID,
		RANUENGAPID: targetRanUENGAPID,
	})
	if err != nil {
		return fmt.Errorf("send HandoverNotify: %w", err)
	}

	// Wait for source gNB to receive UEContextReleaseCommand.
	_, err = sourceGNB.WaitForMessage(
		ngapType.NGAPPDUPresentInitiatingMessage,
		ngapType.InitiatingMessagePresentUEContextReleaseCommand,
		5*time.Second,
	)
	if err != nil {
		return fmt.Errorf("source gNB: wait UEContextReleaseCommand: %w", err)
	}

	// Tear down the old tunnel on source gNB.
	_ = sourceGNB.CloseTunnel(gnbPDUSession.DLTeid)

	// === Post-handover: create tunnel on target gNB and verify connectivity ===
	// The UPF should now be forwarding downlink to the target gNB's N3
	// tunnel (targetDLTEID @ targetN3IP). We set up the corresponding
	// GTP tunnel on the target side.

	// Give the system a moment to propagate the UPF update (if it were
	// implemented correctly). Without the fix, ModifySession is never called
	// and this will fail because downlink goes to the old tunnel.
	time.Sleep(500 * time.Millisecond)

	_, err = targetGNB.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             ueIP,
		UpfIP:            gnbPDUSession.UpfAddress, // UPF address is the same
		TunInterfaceName: n2HandoverTargetTunPrefix,
		ULteid:           gnbPDUSession.ULTeid, // UL TEID to UPF stays the same
		DLteid:           targetDLTEID,         // Our new DL TEID
		MTU:              uePDUSession.MTU,
		QFI:              newUE.GetPDUSession(scenarios.DefaultPDUSessionID).QFI,
	})
	if err != nil {
		return fmt.Errorf("create target GTP tunnel: %w", err)
	}

	// Verify connectivity after handover.
	// Per 3GPP TS 23.502 §4.9.1.3.3, the UPF is updated with the new AN
	// tunnel info during handover completion, so downlink traffic should
	// flow through the target gNB's GTP tunnel.
	cmd = exec.CommandContext(context.Background(), "ping", "-I", n2HandoverTargetTunPrefix, pingDest, "-c", "3", "-W", "2") // #nosec G204

	out, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ping after N2 handover FAILED (UPF not updated with new AN tunnel info): %v\noutput:\n%s", err, string(out))
	}

	logger.Logger.Info("Ping successful after N2 handover", zap.String("dest", pingDest))

	return nil
}
