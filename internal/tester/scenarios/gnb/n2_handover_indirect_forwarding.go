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
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/scenarios/common"
	ngaplib "github.com/ellanetworks/core/ngap"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const n2IndirectForwardingIMSI = "001017271246592"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/n2_handover_indirect_forwarding",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runN2HandoverIndirectForwarding,
		Fixture:   fixtureN2HandoverIndirectForwarding,
	})
}

func fixtureN2HandoverIndirectForwarding(_ scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{
			scenarios.DefaultSubscriberWith(n2IndirectForwardingIMSI, ""),
		},
	}
}

func runN2HandoverIndirectForwarding(_ context.Context, env scenarios.Env, _ any) error {
	if len(env.GNBs) < 2 {
		return fmt.Errorf("n2_handover_indirect_forwarding requires at least 2 gNBs, got %d", len(env.GNBs))
	}

	sourceGNBSpec := env.GNBs[0]
	targetGNBSpec := env.GNBs[1]

	sourceGNB, err := startHandoverGNB("000001", "Source-gNB", env, sourceGNBSpec)
	if err != nil {
		return err
	}

	defer sourceGNB.Close()

	targetGNB, err := startHandoverGNB("000002", "Target-gNB", env, targetGNBSpec)
	if err != nil {
		return err
	}

	defer targetGNB.Close()

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	newUE, err := newDefaultUEForHandover(sourceGNB, n2IndirectForwardingIMSI)
	if err != nil {
		return fmt.Errorf("create UE: %w", err)
	}

	sourceGNB.AddUE(ranUENGAPID, newUE)

	registration, err := sourceGNB.Register(newUE, ranUENGAPID, scenarios.DefaultPDUSessionID, registrationTimeout)
	if err != nil {
		return fmt.Errorf("initial registration: %w", err)
	}

	amfUENGAPID := sourceGNB.GetAMFUENGAPID(ranUENGAPID)

	err = sourceGNB.SendHandoverRequired(&gnb.HandoverRequiredOpts{
		AMFUENGAPID:  amfUENGAPID,
		RANUENGAPID:  ranUENGAPID,
		HandoverType: ngaplib.HandoverTypeIntra5GS,
		TargetGnbID:  "000002",
		PDUSessions: []gnb.HandoverRequiredPDUSession{
			{PDUSessionID: int64(scenarios.DefaultPDUSessionID)},
		},
		SourceToTargetTransparentContainer: n2SourceToTargetContainer,
	})
	if err != nil {
		return fmt.Errorf("send HandoverRequired: %w", err)
	}

	hoReqFrame, err := targetGNB.WaitForMessage(
		gnb.Initiating,
		ngaplib.ProcHandoverResourceAllocation,
		5*time.Second,
	)
	if err != nil {
		return fmt.Errorf("target gNB: wait HandoverRequest: %w", err)
	}

	if err := assertTargetMayForward(hoReqFrame); err != nil {
		return err
	}

	if err := assertSourceToTargetRelayed(hoReqFrame); err != nil {
		return err
	}

	targetAmfUENGAPID, err := common.ExtractAmfUeNgapIDFromHandoverRequest(hoReqFrame.Data)
	if err != nil {
		return fmt.Errorf("extract AMF UE NGAP ID from HandoverRequest: %w", err)
	}

	targetForwardingTEID := targetGNB.AllocateForwardingTEID()

	targetGNB.WatchTEID(targetForwardingTEID)

	err = targetGNB.SendHandoverRequestAcknowledge(&gnb.HandoverRequestAcknowledgeOpts{
		AMFUENGAPID: targetAmfUENGAPID,
		RANUENGAPID: int64(100),
		PDUSessions: []gnb.HandoverAdmittedPDUSession{
			{
				PDUSessionID:   int64(scenarios.DefaultPDUSessionID),
				DLTEID:         uint32(9100),
				DLIP:           netip.MustParseAddr(targetGNBSpec.N3Address),
				ForwardingTEID: targetForwardingTEID,
			},
		},
		TargetToSourceTransparentContainer: n2HandoverRRCContainer,
	})
	if err != nil {
		return fmt.Errorf("send HandoverRequestAcknowledge: %w", err)
	}

	hoCmdFrame, err := sourceGNB.WaitForMessage(
		gnb.Successful,
		ngaplib.ProcHandoverPreparation,
		5*time.Second,
	)
	if err != nil {
		return fmt.Errorf("source gNB: wait HandoverCommand: %w", err)
	}

	if err := assertTargetToSourceRelayed(hoCmdFrame); err != nil {
		return err
	}

	relay, err := forwardingEndpoint(hoCmdFrame, registration.Session.UpfAddress, targetForwardingTEID)
	if err != nil {
		return err
	}

	probe := scenarios.ForwardingProbe{
		Send: func(payload []byte) error {
			return sourceGNB.SendGPDU(relay.teid, relay.addr, payload)
		},
		SendEndMarker: func() error { return sourceGNB.SendEndMarker(relay.teid, relay.addr) },
		Received:      func() int { return targetGNB.WatchedTEIDCount(targetForwardingTEID) },
		EndMarkers:    func() int { return targetGNB.EndMarkerCount(targetForwardingTEID) },
	}

	if err := scenarios.DriveForwardingTunnel(probe); err != nil {
		return err
	}

	logger.Logger.Info("Indirect forwarding relayed user data through the UPF",
		zap.Uint32("upf-teid", relay.teid), zap.Uint32("target-teid", targetForwardingTEID))

	err = targetGNB.SendHandoverNotify(&gnb.HandoverNotifyOpts{
		AMFUENGAPID: targetAmfUENGAPID,
		RANUENGAPID: int64(100),
	})
	if err != nil {
		return fmt.Errorf("send HandoverNotify: %w", err)
	}

	if _, err := sourceGNB.WaitForMessage(gnb.Initiating, ngaplib.ProcUEContextRelease, 5*time.Second); err != nil {
		return fmt.Errorf("source gNB: wait UEContextReleaseCommand: %w", err)
	}

	return scenarios.AwaitForwardingTunnelReleased(probe)
}

