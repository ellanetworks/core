// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"github.com/ellanetworks/ims/sip"
	"github.com/spf13/pflag"
)

const (
	mcpttTunIface    = "s1mcptttun"
	mcpttRouteTable  = 150
	mcpttCallTimeout = 90 * time.Second
	mcpttQuietPeriod = 3 * time.Second

	qciMCPTT = 65

	mcpttNormalPriority  = "mcpttp.11"
	mcpttUpgradePriority = "mcpttp.15"
)

var mcpttPreemptedCaller = scenarios.MCPTTSubscriber("001017400000024", "+15557400024")

type mcpttCallCase struct {
	name     string
	sub      scenarios.SubscriberSpec
	priority string
	arp      s1ap.AllocationAndRetentionPriority
	upgrade  bool
}

var mcpttCalls = []mcpttCallCase{
	{
		name: "ims/4g_mcptt_call", sub: scenarios.MCPTTSubscriber("001017400000021", "+15557400021"),
		priority: mcpttNormalPriority, arp: mcpttARP(3, true), upgrade: true,
	},
	{
		name: "ims/4g_mcptt_call_imminent_peril", sub: scenarios.MCPTTSubscriber("001017400000022", "+15557400022"),
		priority: "mcpttp.13", arp: mcpttARP(2, true),
	},
	{
		name: "ims/4g_mcptt_call_no_preemption", sub: scenarios.MCPTTSubscriber("001017400000023", "+15557400023"),
		priority: "mcpttq.11", arp: mcpttARP(3, false),
	},
}

func init() {
	for _, tc := range mcpttCalls {
		registerMCPTT(tc.name, tc.sub, func(ctx context.Context, env scenarios.Env) error {
			return runMCPTTCall(ctx, env, tc)
		})
	}

	registerMCPTT("ims/4g_mcptt_call_preempted", mcpttPreemptedCaller, runMCPTTCallPreempted)
}

func registerMCPTT(name string, sub scenarios.SubscriberSpec, run func(context.Context, scenarios.Env) error) {
	scenarios.Register(scenarios.Scenario{
		Name:      name,
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return run(ctx, env)
		},
		Fixture: func(env scenarios.Env) scenarios.FixtureSpec {
			return scenarios.MCPTTFixture(env, sub)
		},
	})
}

func mcpttARP(level uint8, preempts bool) s1ap.AllocationAndRetentionPriority {
	arp := s1ap.AllocationAndRetentionPriority{
		PriorityLevel:           level,
		PreemptionCapability:    s1ap.PreemptionShallNotTrigger,
		PreemptionVulnerability: s1ap.PreemptionPreemptable,
	}

	if preempts {
		arp.PreemptionCapability = s1ap.PreemptionMayTrigger
	}

	if level <= 2 {
		arp.PreemptionVulnerability = s1ap.PreemptionNotPreemptable
	}

	return arp
}

func attachMCPTTUE(env scenarios.Env, e *s1enb.ENB, sub scenarios.SubscriberSpec) (scenarios.IMSEndpoint, imsCallUE, func(), error) {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return scenarios.IMSEndpoint{}, imsCallUE{}, nil, err
	}

	att, err := attachAndTunnelIMSUE(env, e, sub, k, opc, mcpttTunIface, scenarios.MCDNN, scenarios.MCUEIPv4Pool, scenarios.MCUEIPv6Pool)
	if err != nil {
		return scenarios.IMSEndpoint{}, imsCallUE{}, nil, fmt.Errorf("UE %s: %w", sub.IMSI, err)
	}

	closeTunnel := func() { e.CloseTunnel(att.res.DLTEID) }

	pcscf, err := scenarios.SelectPCSCF(att.res.PCSCF, !env.HasIPv4())
	if err != nil {
		closeTunnel()
		return scenarios.IMSEndpoint{}, imsCallUE{}, nil, err
	}

	unroute, err := scenarios.RouteFromUE(mcpttTunIface, att.local, pcscf, mcpttRouteTable)
	if err != nil {
		closeTunnel()
		return scenarios.IMSEndpoint{}, imsCallUE{}, nil, err
	}

	cleanup := func() {
		unroute()
		closeTunnel()
	}

	ep := scenarios.IMSEndpoint{Subscriber: sub, PCSCF: pcscf, Local: att.local}

	return ep, imsCallUE{ue: att.ue, res: att.res, internet: att.internet}, cleanup, nil
}

