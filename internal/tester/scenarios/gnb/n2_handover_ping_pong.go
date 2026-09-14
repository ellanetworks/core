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

const (
	n2PingPongIMSI          = "001017271246598"
	n2PingPongWindow        = 2 * time.Second
	n2PingPongReleaseMargin = 750 * time.Millisecond

	n2PingPongFirstGNBID  = "000001"
	n2PingPongSecondGNBID = "000002"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/ngap/n2_handover_ping_pong",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runN2HandoverPingPong,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{
					scenarios.DefaultSubscriberWith(n2PingPongIMSI, ""),
				},
			}
		},
	})
}

type n2LegEndpoints struct {
	TargetAMFUENGAPID int64
	TargetRANUENGAPID int64
	TargetFwdTEID     uint32
	Relay             relayEndpoint
}

func runN2HandoverPingPong(_ context.Context, env scenarios.Env, _ any) error {
	if len(env.GNBs) < 2 {
		return fmt.Errorf("n2_handover_ping_pong requires at least 2 gNBs, got %d", len(env.GNBs))
	}

	firstSpec, secondSpec := env.GNBs[0], env.GNBs[1]

	first, err := startHandoverGNB(n2PingPongFirstGNBID, "PingPong-A-gNB", env, firstSpec)
	if err != nil {
		return err
	}

	defer first.Close()

	second, err := startHandoverGNB(n2PingPongSecondGNBID, "PingPong-B-gNB", env, secondSpec)
	if err != nil {
		return err
	}

	defer second.Close()

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	newUE, err := newDefaultUEForHandover(first, n2PingPongIMSI)
	if err != nil {
		return fmt.Errorf("create UE: %w", err)
	}

	first.AddUE(ranUENGAPID, newUE)

	registration, err := first.Register(newUE, ranUENGAPID, scenarios.DefaultPDUSessionID, registrationTimeout)
	if err != nil {
		return fmt.Errorf("initial registration: %w", err)
	}

	outbound, err := n2PrepareHandoverLeg(&n2LegOpts{
		Source:         first,
		Target:         second,
		TargetGNBID:    n2PingPongSecondGNBID,
		TargetN3Addr:   netip.MustParseAddr(secondSpec.N3Address),
		AMFUENGAPID:    first.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID:    ranUENGAPID,
		NewRANUENGAPID: 100,
		DLTEID:         9100,
		UpfAddress:     registration.Session.UpfAddress,
	})
	if err != nil {
		return fmt.Errorf("first leg A to B: %w", err)
	}

	outboundProbe := n2ForwardingProbe(first, second, outbound)
	if err := scenarios.DriveForwardingTunnel(outboundProbe); err != nil {
		return fmt.Errorf("first leg forwarding A to B: %w", err)
	}

	err = second.SendHandoverNotify(&gnb.HandoverNotifyOpts{
		AMFUENGAPID: outbound.TargetAMFUENGAPID,
		RANUENGAPID: outbound.TargetRANUENGAPID,
	})
	if err != nil {
		return fmt.Errorf("first leg send HandoverNotify: %w", err)
	}

	completed := time.Now()

	if _, err := first.WaitForMessage(gnb.Initiating, ngaplib.ProcUEContextRelease, 5*time.Second); err != nil {
		return fmt.Errorf("first leg: source gNB got no UEContextReleaseCommand: %w", err)
	}

	inbound, err := n2PrepareHandoverLeg(&n2LegOpts{
		Source:         second,
		Target:         first,
		TargetGNBID:    n2PingPongFirstGNBID,
		TargetN3Addr:   netip.MustParseAddr(firstSpec.N3Address),
		AMFUENGAPID:    outbound.TargetAMFUENGAPID,
		RANUENGAPID:    outbound.TargetRANUENGAPID,
		NewRANUENGAPID: 200,
		DLTEID:         9200,
		UpfAddress:     registration.Session.UpfAddress,
	})
	if err != nil {
		return fmt.Errorf("second leg B to A: %w", err)
	}

	if elapsed := time.Since(completed); elapsed >= n2PingPongWindow {
		return fmt.Errorf("the second leg was prepared %v after the first completed, past the %v forwarding window: the overlap was not exercised", elapsed, n2PingPongWindow)
	}

	staleOnB := second.WatchedTEIDCount(outbound.TargetFwdTEID)

	inboundProbe := n2ForwardingProbe(second, first, inbound)
	if err := scenarios.DriveForwardingTunnel(inboundProbe); err != nil {
		return fmt.Errorf("second leg forwarding B to A (the first leg's release tore down the second leg's tunnel?): %w", err)
	}

	if leaked := second.WatchedTEIDCount(outbound.TargetFwdTEID) - staleOnB; leaked != 0 {
		return fmt.Errorf("%d packets sent for the second leg arrived on the first leg's forwarding tunnel at gNB B", leaked)
	}

	if remaining := n2PingPongWindow - time.Since(completed) + n2PingPongReleaseMargin; remaining > 0 {
		time.Sleep(remaining)
	}

	if err := scenarios.AwaitForwardingTunnelStillRelays(inboundProbe); err != nil {
		return fmt.Errorf("the first leg's scheduled release tore down the second leg's forwarding tunnel: %w", err)
	}

	logger.Logger.Info("Ping-pong handover relayed both legs through the UPF",
		zap.Uint32("first-relay-teid", outbound.Relay.teid),
		zap.Uint32("second-relay-teid", inbound.Relay.teid))

	err = first.SendHandoverNotify(&gnb.HandoverNotifyOpts{
		AMFUENGAPID: inbound.TargetAMFUENGAPID,
		RANUENGAPID: inbound.TargetRANUENGAPID,
	})
	if err != nil {
		return fmt.Errorf("second leg send HandoverNotify: %w", err)
	}

	if _, err := second.WaitForMessage(gnb.Initiating, ngaplib.ProcUEContextRelease, 5*time.Second); err != nil {
		return fmt.Errorf("second leg: source gNB got no UEContextReleaseCommand: %w", err)
	}

	return scenarios.AwaitForwardingTunnelReleased(inboundProbe)
}

