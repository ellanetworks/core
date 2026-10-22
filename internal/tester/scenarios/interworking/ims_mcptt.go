// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/s1ap"
	"github.com/ellanetworks/ims/sip"
	"github.com/spf13/pflag"
)

const (
	mcpttCallTimeout    = 90 * time.Second
	mcpttQCI            = uint8(scenarios.MCPTT5QI)
	mcpttNormalPriority = "mcpttp.11"
)

var (
	mcpttHandoverCaller = scenarios.MCPTTSubscriber("001018600000021", "+15558600021")
	mcpttNormalARP      = s1ap.AllocationAndRetentionPriority{PriorityLevel: 3, PreemptionCapability: s1ap.PreemptionMayTrigger, PreemptionVulnerability: s1ap.PreemptionPreemptable}
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "ims/5g_mcptt_call_handover_to_4g",
		BindFlags: func(_ *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return runMCPTTCallHandoverTo4G(ctx, env)
		},
		Fixture: func(env scenarios.Env) scenarios.FixtureSpec {
			return scenarios.MCPTTFixture(env, mcpttHandoverCaller)
		},
	})
}

func registerMCPTTPartyOn5GS(env scenarios.Env, g *gnb.GnodeB, sub scenarios.SubscriberSpec) (*imsParty, error) {
	u, err := newUEOn5GS(g, env, sub, true, imsPDUSessionID, scenarios.MCDNN)
	if err != nil {
		return nil, err
	}

	g.AddUE(imsFirstGNBRANUEID, u)

	reg, err := g.Register(u, imsFirstGNBRANUEID, imsPDUSessionID, registrationTimeout)
	if err != nil {
		return nil, fmt.Errorf("UE %s: registration over NR: %w", sub.IMSI, err)
	}

	if err := assertEPSNASAlgorithms(u); err != nil {
		return nil, err
	}

	acc, err := fgs.ParsePDUSessionEstablishmentAccept(reg.Session.Accept)
	if err != nil {
		return nil, fmt.Errorf("UE %s: parse PDU Session Establishment Accept: %w", sub.IMSI, err)
	}

	if acc.ExtendedPCO == nil {
		return nil, fmt.Errorf("UE %s: the PDU Session Establishment Accept has no ePCO", sub.IMSI)
	}

	p := &imsParty{
		env:     env,
		sub:     sub,
		iface:   imsTunIface + "0",
		table:   imsRouteTable,
		ueIPv4:  reg.Session.UEIPv4,
		ueIPv6:  reg.Session.UEIPv6,
		nr:      &nrAttachment{g: g, u: u, ranUEID: imsFirstGNBRANUEID, session: reg.Session},
		bearers: imsOnlyBearers,
		mc:      true,
	}

	pcscf, err := scenarios.SelectPCSCF(acc.ExtendedPCO.PCSCFAddresses(), !env.HasIPv4())
	if err != nil {
		return nil, err
	}

	return p, p.route(pcscf)
}

func runMCPTTCallHandoverTo4G(ctx context.Context, env scenarios.Env) error {
	ctx, cancel := context.WithTimeout(ctx, mcpttCallTimeout)
	defer cancel()

	r, err := startIMSCallRANs(env)
	if err != nil {
		return err
	}

	defer r.close()

	p, err := registerMCPTTPartyOn5GS(env, r.g, mcpttHandoverCaller)
	if err != nil {
		return err
	}

	defer p.close()

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

	c, err := scenarios.NewMCPTTClient(p.ep, sip.UDP)
	if err != nil {
		return err
	}

	defer c.Close()

	if err := c.Register(ctx); err != nil {
		return err
	}

	if err := scenarios.RequireThirdPartyRegister(ctx, as, mcpttHandoverCaller); err != nil {
		return err
	}

	call, err := c.Call(ctx, scenarios.MCPTTPSI("grp-1"), mcpttNormalPriority)
	if err != nil {
		return err
	}

	callErr := mcpttHandoverSteps(ctx, r, p, as, call)

	if err := c.Deregister(ctx); err != nil {
		callErr = errors.Join(callErr, err)
	}

	return callErr
}