type mcpttBearers struct {
	voice *s1enb.DedicatedBearer
}

func (b mcpttBearers) dedicated() []*s1enb.DedicatedBearer {
	return []*s1enb.DedicatedBearer{b.voice}
}

type mcpttENB struct {
	e *s1enb.ENB
	u imsCallUE

	priority string
	arp      s1ap.AllocationAndRetentionPriority
	upgrade  bool

	setup    chan error
	upgraded chan error
	released chan error
}

func newMCPTTENB(s *mcpttSession, priority string, arp s1ap.AllocationAndRetentionPriority, upgrade bool) *mcpttENB {
	return &mcpttENB{
		e: s.e, u: s.u, priority: priority, arp: arp, upgrade: upgrade,
		setup: make(chan error, 1), upgraded: make(chan error, 1), released: make(chan error, 1),
	}
}

func (m *mcpttENB) run() {
	b, err := m.acceptBearers()

	m.setup <- err

	if err != nil {
		m.upgraded <- errors.New("no MCPTT bearers to upgrade")

		m.released <- errors.New("no MCPTT bearers to release")

		return
	}

	if m.upgrade {
		err = m.awaitUpgrade(b)

		m.upgraded <- err

		if err != nil {
			m.released <- errors.New("the MCPTT bearers were not upgraded")
			return
		}
	}

	m.released <- m.awaitRelease(b)
}

func (m *mcpttENB) acceptBearers() (mcpttBearers, error) {
	d, err := m.e.AcceptDedicatedBearer(m.u.ue, m.u.res.ENBUES1APID, voiceBearerTimeout)
	if err != nil {
		return mcpttBearers{}, fmt.Errorf("UE %s MCPTT dedicated bearer: %w", m.u.ue.IMSI, err)
	}

	return mcpttBearers{voice: d}, m.checkVoice(d)
}

func (m *mcpttENB) checkVoice(b *s1enb.DedicatedBearer) error {
	if b == nil {
		return fmt.Errorf("UE %s has no QCI %d MCPTT bearer", m.u.ue.IMSI, qciMCPTT)
	}

	rates, ok := b.EPSQoS.GBRBitRates()

	switch {
	case b.QoS.QCI != qciMCPTT || b.EPSQoS.QCI != qciMCPTT:
		return fmt.Errorf("UE %s MCPTT bearer at QCI %d (EPS QoS %d), want %d for speech and floor control (TS 23.379 §5.7.3, TS 23.289 §4.3.3.2)", m.u.ue.IMSI, b.QoS.QCI, b.EPSQoS.QCI, qciMCPTT)
	case b.LinkedEBI != uint8(m.u.res.ERABID):
		return fmt.Errorf("UE %s MCPTT speech bearer linked to EBI %d, want the %s default bearer %d", m.u.ue.IMSI, b.LinkedEBI, scenarios.MCDNN, m.u.res.ERABID)
	case b.QoS.GBR == nil || b.QoS.GBR.GuaranteedBitrateDL == 0 || b.QoS.GBR.GuaranteedBitrateUL == 0:
		return fmt.Errorf("UE %s MCPTT speech E-RAB has no guaranteed bit rate: %+v", m.u.ue.IMSI, b.QoS.GBR)
	case !ok || rates.GuaranteedDownlinkKbps == 0 || rates.GuaranteedUplinkKbps == 0:
		return fmt.Errorf("UE %s MCPTT speech EPS QoS has no guaranteed bit rate", m.u.ue.IMSI)
	case b.QoS.ARP != m.arp:
		return fmt.Errorf("UE %s MCPTT speech E-RAB ARP %+v for %s, want %+v (TS 29.214 Annex A.13.2)", m.u.ue.IMSI, b.QoS.ARP, m.priority, m.arp)
	case b.TFT.Operation != eps.TFTCreate || len(b.TFT.Filters) == 0:
		return fmt.Errorf("UE %s MCPTT speech bearer TFT %+v, want new packet filters", m.u.ue.IMSI, b.TFT)
	case len(localPorts(b.TFT)) < 3:
		return fmt.Errorf("UE %s MCPTT bearer TFT local ports %v, want speech RTP, RTCP and floor control on the one QCI %d bearer", m.u.ue.IMSI, localPorts(b.TFT), qciMCPTT)
	}

	return nil
}