type n2LegOpts struct {
	Source         *gnb.GnodeB
	Target         *gnb.GnodeB
	TargetGNBID    string
	TargetN3Addr   netip.Addr
	AMFUENGAPID    int64
	RANUENGAPID    int64
	NewRANUENGAPID int64
	DLTEID         uint32
	UpfAddress     string
}

func n2PrepareHandoverLeg(opts *n2LegOpts) (n2LegEndpoints, error) {
	err := opts.Source.SendHandoverRequired(&gnb.HandoverRequiredOpts{
		AMFUENGAPID:  opts.AMFUENGAPID,
		RANUENGAPID:  opts.RANUENGAPID,
		HandoverType: ngaplib.HandoverTypeIntra5GS,
		TargetGnbID:  opts.TargetGNBID,
		PDUSessions: []gnb.HandoverRequiredPDUSession{
			{PDUSessionID: int64(scenarios.DefaultPDUSessionID)},
		},
	})
	if err != nil {
		return n2LegEndpoints{}, fmt.Errorf("send HandoverRequired: %w", err)
	}

	hoReqFrame, err := opts.Target.WaitForMessage(gnb.Initiating, ngaplib.ProcHandoverResourceAllocation, 5*time.Second)
	if err != nil {
		return n2LegEndpoints{}, fmt.Errorf("target gNB: wait HandoverRequest: %w", err)
	}

	if err := assertTargetMayForward(hoReqFrame); err != nil {
		return n2LegEndpoints{}, err
	}

	targetAMFUENGAPID, err := common.ExtractAmfUeNgapIDFromHandoverRequest(hoReqFrame.Data)
	if err != nil {
		return n2LegEndpoints{}, fmt.Errorf("extract AMF UE NGAP ID from HandoverRequest: %w", err)
	}

	forwardingTEID := opts.Target.AllocateForwardingTEID()

	opts.Target.WatchTEID(forwardingTEID)

	err = opts.Target.SendHandoverRequestAcknowledge(&gnb.HandoverRequestAcknowledgeOpts{
		AMFUENGAPID: targetAMFUENGAPID,
		RANUENGAPID: opts.NewRANUENGAPID,
		PDUSessions: []gnb.HandoverAdmittedPDUSession{
			{
				PDUSessionID:   int64(scenarios.DefaultPDUSessionID),
				DLTEID:         opts.DLTEID,
				DLIP:           opts.TargetN3Addr,
				ForwardingTEID: forwardingTEID,
			},
		},
		TargetToSourceTransparentContainer: n2HandoverRRCContainer,
	})
	if err != nil {
		return n2LegEndpoints{}, fmt.Errorf("send HandoverRequestAcknowledge: %w", err)
	}

	hoCmdFrame, err := opts.Source.WaitForMessage(gnb.Successful, ngaplib.ProcHandoverPreparation, 5*time.Second)
	if err != nil {
		return n2LegEndpoints{}, fmt.Errorf("source gNB: wait HandoverCommand: %w", err)
	}

	relay, err := forwardingEndpoint(hoCmdFrame, opts.UpfAddress, forwardingTEID)
	if err != nil {
		return n2LegEndpoints{}, err
	}

	return n2LegEndpoints{
		TargetAMFUENGAPID: targetAMFUENGAPID,
		TargetRANUENGAPID: opts.NewRANUENGAPID,
		TargetFwdTEID:     forwardingTEID,
		Relay:             relay,
	}, nil
}

func n2ForwardingProbe(source, target *gnb.GnodeB, leg n2LegEndpoints) scenarios.ForwardingProbe {
	return scenarios.ForwardingProbe{
		Send: func(payload []byte) error {
			return source.SendGPDU(leg.Relay.teid, leg.Relay.addr, payload)
		},
		SendEndMarker: func() error { return source.SendEndMarker(leg.Relay.teid, leg.Relay.addr) },
		Received:      func() int { return target.WatchedTEIDCount(leg.TargetFwdTEID) },
		EndMarkers:    func() int { return target.EndMarkerCount(leg.TargetFwdTEID) },
	}
}