func mcpttHandoverSteps(ctx context.Context, r *imsCallRANs, p *imsParty, as *scenarios.MCPTTServer, call *scenarios.MCPTTCall) error {
	hangUp := func(err error) error {
		return errors.Join(err, call.Bye(ctx))
	}

	if err := scenarios.RequireMCPTTInvite(ctx, as, 1, mcpttNormalPriority); err != nil {
		return hangUp(err)
	}

	if err := scenarios.RequireAuthorisedPriority(call.Response(), mcpttNormalPriority); err != nil {
		return hangUp(fmt.Errorf("200 to the INVITE: %w", err))
	}

	qfi, err := p.nr.awaitVoiceQFI(ctx)
	if err != nil {
		return hangUp(err)
	}

	if err := requireMCPTTFlowOn5GS(p.nr, qfi); err != nil {
		return hangUp(err)
	}

	h, err := p.handOverToEPSWith(r.e, epsHandoverOpts{carried: map[uint8]s1ap.ERABID{mcpttQCI: imsVoiceEBI}})
	if err != nil {
		return hangUp(fmt.Errorf("handover to EPS: %w", err))
	}

	if qos := h.qos[mcpttQCI]; qos.ARP != mcpttNormalARP {
		return hangUp(fmt.Errorf("the QCI %d E-RAB of the Handover Request has ARP %+v for %s, want the call's %+v kept (TS 23.502 §4.11.1.2.1 step 2, TS 29.214 Annex A.13.2)", mcpttQCI, qos.ARP, mcpttNormalPriority, mcpttNormalARP))
	}

	if err := call.Bye(ctx); err != nil {
		return err
	}

	if _, err := as.AwaitRequest(ctx, "BYE", 1); err != nil {
		return err
	}

	if err := h.lte.awaitVoiceRelease(h.bearers[mcpttQCI].erab); err != nil {
		return fmt.Errorf("on EPS after the BYE: %w", err)
	}

	return nil
}

func requireMCPTTFlowOn5GS(a *nrAttachment, qfi uint8) error {
	flow, ok := a.g.DedicatedQoSFlow(a.ranUEID, int64(a.session.PDUSessionID), scenarios.MCPTT5QI)

	switch {
	case !ok || flow.QFI != qfi:
		return fmt.Errorf("the QoS flow QFI %d is not a 5QI %d flow (%+v), want speech and floor control on it (TS 23.379 §5.7.3, TS 23.289 §4.3.3.2)", qfi, scenarios.MCPTT5QI, flow)
	case !flow.GBR || flow.PriArp != int64(mcpttNormalARP.PriorityLevel) || flow.PreemptCap != ngap.PreemptionMayTrigger || flow.PreemptVuln != ngap.PreemptionPreemptable:
		return fmt.Errorf("the MCPTT QoS flow is %+v for %s, want GBR with ARP %+v (TS 29.513 §7.3.2 table 7.3.2-2, TS 29.214 Annex A.13.2)", flow, mcpttNormalPriority, mcpttNormalARP)
	}

	raw, err := a.u.WaitForNASGSMMessage(uint8(fgs.MsgPDUSessionModificationCommand), imsVoiceTimeout)
	if err != nil {
		return fmt.Errorf("no PDU Session Modification Command for the MCPTT flow: %w", err)
	}

	cmd, err := fgs.ParsePDUSessionModificationCommand(raw)
	if err != nil {
		return fmt.Errorf("parse the PDU Session Modification Command: %w", err)
	}

	desc, err := scenarios.RequireMCPTTQoSFlow(cmd, qfi)
	if err != nil {
		return err
	}

	ebi, ok := desc.EPSBearerID()
	if !ok || ebi != uint8(imsVoiceEBI) {
		return fmt.Errorf("the MCPTT QoS flow description carries EPS bearer identity %d (present %t), want EBI %d (TS 24.501 §6.1.4.1)", ebi, ok, imsVoiceEBI)
	}

	i := slices.IndexFunc(cmd.MappedEPSBearerContexts, func(c fgs.MappedEPSBearerContext) bool { return c.EPSBearerIdentity == ebi })
	if i < 0 {
		return fmt.Errorf("the PDU Session Modification Command maps EPS bearers %+v, want EBI %d for the MCPTT flow (TS 24.501 §6.1.4.1)", cmd.MappedEPSBearerContexts, ebi)
	}

	mapped := cmd.MappedEPSBearerContexts[i]

	j := slices.IndexFunc(mapped.Parameters, func(p fgs.EPSParameter) bool { return p.Identifier == fgs.EPSParameterMappedEPSQoS })
	if mapped.Operation != fgs.MappedEPSBearerOpCreate || j < 0 {
		return fmt.Errorf("the mapped EPS bearer context %d is %+v, want a new bearer with its EPS QoS", ebi, mapped)
	}

	qos, err := eps.ParseEPSQoS(mapped.Parameters[j].Contents)
	if err != nil {
		return fmt.Errorf("parse the mapped EPS QoS of EBI %d: %w", ebi, err)
	}

	rates, ok := qos.GBRBitRates()

	switch {
	case qos.QCI != mcpttQCI:
		return fmt.Errorf("the mapped EPS bearer %d has QCI %d, want %d (TS 23.379 §5.7.3, TS 23.289 §4.3.3.2)", ebi, qos.QCI, mcpttQCI)
	case !ok || rates.GuaranteedUplinkKbps == 0 || rates.GuaranteedDownlinkKbps == 0:
		return fmt.Errorf("the mapped EPS bearer %d has no guaranteed bit rate", ebi)
	}

	return nil
}