func (m *mcpttENB) awaitUpgrade(b mcpttBearers) error {
	want := mcpttARP(1, true)
	modified := map[s1ap.ERABID]bool{}

	for range b.dedicated() {
		req, _, err := m.e.ModifyBearerViaERABModify(m.u.ue, m.u.res.ENBUES1APID, voiceBearerTimeout)
		if err != nil {
			return fmt.Errorf("UE %s MCPTT bearers after the upgrade to %s: %w", m.u.ue.IMSI, mcpttUpgradePriority, err)
		}

		item := req.ERABToBeModified[0]

		i := slices.IndexFunc(b.dedicated(), func(d *s1enb.DedicatedBearer) bool { return d.ERABID == item.ERABID })

		switch {
		case i < 0:
			return fmt.Errorf("UE %s E-RAB Modify Request for E-RAB %d, want the MCPTT bearers %d modified in place", m.u.ue.IMSI, item.ERABID, erabIDs(b.dedicated()))
		case modified[item.ERABID]:
			return fmt.Errorf("UE %s E-RAB %d modified twice for the upgrade to %s", m.u.ue.IMSI, item.ERABID, mcpttUpgradePriority)
		case item.QoS.QCI != b.dedicated()[i].QoS.QCI:
			return fmt.Errorf("UE %s E-RAB %d modified to QCI %d, want its QCI %d kept", m.u.ue.IMSI, item.ERABID, item.QoS.QCI, b.dedicated()[i].QoS.QCI)
		case item.QoS.ARP != want:
			return fmt.Errorf("UE %s QCI %d E-RAB %d ARP %+v after the upgrade to %s, want %+v (TS 29.214 Annex A.13.2)", m.u.ue.IMSI, item.QoS.QCI, item.ERABID, item.QoS.ARP, mcpttUpgradePriority, want)
		case item.QoS.GBR == nil:
			return fmt.Errorf("UE %s MCPTT speech E-RAB %d lost its GBR QoS Information in the upgrade", m.u.ue.IMSI, item.ERABID)
		}

		modified[item.ERABID] = true
		b.dedicated()[i].QoS = item.QoS
	}

	return nil
}

func (m *mcpttENB) awaitRelease(b mcpttBearers) error {
	released := map[s1ap.ERABID]bool{}

	for range b.dedicated() {
		erab, deact, err := m.e.AwaitDedicatedDeactivation(m.u.ue, m.u.res.ENBUES1APID, voiceBearerTimeout)

		switch {
		case err != nil:
			return fmt.Errorf("UE %s MCPTT bearer release after BYE: %w", m.u.ue.IMSI, err)
		case !slices.ContainsFunc(b.dedicated(), func(d *s1enb.DedicatedBearer) bool { return d.ERABID == erab }):
			return fmt.Errorf("UE %s released E-RAB %d, want the MCPTT bearers %d", m.u.ue.IMSI, erab, erabIDs(b.dedicated()))
		case deact == nil || deact.Cause != eps.ESMCauseRegularDeactivation:
			return fmt.Errorf("UE %s MCPTT bearer %d deactivated with %+v, want cause #36", m.u.ue.IMSI, erab, deact)
		}

		released[erab] = true
	}

	if len(released) != len(b.dedicated()) {
		return fmt.Errorf("UE %s released E-RABs %v, want the MCPTT bearers %d", m.u.ue.IMSI, released, erabIDs(b.dedicated()))
	}

	return nil
}

