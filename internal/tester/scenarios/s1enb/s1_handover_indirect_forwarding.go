// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"net/netip"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const s1hoIndirectIMSI = "001017271246682"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/s1_handover_indirect_forwarding",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1HandoverIndirectForwarding,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(s1hoIndirectIMSI, "")},
			}
		},
	})
}

func runS1HandoverIndirectForwarding(_ context.Context, env scenarios.Env, _ any) error {
	if len(env.GNBs) < 2 {
		return fmt.Errorf("s1_handover_indirect_forwarding requires at least 2 eNB radios, got %d", len(env.GNBs))
	}

	s1mme, err := s1mmeAddress(env.FirstCore())
	if err != nil {
		return err
	}

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	enbID, err := strconv.ParseUint(scenarios.DefaultGNBID, 16, 32)
	if err != nil {
		return fmt.Errorf("parse eNB ID %q: %w", scenarios.DefaultGNBID, err)
	}

	sourceSpec, targetSpec := env.GNBs[0], env.GNBs[1]

	source, err := s1enb.Start(&s1enb.StartOpts{
		ENBID: uint32(enbID), MCC: scenarios.DefaultMCC, MNC: scenarios.DefaultMNC, TAC: scenarios.DefaultTAC,
		Name: "Source-S1eNB", CoreS1MMEAddress: s1mme,
		ENBAddress: sourceSpec.N2Address, ENBN3Address: sourceSpec.N3Address, EnableDatapath: true,
	})
	if err != nil {
		return fmt.Errorf("start source eNB: %w", err)
	}

	defer func() { _ = source.Close() }()

	target, err := s1enb.Start(&s1enb.StartOpts{
		ENBID: uint32(enbID) + 1, MCC: scenarios.DefaultMCC, MNC: scenarios.DefaultMNC, TAC: scenarios.DefaultTAC,
		Name: "Target-S1eNB", CoreS1MMEAddress: s1mme,
		ENBAddress: targetSpec.N2Address, ENBN3Address: targetSpec.N3Address, EnableDatapath: true,
	})
	if err != nil {
		return fmt.Errorf("start target eNB: %w", err)
	}

	defer func() { _ = target.Close() }()

	ue := source.NewUE(s1hoIndirectIMSI, k, opc)
	ue.RequestPDNType(env.PDUSessionType())

	res, err := source.Attach(ue, attachTimeout)
	if err != nil {
		return fmt.Errorf("attach on source eNB: %w", err)
	}

	if err := source.SendHandoverRequired(res.ENBUES1APID, res.MMEUES1APID, target.GlobalENBID(), false); err != nil {
		return fmt.Errorf("send Handover Required: %w", err)
	}

	hoReq, err := target.WaitForHandoverRequest(10 * time.Second)
	if err != nil {
		return fmt.Errorf("await Handover Request: %w", err)
	}

	if len(hoReq.ERABToBeSetup) != 1 {
		return fmt.Errorf("handover request carried %d E-RABs, want 1", len(hoReq.ERABToBeSetup))
	}

	if ext := hoReq.ERABToBeSetup[0].Extensions; ext != nil && ext.DataForwardingNotPossible != nil {
		return fmt.Errorf("the target was told data forwarding is not possible, so it allocates no forwarding endpoint")
	}

	if err := assertSourceToTargetRelayed(hoReq); err != nil {
		return err
	}

	targetMMEUEID := int64(hoReq.MMEUES1APID)
	targetENBUEID := target.AllocateENBUEID()

	_, targetForwardingTEID, err := target.SendHandoverRequestAcknowledge(targetENBUEID, targetMMEUEID, res.ERABID, true)
	if err != nil {
		return fmt.Errorf("send Handover Request Acknowledge: %w", err)
	}

	target.WatchTEID(targetForwardingTEID)

	cmd, err := source.WaitForHandoverCommand(res.ENBUES1APID, 10*time.Second)
	if err != nil {
		return fmt.Errorf("await Handover Command: %w", err)
	}

	if err := assertTargetToSourceRelayed(cmd); err != nil {
		return err
	}

	relayTEID, relayAddr, err := s1ForwardingEndpoint(cmd, res.ERABID, res.UpfAddress, targetForwardingTEID)
	if err != nil {
		return err
	}

	probe := scenarios.ForwardingProbe{
		Send: func(payload []byte) error {
			return source.SendGPDU(relayTEID, relayAddr, payload)
		},
		SendEndMarker: func() error { return source.SendEndMarker(relayTEID, relayAddr) },
		Received:      func() int { return target.WatchedTEIDCount(targetForwardingTEID) },
		EndMarkers:    func() int { return target.EndMarkerCount(targetForwardingTEID) },
	}

	if err := scenarios.DriveForwardingTunnel(probe); err != nil {
		return err
	}

	logger.Logger.Info("Indirect forwarding relayed user data through the UPF",
		zap.Uint32("upf-teid", relayTEID), zap.Uint32("target-teid", targetForwardingTEID))

	if err := target.SendHandoverNotify(targetENBUEID, targetMMEUEID); err != nil {
		return fmt.Errorf("send Handover Notify: %w", err)
	}

	if _, err := source.WaitForUEContextReleaseCommand(res.ENBUES1APID, 10*time.Second); err != nil {
		return fmt.Errorf("await source UE Context Release Command: %w", err)
	}

	if err := source.SendUEContextReleaseComplete(res.MMEUES1APID, res.ENBUES1APID); err != nil {
		return fmt.Errorf("send source UE Context Release Complete: %w", err)
	}

	return scenarios.AwaitForwardingTunnelReleased(probe)
}

func s1ForwardingEndpoint(cmd *s1ap.HandoverCommand, erabID s1ap.ERABID, upfAddress string, targetForwardingTEID uint32) (uint32, netip.Addr, error) {
	if len(cmd.ERABSubjecttoDataForwarding) != 1 {
		return 0, netip.Addr{}, fmt.Errorf("handover command carried %d forwarding items, want 1", len(cmd.ERABSubjecttoDataForwarding))
	}

	item := cmd.ERABSubjecttoDataForwarding[0]
	if item.ERABID != erabID {
		return 0, netip.Addr{}, fmt.Errorf("forwarding item names E-RAB %d, want %d", item.ERABID, erabID)
	}

	if item.DLGTPTEID == nil {
		return 0, netip.Addr{}, fmt.Errorf("forwarding item carried no DL GTP-TEID, so the source forwards nothing")
	}

	teid := uint32(*item.DLGTPTEID)
	if teid == targetForwardingTEID {
		return 0, netip.Addr{}, fmt.Errorf("the source was given the target's forwarding TEID %#x, not a tunnel on the UPF", teid)
	}

	v4, v6 := item.DLTransportLayerAddr.IPs()

	upf, err := netip.ParseAddr(upfAddress)
	if err != nil {
		return 0, netip.Addr{}, fmt.Errorf("parse the S-GW address %q: %w", upfAddress, err)
	}

	if v4.Unmap() != upf && v6.Unmap() != upf {
		return 0, netip.Addr{}, fmt.Errorf("the forwarding endpoint is at %v/%v, want the S-GW's %v", v4, v6, upf)
	}

	return teid, upf, nil
}
