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

const (
	s1PingPongIMSI          = "001017271246686"
	s1PingPongWindow        = 2 * time.Second
	s1PingPongReleaseMargin = 750 * time.Millisecond
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/s1_handover_ping_pong",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1HandoverPingPong,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(s1PingPongIMSI, "")},
			}
		},
	})
}

type s1LegEndpoints struct {
	TargetENBUEID int64
	TargetMMEUEID int64
	TargetFwdTEID uint32
	RelayTEID     uint32
	RelayAddr     netip.Addr
}

func runS1HandoverPingPong(_ context.Context, env scenarios.Env, _ any) error {
	if len(env.GNBs) < 2 {
		return fmt.Errorf("s1_handover_ping_pong requires at least 2 eNB radios, got %d", len(env.GNBs))
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

	firstSpec, secondSpec := env.GNBs[0], env.GNBs[1]

	first, err := s1enb.Start(&s1enb.StartOpts{
		ENBID: uint32(enbID), MCC: scenarios.DefaultMCC, MNC: scenarios.DefaultMNC, TAC: scenarios.DefaultTAC,
		Name: "PingPong-A-S1eNB", CoreS1MMEAddress: s1mme,
		ENBAddress: firstSpec.N2Address, ENBN3Address: firstSpec.N3Address, EnableDatapath: true,
	})
	if err != nil {
		return fmt.Errorf("start eNB A: %w", err)
	}

	defer func() { _ = first.Close() }()

	second, err := s1enb.Start(&s1enb.StartOpts{
		ENBID: uint32(enbID) + 1, MCC: scenarios.DefaultMCC, MNC: scenarios.DefaultMNC, TAC: scenarios.DefaultTAC,
		Name: "PingPong-B-S1eNB", CoreS1MMEAddress: s1mme,
		ENBAddress: secondSpec.N2Address, ENBN3Address: secondSpec.N3Address, EnableDatapath: true,
	})
	if err != nil {
		return fmt.Errorf("start eNB B: %w", err)
	}

	defer func() { _ = second.Close() }()

	ue := first.NewUE(s1PingPongIMSI, k, opc)
	ue.RequestPDNType(env.PDUSessionType())

	res, err := first.Attach(ue, attachTimeout)
	if err != nil {
		return fmt.Errorf("attach on eNB A: %w", err)
	}

	outbound, err := s1PrepareHandoverLeg(first, second, res.ENBUES1APID, res.MMEUES1APID, res.ERABID, res.UpfAddress)
	if err != nil {
		return fmt.Errorf("first leg A to B: %w", err)
	}

	outboundProbe := s1ForwardingProbe(first, second, outbound)
	if err := scenarios.DriveForwardingTunnel(outboundProbe); err != nil {
		return fmt.Errorf("first leg forwarding A to B: %w", err)
	}

	if err := second.SendHandoverNotify(outbound.TargetENBUEID, outbound.TargetMMEUEID); err != nil {
		return fmt.Errorf("first leg send Handover Notify: %w", err)
	}

	completed := time.Now()

	if _, err := first.WaitForUEContextReleaseCommand(res.ENBUES1APID, 10*time.Second); err != nil {
		return fmt.Errorf("first leg await source UE Context Release Command: %w", err)
	}

	if err := first.SendUEContextReleaseComplete(res.MMEUES1APID, res.ENBUES1APID); err != nil {
		return fmt.Errorf("first leg send source UE Context Release Complete: %w", err)
	}

	inbound, err := s1PrepareHandoverLeg(second, first, outbound.TargetENBUEID, outbound.TargetMMEUEID, res.ERABID, res.UpfAddress)
	if err != nil {
		return fmt.Errorf("second leg B to A: %w", err)
	}

	if elapsed := time.Since(completed); elapsed >= s1PingPongWindow {
		return fmt.Errorf("the second leg was prepared %v after the first completed, past the %v forwarding window: the overlap was not exercised", elapsed, s1PingPongWindow)
	}

	if inbound.RelayTEID == outbound.RelayTEID && inbound.RelayAddr == outbound.RelayAddr {
		logger.Logger.Info("the core reused the same forwarding endpoint for the second leg",
			zap.Uint32("relay-teid", inbound.RelayTEID))
	}

	staleOnB := second.WatchedTEIDCount(outbound.TargetFwdTEID)

	inboundProbe := s1ForwardingProbe(second, first, inbound)
	if err := scenarios.DriveForwardingTunnel(inboundProbe); err != nil {
		return fmt.Errorf("second leg forwarding B to A (the first leg's release tore down the second leg's tunnel?): %w", err)
	}

	if leaked := second.WatchedTEIDCount(outbound.TargetFwdTEID) - staleOnB; leaked != 0 {
		return fmt.Errorf("%d packets sent for the second leg arrived on the first leg's forwarding tunnel at eNB B", leaked)
	}

	if remaining := s1PingPongWindow - time.Since(completed) + s1PingPongReleaseMargin; remaining > 0 {
		time.Sleep(remaining)
	}

	if err := scenarios.AwaitForwardingTunnelStillRelays(inboundProbe); err != nil {
		return fmt.Errorf("the first leg's scheduled release tore down the second leg's forwarding tunnel: %w", err)
	}

	logger.Logger.Info("Ping-pong handover relayed both legs through the UPF",
		zap.Uint32("first-relay-teid", outbound.RelayTEID),
		zap.Uint32("second-relay-teid", inbound.RelayTEID))

	if err := first.SendHandoverNotify(inbound.TargetENBUEID, inbound.TargetMMEUEID); err != nil {
		return fmt.Errorf("second leg send Handover Notify: %w", err)
	}

	if _, err := second.WaitForUEContextReleaseCommand(outbound.TargetENBUEID, 10*time.Second); err != nil {
		return fmt.Errorf("second leg await source UE Context Release Command: %w", err)
	}

	if err := second.SendUEContextReleaseComplete(outbound.TargetMMEUEID, outbound.TargetENBUEID); err != nil {
		return fmt.Errorf("second leg send source UE Context Release Complete: %w", err)
	}

	return scenarios.AwaitForwardingTunnelReleased(inboundProbe)
}

func s1PrepareHandoverLeg(source, target *s1enb.ENB, sourceENBUEID, sourceMMEUEID int64, erabID s1ap.ERABID, upfAddress string) (s1LegEndpoints, error) {
	if err := source.SendHandoverRequired(sourceENBUEID, sourceMMEUEID, target.GlobalENBID(), false); err != nil {
		return s1LegEndpoints{}, fmt.Errorf("send Handover Required: %w", err)
	}

	hoReq, err := target.WaitForHandoverRequest(10 * time.Second)
	if err != nil {
		return s1LegEndpoints{}, fmt.Errorf("await Handover Request: %w", err)
	}

	if len(hoReq.ERABToBeSetup) != 1 {
		return s1LegEndpoints{}, fmt.Errorf("handover request carried %d E-RABs, want 1", len(hoReq.ERABToBeSetup))
	}

	if ext := hoReq.ERABToBeSetup[0].Extensions; ext != nil && ext.DataForwardingNotPossible != nil {
		return s1LegEndpoints{}, fmt.Errorf("the target was told data forwarding is not possible, so it allocates no forwarding endpoint")
	}

	if err := assertSourceToTargetRelayed(hoReq); err != nil {
		return s1LegEndpoints{}, err
	}

	targetMMEUEID := int64(hoReq.MMEUES1APID)
	targetENBUEID := target.AllocateENBUEID()

	_, targetFwdTEID, err := target.SendHandoverRequestAcknowledge(targetENBUEID, targetMMEUEID, erabID, true)
	if err != nil {
		return s1LegEndpoints{}, fmt.Errorf("send Handover Request Acknowledge: %w", err)
	}

	target.WatchTEID(targetFwdTEID)

	cmd, err := source.WaitForHandoverCommand(sourceENBUEID, 10*time.Second)
	if err != nil {
		return s1LegEndpoints{}, fmt.Errorf("await Handover Command: %w", err)
	}

	if err := assertTargetToSourceRelayed(cmd); err != nil {
		return s1LegEndpoints{}, err
	}

	relayTEID, relayAddr, err := s1ForwardingEndpoint(cmd, erabID, upfAddress, targetFwdTEID)
	if err != nil {
		return s1LegEndpoints{}, err
	}

	return s1LegEndpoints{
		TargetENBUEID: targetENBUEID,
		TargetMMEUEID: targetMMEUEID,
		TargetFwdTEID: targetFwdTEID,
		RelayTEID:     relayTEID,
		RelayAddr:     relayAddr,
	}, nil
}

func s1ForwardingProbe(source, target *s1enb.ENB, leg s1LegEndpoints) scenarios.ForwardingProbe {
	return scenarios.ForwardingProbe{
		Send: func(payload []byte) error {
			return source.SendGPDU(leg.RelayTEID, leg.RelayAddr, payload)
		},
		SendEndMarker: func() error { return source.SendEndMarker(leg.RelayTEID, leg.RelayAddr) },
		Received:      func() int { return target.WatchedTEIDCount(leg.TargetFwdTEID) },
		EndMarkers:    func() int { return target.EndMarkerCount(leg.TargetFwdTEID) },
	}
}