func localPorts(t eps.TrafficFlowTemplate) []uint16 {
	var out []uint16

	for _, f := range t.Filters {
		for _, c := range f.Components {
			if c.Type != eps.TFTSingleLocalPort || len(c.Value) != 2 {
				continue
			}

			if port := uint16(c.Value[0])<<8 | uint16(c.Value[1]); !slices.Contains(out, port) {
				out = append(out, port)
			}
		}
	}

	return out
}

func erabIDs(bs []*s1enb.DedicatedBearer) []s1ap.ERABID {
	out := make([]s1ap.ERABID, len(bs))
	for i, b := range bs {
		out[i] = b.ERABID
	}

	return out
}

type mcpttSession struct {
	e     *s1enb.ENB
	u     imsCallUE
	as    *scenarios.MCPTTServer
	c     *scenarios.MCPTTClient
	close func()
}

func startMCPTTSession(ctx context.Context, env scenarios.Env, sub scenarios.SubscriberSpec) (*mcpttSession, error) {
	e, err := startENBWithDatapath(env)
	if err != nil {
		return nil, fmt.Errorf("start S1 eNB: %w", err)
	}

	cleanups := []func(){func() { _ = e.Close() }}
	s := &mcpttSession{e: e}
	s.close = func() {
		for _, c := range slices.Backward(cleanups) {
			c()
		}
	}

	fail := func(err error) (*mcpttSession, error) {
		s.close()
		return nil, err
	}

	ep, u, cleanup, err := attachMCPTTUE(env, e, sub)
	if err != nil {
		return fail(err)
	}

	cleanups = append(cleanups, cleanup)
	s.u = u

	if u.res.QCI != scenarios.MCPolicy5QI || u.res.ARP != scenarios.MCPolicyARP {
		return fail(fmt.Errorf("%s default bearer at QCI %d ARP %d, want QCI %d ARP %d", scenarios.MCDNN, u.res.QCI, u.res.ARP, scenarios.MCPolicy5QI, scenarios.MCPolicyARP))
	}

	as, err := scenarios.StartMCPTTServer(ctx, env)
	if err != nil {
		return fail(err)
	}

	cleanups = append(cleanups, as.Close)
	s.as = as

	unprovision, err := scenarios.ProvisionMCPTTService(ctx, env, scenarios.MCProfileName, scenarios.MCPTTSet, as.URI())
	if err != nil {
		return fail(err)
	}

	cleanups = append(cleanups, unprovision)

	c, err := scenarios.NewMCPTTClient(ep, sip.UDP)
	if err != nil {
		return fail(err)
	}

	cleanups = append(cleanups, c.Close)
	s.c = c

	if err := c.Register(ctx); err != nil {
		return fail(err)
	}

	if err := scenarios.RequireThirdPartyRegister(ctx, as, sub); err != nil {
		return fail(err)
	}

	return s, nil
}

func runMCPTTCall(ctx context.Context, env scenarios.Env, tc mcpttCallCase) error {
	ctx, cancel := context.WithTimeout(ctx, mcpttCallTimeout)
	defer cancel()

	s, err := startMCPTTSession(ctx, env, tc.sub)
	if err != nil {
		return err
	}

	defer s.close()

	m := newMCPTTENB(s, tc.priority, tc.arp, tc.upgrade)

	go m.run()

	call, err := s.c.Call(ctx, scenarios.MCPTTPSI("grp-1"), tc.priority)
	if err != nil {
		return errors.Join(err, <-m.setup)
	}

	callErr := mcpttCallSteps(ctx, s.as, call, m)

	if err := s.c.Deregister(ctx); err != nil {
		callErr = errors.Join(callErr, err)
	}

	return callErr
}

func mcpttCallSteps(ctx context.Context, as *scenarios.MCPTTServer, call *scenarios.MCPTTCall, m *mcpttENB) error {
	hangUp := func(err error) error {
		return errors.Join(err, call.Bye(ctx))
	}

	if err := scenarios.RequireMCPTTInvite(ctx, as, 1, m.priority); err != nil {
		return hangUp(err)
	}

	if err := scenarios.RequireAuthorisedPriority(call.Response(), m.priority); err != nil {
		return hangUp(fmt.Errorf("200 to the INVITE: %w", err))
	}

	if err := <-m.setup; err != nil {
		return hangUp(err)
	}

	if m.upgrade {
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

		if err := <-m.upgraded; err != nil {
			return hangUp(err)
		}
	}

	if err := call.Bye(ctx); err != nil {
		return err
	}

	if _, err := as.AwaitRequest(ctx, "BYE", 1); err != nil {
		return err
	}

	return <-m.released
}

