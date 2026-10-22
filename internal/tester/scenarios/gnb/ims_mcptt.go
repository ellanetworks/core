// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/ims/sip"
	"github.com/spf13/pflag"
)

const (
	mcpttTunIface    = gtpInterfaceNamePrefix + "mc0"
	mcpttRouteTable  = 150
	mcpttCallTimeout = 90 * time.Second

	mcpttNormalPriority  = "mcpttp.11"
	mcpttUpgradePriority = "mcpttp.15"
)

var mcpttCaller = scenarios.MCPTTSubscriber("001018400000021", "+15558400021")

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "ims/5g_mcptt_call",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return runMCPTTCall(ctx, env)
		},
		Fixture: func(env scenarios.Env) scenarios.FixtureSpec {
			return scenarios.MCPTTFixture(env, mcpttCaller)
		},
	})
}

func mcpttFlow(level int64) gnb.QoSFlowParams {
	p := gnb.QoSFlowParams{
		FiveQI:      scenarios.MCPTT5QI,
		PriArp:      level,
		PreemptCap:  ngap.PreemptionMayTrigger,
		PreemptVuln: ngap.PreemptionPreemptable,
		GBR:         true,
	}

	if level <= 2 {
		p.PreemptVuln = ngap.PreemptionNotPreemptable
	}

	return p
}

type mcpttLeg struct {
	voiceLeg
	u *ue.UE
}

func (l mcpttLeg) awaitFlow(ctx context.Context, done func(gnb.QoSFlowParams) bool) (gnb.QoSFlowParams, error) {
	ctx, cancel := context.WithTimeout(ctx, qosFlowTimeout)
	defer cancel()

	for {
		f, ok := l.gnb.DedicatedQoSFlow(l.ranUEID, int64(l.session.PDUSessionID), scenarios.MCPTT5QI)
		if ok && done(f) {
			return f, nil
		}

		select {
		case <-ctx.Done():
			return f, fmt.Errorf("the gNB holds QoS flows %v, MCPTT flow %+v: %w", l.flows(), f, ctx.Err())
		case <-time.After(voicePacketInterval):
		}
	}
}

func (l mcpttLeg) requireFlow(f gnb.QoSFlowParams, level int64, priority string) error {
	want := mcpttFlow(level)
	want.QFI = f.QFI

	switch {
	case len(l.flows()) != 1:
		return fmt.Errorf("the gNB holds QoS flows %v, want one 5QI %d flow for speech and floor control (TS 23.379 §5.7.3, TS 23.289 §4.3.3.2)", l.flows(), scenarios.MCPTT5QI)
	case f != want:
		return fmt.Errorf("the MCPTT QoS flow is %+v for %s, want %+v (TS 29.513 §7.3.2 table 7.3.2-2, TS 29.214 Annex A.13.2)", f, priority, want)
	}

	return nil
}

func runMCPTTCall(ctx context.Context, env scenarios.Env) error {
	ctx, cancel := context.WithTimeout(ctx, mcpttCallTimeout)
	defer cancel()

	g, err := startGNB(env)
	if err != nil {
		return err
	}

	defer g.Close()

	ranUEID := int64(scenarios.DefaultRANUENGAPID)

	u, _, session, local, err := registerAndTunnelIMSUE(env, g, mcpttCaller, ranUEID, mcpttTunIface, scenarios.MCDNN, scenarios.MCUEIPv4Pool, scenarios.MCUEIPv6Pool)
	if err != nil {
		return fmt.Errorf("UE %s: %w", mcpttCaller.IMSI, err)
	}

	defer g.CloseTunnel(session.DLTEID)

	if session.FiveQI != scenarios.MCPolicy5QI || session.ARP != scenarios.MCPolicyARP {
		return fmt.Errorf("%s default QoS flow at 5QI %d ARP %d, want 5QI %d ARP %d", scenarios.MCDNN, session.FiveQI, session.ARP, scenarios.MCPolicy5QI, scenarios.MCPolicyARP)
	}

	acc, err := fgs.ParsePDUSessionEstablishmentAccept(session.Accept)
	if err != nil {
		return fmt.Errorf("parse PDU Session Establishment Accept: %w", err)
	}

	if acc.ExtendedPCO == nil {
		return fmt.Errorf("the PDU Session Establishment Accept has no ePCO")
	}

	pcscf, err := scenarios.SelectPCSCF(acc.ExtendedPCO.PCSCFAddresses(), !env.HasIPv4())
	if err != nil {
		return err
	}

	unroute, err := scenarios.RouteFromUE(mcpttTunIface, local, pcscf, mcpttRouteTable)
	if err != nil {
		return err
	}

	defer unroute()

	as, err := scenarios.StartMCPTTServer(ctx, env)
	if err != nil {
		return err
	}

	defer as.Close()

	unprovision, err := scenarios.ProvisionMCPTTService(ctx, env, scenarios.MCProfileName, as.URI())
	if err != nil {
		return err
	}

	defer unprovision()

	c, err := scenarios.NewMCPTTClient(scenarios.IMSEndpoint{Subscriber: mcpttCaller, PCSCF: pcscf, Local: local}, sip.UDP)
	if err != nil {
		return err
	}

	defer c.Close()

	if err := c.Register(ctx); err != nil {
		return err
	}

	if err := scenarios.RequireThirdPartyRegister(ctx, as, mcpttCaller); err != nil {
		return err
	}

	call, err := c.Call(ctx, scenarios.MCPTTPSI("grp-1"), mcpttNormalPriority)
	if err != nil {
		return err
	}

	callErr := mcpttCallSteps(ctx, as, call, mcpttLeg{voiceLeg: voiceLeg{gnb: g, ranUEID: ranUEID, session: session}, u: u})

	if err := c.Deregister(ctx); err != nil {
		callErr = errors.Join(callErr, err)
	}

	return callErr
}