func startHandoverGNB(gnbID, name string, env scenarios.Env, spec scenarios.GNB) (*gnb.GnodeB, error) {
	node, err := gnb.Start(&gnb.StartOpts{
		GnbID:           gnbID,
		MCC:             scenarios.DefaultMCC,
		MNC:             scenarios.DefaultMNC,
		SST:             scenarios.DefaultSST,
		SD:              scenarios.DefaultSD,
		DNN:             scenarios.DefaultDNN,
		TAC:             scenarios.DefaultTAC,
		Name:            name,
		CoreN2Addresses: env.CoreN2Addresses,
		GnbN2Address:    spec.N2Address,
		GnbN3Address:    spec.N3Address,
	})
	if err != nil {
		return nil, fmt.Errorf("start %s: %w", name, err)
	}

	if _, err := node.WaitForMessage(gnb.Successful, ngaplib.ProcNGSetup, 2*time.Second); err != nil {
		node.Close()

		return nil, fmt.Errorf("%s: wait NGSetupResponse: %w", name, err)
	}

	return node, nil
}

func assertTargetMayForward(frame gnb.SCTPFrame) error {
	req, err := ngaplib.ParseHandoverRequest(frame.Value)
	if err != nil {
		return fmt.Errorf("parse HandoverRequest: %w", err)
	}

	if len(req.PDUSessionResourceSetupListHOReq) != 1 {
		return fmt.Errorf("HandoverRequest set up %d PDU sessions, want 1", len(req.PDUSessionResourceSetupListHOReq))
	}

	transfer, err := ngaplib.ParsePDUSessionResourceSetupRequestTransfer(req.PDUSessionResourceSetupListHOReq[0].Transfer)
	if err != nil {
		return fmt.Errorf("parse PDUSessionResourceSetupRequestTransfer: %w", err)
	}

	if transfer.DataForwardingNotPossible != nil {
		return fmt.Errorf("the target was told data forwarding is not possible, so it allocates no forwarding endpoint")
	}

	if transfer.DirectForwardingPathAvailability != nil {
		return fmt.Errorf("a direct forwarding path was claimed though the source reported none")
	}

	return nil
}

type relayEndpoint struct {
	teid uint32
	addr netip.Addr
}

func forwardingEndpoint(frame gnb.SCTPFrame, upfAddress string, targetForwardingTEID uint32) (relayEndpoint, error) {
	cmd, err := ngaplib.ParseHandoverCommand(frame.Value)
	if err != nil {
		return relayEndpoint{}, fmt.Errorf("parse HandoverCommand: %w", err)
	}

	if len(cmd.PDUSessionResourceHandoverList) != 1 {
		return relayEndpoint{}, fmt.Errorf("HandoverCommand handed over %d PDU sessions, want 1", len(cmd.PDUSessionResourceHandoverList))
	}

	transfer, err := ngaplib.ParseHandoverCommandTransfer(cmd.PDUSessionResourceHandoverList[0].Transfer)
	if err != nil {
		return relayEndpoint{}, fmt.Errorf("parse HandoverCommandTransfer: %w", err)
	}

	if transfer.DLForwardingUPTNLInformation == nil {
		return relayEndpoint{}, fmt.Errorf("HandoverCommandTransfer carried no forwarding endpoint, so the source forwards nothing")
	}

	teid := uint32(transfer.DLForwardingUPTNLInformation.GTPTunnel.GTPTEID)
	if teid == targetForwardingTEID {
		return relayEndpoint{}, fmt.Errorf("the source was given the target's forwarding TEID %#x, not a tunnel on the UPF", teid)
	}

	v4, v6 := transfer.DLForwardingUPTNLInformation.GTPTunnel.TransportLayerAddress.IPs()

	upf, err := netip.ParseAddr(upfAddress)
	if err != nil {
		return relayEndpoint{}, fmt.Errorf("parse the UPF address %q: %w", upfAddress, err)
	}

	if v4.Unmap() != upf && v6.Unmap() != upf {
		return relayEndpoint{}, fmt.Errorf("the forwarding endpoint is at %v/%v, want the UPF's %v", v4, v6, upf)
	}

	if len(transfer.QosFlowToBeForwarded) != 1 {
		return relayEndpoint{}, fmt.Errorf("QoS flows to be forwarded = %d, want 1", len(transfer.QosFlowToBeForwarded))
	}

	return relayEndpoint{teid: teid, addr: upf}, nil
}