func runMCPTTCallPreempted(ctx context.Context, env scenarios.Env) error {
	ctx, cancel := context.WithTimeout(ctx, mcpttCallTimeout)
	defer cancel()

	s, err := startMCPTTSession(ctx, env, mcpttPreemptedCaller)
	if err != nil {
		return err
	}

	defer s.close()

	m := newMCPTTENB(s, mcpttNormalPriority, mcpttARP(3, true), false)
	accepted := make(chan mcpttBearers, 1)

	go func() {
		b, err := m.acceptBearers()
		m.setup <- err

		accepted <- b
	}()

	call, err := s.c.Call(ctx, scenarios.MCPTTPSI("grp-1"), mcpttNormalPriority)
	if err != nil {
		return errors.Join(err, <-m.setup)
	}

	callErr := mcpttPreemptionSteps(ctx, s, call, m, accepted)

	if err := s.c.Deregister(ctx); err != nil {
		callErr = errors.Join(callErr, err)
	}

	return callErr
}

func mcpttPreemptionSteps(ctx context.Context, s *mcpttSession, call *scenarios.MCPTTCall, m *mcpttENB, accepted <-chan mcpttBearers) error {
	hangUp := func(err error) error {
		return errors.Join(err, call.Bye(ctx))
	}

	if err := scenarios.RequireMCPTTInvite(ctx, s.as, 1, m.priority); err != nil {
		return hangUp(err)
	}

	if err := <-m.setup; err != nil {
		return hangUp(err)
	}

	b := <-accepted

	if err := s.e.SendERABReleaseIndication(s.u.res.MMEUES1APID, s.u.res.ENBUES1APID, b.voice.ERABID, s1enb.CauseReleaseDueToPreemption); err != nil {
		return hangUp(err)
	}

	bye, err := s.as.AwaitRequest(ctx, "BYE", 1)
	if err != nil {
		return hangUp(fmt.Errorf("the call outlived its pre-empted MCPTT bearer, want the P-CSCF to release it (TS 24.229 §5.2.8.1.2): %w", err))
	}

	if err := requireBearerLossReason(bye); err != nil {
		return err
	}

	frame, err := s.e.WaitForAnyMessage(s.u.res.ENBUES1APID, s1enb.Initiating, []s1ap.ProcedureCode{s1ap.ProcERABSetup, s1ap.ProcERABModify, s1ap.ProcERABRelease}, mcpttQuietPeriod)
	if err == nil {
		return fmt.Errorf("UE %s got %s after the eNB pre-empted its MCPTT E-RAB %d, want no E-RAB signalling: the release ends at the core (TS 23.401 §5.4.4.2 step 7) and the PCF does not request the rule again", m.u.ue.IMSI, frame.ProcedureCode, b.voice.ERABID)
	}

	select {
	case <-call.Done():
		return fmt.Errorf("UE %s got a BYE for the pre-empted call, want the P-CSCF to release only toward the called party (TS 24.229 §5.2.8.1.2)", m.u.ue.IMSI)
	default:
	}

	return nil
}

func requireBearerLossReason(bye *sip.Request) error {
	reasons, err := bye.Header.Reasons()
	if err != nil {
		return fmt.Errorf("the BYE at the MCPTT server has a malformed Reason %q: %w", bye.Header.Values("Reason"), err)
	}

	if !slices.ContainsFunc(reasons, func(r sip.Reason) bool {
		cause, ok := r.Cause()
		return ok && r.Is("SIP") && cause == 503
	}) {
		return fmt.Errorf("the BYE at the MCPTT server has Reason %q, want SIP cause 503 for the lost bearer (TS 24.229 §5.2.8.1.2)", bye.Header.Values("Reason"))
	}

	return nil
}