func mcpttCallSteps(ctx context.Context, as *scenarios.MCPTTServer, call *scenarios.MCPTTCall, l mcpttLeg) error {
	hangUp := func(err error) error {
		return errors.Join(err, call.Bye(ctx))
	}

	if err := scenarios.RequireMCPTTInvite(ctx, as, 1, mcpttNormalPriority); err != nil {
		return hangUp(err)
	}

	if err := scenarios.RequireAuthorisedPriority(call.Response(), mcpttNormalPriority); err != nil {
		return hangUp(fmt.Errorf("200 to the INVITE: %w", err))
	}

	flow, err := l.awaitFlow(ctx, func(gnb.QoSFlowParams) bool { return true })
	if err != nil {
		return hangUp(fmt.Errorf("no 5QI %d MCPTT QoS flow: %w", scenarios.MCPTT5QI, err))
	}

	if err := l.requireFlow(flow, 3, mcpttNormalPriority); err != nil {
		return hangUp(err)
	}

	if err := requireMCPTTRules(l.u, flow.QFI); err != nil {
		return hangUp(err)
	}

	modifies := l.gnb.ModifyRequestCount(l.ranUEID)

	res, err := call.Reprioritize(ctx, mcpttUpgradePriority)
	if err != nil {
		return hangUp(err)
	}

	if err := scenarios.RequireMCPTTInvite(ctx, as, 2, mcpttUpgradePriority); err != nil {
		return hangUp(err)
	}

	if err := scenarios.RequireAuthorisedPriority(res, mcpttUpgradePriority); err != nil {
		return hangUp(fmt.Errorf("200 to the re-INVITE: %w", err))
	}

	upgraded, err := l.awaitFlow(ctx, func(f gnb.QoSFlowParams) bool { return f.PriArp != flow.PriArp })
	if err != nil {
		return hangUp(fmt.Errorf("the MCPTT QoS flow kept its ARP after the upgrade to %s: %w", mcpttUpgradePriority, err))
	}

	switch {
	case l.gnb.ModifyRequestCount(l.ranUEID) == modifies:
		return hangUp(fmt.Errorf("the gNB got no PDU Session Resource Modify Request for the upgrade to %s (TS 23.502 §4.3.3.2)", mcpttUpgradePriority))
	case upgraded.QFI != flow.QFI:
		return hangUp(fmt.Errorf("the upgrade to %s moved the MCPTT flow from QFI %d to %d, want it modified in place", mcpttUpgradePriority, flow.QFI, upgraded.QFI))
	}

	if err := l.requireFlow(upgraded, 1, mcpttUpgradePriority); err != nil {
		return hangUp(err)
	}

	if err := call.Bye(ctx); err != nil {
		return err
	}

	if _, err := as.AwaitRequest(ctx, "BYE", 1); err != nil {
		return err
	}

	if err := awaitQoSFlowsReleased(ctx, []voiceLeg{l.voiceLeg}); err != nil {
		return fmt.Errorf("after the BYE: %w", err)
	}

	return nil
}

func requireMCPTTRules(u *ue.UE, qfi uint8) error {
	raw, err := u.WaitForNASGSMMessage(uint8(fgs.MsgPDUSessionModificationCommand), qosFlowTimeout)
	if err != nil {
		return fmt.Errorf("no PDU Session Modification Command for the MCPTT flow: %w", err)
	}

	cmd, err := fgs.ParsePDUSessionModificationCommand(raw)
	if err != nil {
		return fmt.Errorf("parse the PDU Session Modification Command: %w", err)
	}

	_, err = scenarios.RequireMCPTTQoSFlow(cmd, qfi)

	return err
}
