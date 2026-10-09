// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
)

var (
	imsHandoverTo4GCaller      = scenarios.IMSSubscriber("001018600000001", "+15558600001")
	imsHandoverTo4GCallee      = scenarios.IMSSubscriber("001018600000002", "+15558600002")
	imsHandoverTo5GCaller      = scenarios.IMSSubscriber("001018600000003", "+15558600003")
	imsHandoverTo5GCallee      = scenarios.IMSSubscriber("001018600000004", "+15558600004")
	imsIdleCaller              = scenarios.IMSSubscriber("001018600000005", "+15558600005")
	imsIdleCallee              = scenarios.IMSSubscriber("001018600000006", "+15558600006")
	imsRefusedBy4GCaller       = scenarios.IMSSubscriber("001018600000007", "+15558600007")
	imsRefusedBy4GCallee       = scenarios.IMSSubscriber("001018600000008", "+15558600008")
	imsRefusedBy5GCaller       = scenarios.IMSSubscriber("001018600000009", "+15558600009")
	imsRefusedBy5GCallee       = scenarios.IMSSubscriber("001018600000010", "+15558600010")
	imsFallbackHandoverCaller  = scenarios.IMSSubscriber("001018600000011", "+15558600011")
	imsFallbackHandoverCallee  = scenarios.IMSSubscriber("001018600000012", "+15558600012")
	imsFallbackRedirectCaller  = scenarios.IMSSubscriber("001018600000013", "+15558600013")
	imsFallbackRedirectCallee  = scenarios.IMSSubscriber("001018600000014", "+15558600014")
	imsEPSFallbackCause        = ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkIMSVoiceEPSFallbackTriggered}
	imsRedirectionCause        = ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkRedirection}
	imsFallbackRequestDeadline = 10 * time.Second
)

func init() {
	registerIMS("ims/5g_call_handover_to_4g_and_back", runIMSCallHandoverTo4GAndBack, imsHandoverTo4GCaller, imsHandoverTo4GCallee)
	registerIMS("ims/4g_call_handover_to_5g_and_back", runIMSCallHandoverTo5GAndBack, imsHandoverTo5GCaller, imsHandoverTo5GCallee)
	registerIMS("ims/5g_call_idle_to_4g_and_back", runIMSCallIdleTo4GAndBack, imsIdleCaller, imsIdleCallee)
	registerIMS("ims/5g_call_handover_to_4g_voice_refused", runIMSCallVoiceRefusedBy4G, imsRefusedBy4GCaller, imsRefusedBy4GCallee)
	registerIMS("ims/4g_call_handover_to_5g_voice_refused", runIMSCallVoiceRefusedBy5G, imsRefusedBy5GCaller, imsRefusedBy5GCallee)
	registerIMS("ims/5g_call_eps_fallback_handover", func(ctx context.Context, env scenarios.Env) error {
		return runIMSCallEPSFallback(ctx, env, imsFallbackHandoverCaller, imsFallbackHandoverCallee, fallBackByHandover)
	}, imsFallbackHandoverCaller, imsFallbackHandoverCallee)
	registerIMS("ims/5g_call_eps_fallback_redirection", func(ctx context.Context, env scenarios.Env) error {
		return runIMSCallEPSFallback(ctx, env, imsFallbackRedirectCaller, imsFallbackRedirectCallee, fallBackByRedirection)
	}, imsFallbackRedirectCaller, imsFallbackRedirectCallee)
}

type imsCallRANs struct {
	g *gnb.GnodeB
	e *s1enb.ENB
}

func startIMSCallRANs(env scenarios.Env) (*imsCallRANs, error) {
	g, err := startGNB(env)
	if err != nil {
		return nil, err
	}

	e, err := startENBOnSecondaryN3(env)
	if err != nil {
		g.Close()
		return nil, err
	}

	return &imsCallRANs{g: g, e: e}, nil
}

func (r *imsCallRANs) close() {
	_ = r.e.Close()
	r.g.Close()
}

func (r *imsCallRANs) partiesOn5GS(env scenarios.Env, caller, callee scenarios.SubscriberSpec) (*imsParty, *imsParty, error) {
	a, err := registerIMSPartyOn5GS(env, r.g, caller, imsFirstGNBRANUEID, 0)
	if err != nil {
		return nil, nil, err
	}

	b, err := registerIMSPartyOn5GS(env, r.g, callee, imsFirstGNBRANUEID+1, 1)
	if err != nil {
		a.close()
		return nil, nil, err
	}

	return a, b, nil
}

func (r *imsCallRANs) partiesOn4G(env scenarios.Env, caller, callee scenarios.SubscriberSpec) (*imsParty, *imsParty, error) {
	a, err := attachIMSPartyOn4G(env, r.e, caller, 0)
	if err != nil {
		return nil, nil, err
	}

	b, err := attachIMSPartyOn4G(env, r.e, callee, 1)
	if err != nil {
		a.close()
		return nil, nil, err
	}

	return a, b, nil
}

func awaitQoSFlowsReleased(ctx context.Context, attachments ...*nrAttachment) error {
	ctx, cancel := context.WithTimeout(ctx, imsVoiceTimeout)
	defer cancel()

	for {
		held := 0
		for _, a := range attachments {
			held += len(a.g.QoSFlows(a.ranUEID, int64(imsPDUSessionID)))
		}

		if held == 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("the gNB still holds %d voice QoS flows after the call: %w", held, ctx.Err())
		case <-time.After(imsPacketInterval):
		}
	}
}

func voiceBearerOn4G(ctx context.Context, ch <-chan acceptedBearer) (*s1enb.DedicatedBearer, error) {
	b, err := awaitBearer(ctx, ch)
	if err != nil {
		return nil, err
	}

	if b.ERABID != imsVoiceEBI {
		return nil, fmt.Errorf("the voice bearer is E-RAB %d, want EBI %d", b.ERABID, imsVoiceEBI)
	}

	return b, nil
}

func runIMSCallHandoverTo4GAndBack(ctx context.Context, env scenarios.Env) error {
	r, err := startIMSCallRANs(env)
	if err != nil {
		return err
	}

	defer r.close()

	caller, callee, err := r.partiesOn5GS(env, imsHandoverTo4GCaller, imsHandoverTo4GCallee)
	if err != nil {
		return err
	}

	defer caller.close()
	defer callee.close()

	media := func(ctx context.Context, m scenarios.IMSMedia) error {
		callerQFI, err := caller.nr.awaitVoiceQFI(ctx)
		if err != nil {
			return err
		}

		calleeQFI, err := callee.nr.awaitVoiceQFI(ctx)
		if err != nil {
			return err
		}

		callerVoice := nrVoice{a: caller.nr, qfi: callerQFI}

		if err := sendVoiceBothWays(ctx, callerVoice, nrVoice{a: callee.nr, qfi: calleeQFI}, m); err != nil {
			return fmt.Errorf("on 5GS: %w", err)
		}

		onEPS, err := callee.handOverToEPS(r.e, true, false)
		if err != nil {
			return fmt.Errorf("handover to EPS: %w", err)
		}

		if err := sendVoiceBothWays(ctx, callerVoice, *onEPS.voice, m); err != nil {
			return fmt.Errorf("after the handover to EPS: %w", err)
		}

		back, err := callee.handOverTo5GS(r.g, imsMovedGNBRANUEIDs, calleeQFI, false)
		if err != nil {
			return fmt.Errorf("handover back to 5GS: %w", err)
		}

		if err := sendVoiceBothWays(ctx, callerVoice, *back.voice, m); err != nil {
			return fmt.Errorf("back on 5GS: %w", err)
		}

		return nil
	}

	if err := scenarios.RequireIMSCallKept(ctx, caller.ep, callee.ep, scenarios.IMSTransports[0], media); err != nil {
		return err
	}

	return awaitQoSFlowsReleased(ctx, caller.nr, callee.nr)
}

func runIMSCallHandoverTo5GAndBack(ctx context.Context, env scenarios.Env) error {
	r, err := startIMSCallRANs(env)
	if err != nil {
		return err
	}

	defer r.close()

	caller, callee, err := r.partiesOn4G(env, imsHandoverTo5GCaller, imsHandoverTo5GCallee)
	if err != nil {
		return err
	}

	defer caller.close()
	defer callee.close()

	callerBearer, calleeBearer := caller.lte.acceptVoiceBearer(), callee.lte.acceptVoiceBearer()
	calleeReleased := make(chan error, 1)

	var callerVoice lteVoice

	media := func(ctx context.Context, m scenarios.IMSMedia) error {
		cb, err := voiceBearerOn4G(ctx, callerBearer)
		if err != nil {
			return fmt.Errorf("caller: %w", err)
		}

		kb, err := voiceBearerOn4G(ctx, calleeBearer)
		if err != nil {
			return fmt.Errorf("callee: %w", err)
		}

		callerVoice = caller.lte.voice(cb)

		if err := sendVoiceBothWays(ctx, callerVoice, callee.lte.voice(kb), m); err != nil {
			return fmt.Errorf("on EPS: %w", err)
		}

		on5GS, err := callee.handOverTo5GS(r.g, imsMovedGNBRANUEIDs, 0, false)
		if err != nil {
			return fmt.Errorf("handover to 5GS: %w", err)
		}

		if err := sendVoiceBothWays(ctx, callerVoice, *on5GS.voice, m); err != nil {
			return fmt.Errorf("after the handover to 5GS: %w", err)
		}

		back, err := callee.handOverToEPS(r.e, true, false)
		if err != nil {
			return fmt.Errorf("handover back to EPS: %w", err)
		}

		go func() { calleeReleased <- back.lte.awaitVoiceRelease(imsVoiceEBI) }()

		if err := sendVoiceBothWays(ctx, callerVoice, *back.voice, m); err != nil {
			return fmt.Errorf("back on EPS: %w", err)
		}

		return nil
	}

	if err := scenarios.RequireIMSCallKept(ctx, caller.ep, callee.ep, scenarios.IMSTransports[0], media); err != nil {
		return err
	}

	if err := caller.lte.awaitVoiceRelease(callerVoice.erab); err != nil {
		return fmt.Errorf("caller: %w", err)
	}

	if err := <-calleeReleased; err != nil {
		return fmt.Errorf("callee: %w", err)
	}

	return nil
}

func runIMSCallIdleTo4GAndBack(ctx context.Context, env scenarios.Env) error {
	r, err := startIMSCallRANs(env)
	if err != nil {
		return err
	}

	defer r.close()

	caller, callee, err := r.partiesOn5GS(env, imsIdleCaller, imsIdleCallee)
	if err != nil {
		return err
	}

	defer caller.close()
	defer callee.close()

	media := func(ctx context.Context, m scenarios.IMSMedia) error {
		callerQFI, err := caller.nr.awaitVoiceQFI(ctx)
		if err != nil {
			return err
		}

		calleeQFI, err := callee.nr.awaitVoiceQFI(ctx)
		if err != nil {
			return err
		}

		callerVoice := nrVoice{a: caller.nr, qfi: callerQFI}

		if err := sendVoiceBothWays(ctx, callerVoice, nrVoice{a: callee.nr, qfi: calleeQFI}, m); err != nil {
			return fmt.Errorf("on 5GS: %w", err)
		}

		tau, epsUE, err := callee.idleToEPS(r.e)
		if err != nil {
			return err
		}

		back, err := callee.idleTo5GS(ctx, r.g, epsUE, tau, imsMovedGNBRANUEIDs)
		if err != nil {
			return err
		}

		if back.voice.qfi != calleeQFI {
			return fmt.Errorf("the voice flow came back as QFI %d, want QFI %d kept across the idle round trip (TS 23.502 §4.11.1.1)", back.voice.qfi, calleeQFI)
		}

		if err := sendVoiceBothWays(ctx, callerVoice, *back.voice, m); err != nil {
			return fmt.Errorf("back on 5GS: %w", err)
		}

		return nil
	}

	if err := scenarios.RequireIMSCallKept(ctx, caller.ep, callee.ep, scenarios.IMSTransports[0], media); err != nil {
		return err
	}

	return awaitQoSFlowsReleased(ctx, caller.nr, callee.nr)
}

func voiceBearerStatus() *nas.EPSBearerContextStatus {
	var status nas.EPSBearerContextStatus

	status.Active[imsDefaultEBI] = true
	status.Active[imsVoiceEBI] = true

	return &status
}

func (p *imsParty) idleToEPS(e *s1enb.ENB) (*s1enb.AttachResult, *s1enb.UE, error) {
	nr := p.nr

	if err := nr.g.ReleaseContext(nr.u, nr.ranUEID, []uint8{imsPDUSessionID}, gnb.CauseUserInactivity, releaseTimeout); err != nil {
		return nil, nil, fmt.Errorf("release the NR connection over user inactivity: %w", err)
	}

	security, guti, err := idleMobilityMaterial(nr.u)
	if err != nil {
		return nil, nil, err
	}

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return nil, nil, err
	}

	epsUE := e.NewUE(p.sub.IMSI, k, opc)

	tau, err := e.TrackingAreaUpdateFrom5GS(epsUE, s1enb.IdleTrackingAreaUpdateOpts{
		GUTI:         guti,
		UpdateType:   eps.EPSUpdateTypeTA,
		BearerStatus: voiceBearerStatus(),
		Security:     security,
	}, attachTimeout)
	if err != nil {
		return nil, nil, fmt.Errorf("tracking area update after the idle move to EPS: %w", err)
	}

	switch {
	case tau.BearerStatus == nil:
		return nil, nil, errors.New("the tracking area update accept carries no EPS bearer context status")
	case !tau.BearerStatus.Active[imsDefaultEBI] || !tau.BearerStatus.Active[imsVoiceEBI]:
		return nil, nil, fmt.Errorf("EPS bearer context status = %v, want the default bearer %d and the voice bearer %d active (TS 23.502 §4.11.1.3.2 step 5c)",
			tau.BearerStatus.Active, imsDefaultEBI, imsVoiceEBI)
	case tau.GUTI == nil || tau.GUTI.GUTI == nil:
		return nil, nil, errors.New("the tracking area update accept assigned no GUTI")
	}

	return tau, epsUE, nil
}

func (p *imsParty) idleTo5GS(ctx context.Context, g *gnb.GnodeB, epsUE *s1enb.UE, tau *s1enb.AttachResult, ranUEID int64) (fiveGSHandover, error) {
	u := p.nr.u
	status := voiceBearerStatus()

	container, err := epsUE.BuildTrackingAreaUpdateForContainer(*tau.GUTI.GUTI, status)
	if err != nil {
		return fiveGSHandover{}, err
	}

	var sessions [16]bool

	sessions[imsPDUSessionID] = true

	g.AddUE(ranUEID, u)

	if err := u.SendIdleMobilityRegistration(ue.IdleRegistrationOpts{
		RANUENGAPID:            ranUEID,
		MappedGUTI:             fgs.GUTIIdentity(etsi.MapGUTIEPSTo5G(*tau.GUTI.GUTI)),
		EPSNASMessageContainer: container,
		PDUSessionStatus:       &sessions,
		UplinkDataStatus:       &sessions,
		EPSBearerContextStatus: status,
	}); err != nil {
		return fiveGSHandover{}, fmt.Errorf("mobility registration back over NR: %w", err)
	}

	plain, err := u.WaitForNASGMMMessage(uint8(fgs.MsgRegistrationAccept), attachTimeout)
	if err != nil {
		return fiveGSHandover{}, fmt.Errorf("registration accept for the return to 5GS: %w", err)
	}

	accept, err := fgs.ParseRegistrationAccept(plain)
	if err != nil {
		return fiveGSHandover{}, fmt.Errorf("parse the registration accept: %w", err)
	}

	if s := accept.EPSBearerContextStatus; s == nil || !s.Active[imsVoiceEBI] {
		return fiveGSHandover{}, fmt.Errorf("the registration accept EPS bearer context status %+v omits the voice EBI %d, so the UE would delete its voice flow (TS 24.501 §5.5.1.3.4)", s, imsVoiceEBI)
	}

	session, err := awaitSession(ctx, g, ranUEID)
	if err != nil {
		return fiveGSHandover{}, err
	}

	nr := &nrAttachment{g: g, u: u, ranUEID: ranUEID, session: session}

	qfi, err := nr.awaitVoiceQFI(ctx)
	if err != nil {
		return fiveGSHandover{}, fmt.Errorf("after the return to 5GS: %w", err)
	}

	return fiveGSHandover{nr: nr, voice: &nrVoice{a: nr, qfi: qfi}}, p.rehome(nr, nil)
}

func awaitSession(ctx context.Context, g *gnb.GnodeB, ranUEID int64) (gnb.PDUSessionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, imsVoiceTimeout)
	defer cancel()

	for {
		if s, ok := g.PDUSession(ranUEID, imsPDUSessionID); ok && s.ULTEID != 0 {
			return s, nil
		}

		select {
		case <-ctx.Done():
			return gnb.PDUSessionResult{}, fmt.Errorf("the gNB holds no user plane for PDU session %d: %w", imsPDUSessionID, ctx.Err())
		case <-time.After(imsPacketInterval):
		}
	}
}

func runIMSCallVoiceRefusedBy4G(ctx context.Context, env scenarios.Env) error {
	r, err := startIMSCallRANs(env)
	if err != nil {
		return err
	}

	defer r.close()

	caller, callee, err := r.partiesOn5GS(env, imsRefusedBy4GCaller, imsRefusedBy4GCallee)
	if err != nil {
		return err
	}

	defer caller.close()
	defer callee.close()

	lose := func(ctx context.Context) error {
		if _, err := callee.nr.awaitVoiceQFI(ctx); err != nil {
			return err
		}

		_, err := callee.handOverToEPS(r.e, true, true)

		return err
	}

	if err := scenarios.RequireIMSCallLost(ctx, caller.ep, callee.ep, scenarios.IMSTransports[0], lose); err != nil {
		return err
	}

	return awaitQoSFlowsReleased(ctx, caller.nr)
}

func runIMSCallVoiceRefusedBy5G(ctx context.Context, env scenarios.Env) error {
	r, err := startIMSCallRANs(env)
	if err != nil {
		return err
	}

	defer r.close()

	caller, callee, err := r.partiesOn4G(env, imsRefusedBy5GCaller, imsRefusedBy5GCallee)
	if err != nil {
		return err
	}

	defer caller.close()
	defer callee.close()

	callerBearer, calleeBearer := caller.lte.acceptVoiceBearer(), callee.lte.acceptVoiceBearer()

	lose := func(ctx context.Context) error {
		if _, err := voiceBearerOn4G(ctx, calleeBearer); err != nil {
			return fmt.Errorf("callee: %w", err)
		}

		_, err := callee.handOverTo5GS(r.g, imsMovedGNBRANUEIDs, 0, true)

		return err
	}

	if err := scenarios.RequireIMSCallLost(ctx, caller.ep, callee.ep, scenarios.IMSTransports[0], lose); err != nil {
		return err
	}

	cb, err := voiceBearerOn4G(ctx, callerBearer)
	if err != nil {
		return fmt.Errorf("caller: %w", err)
	}

	if err := caller.lte.awaitVoiceRelease(cb.ERABID); err != nil {
		return fmt.Errorf("caller: %w", err)
	}

	return awaitQoSFlowsReleased(ctx, callee.nr)
}

type epsFallback func(ctx context.Context, p *imsParty, e *s1enb.ENB) (*lteAttachment, error)

func fallBackByHandover(_ context.Context, p *imsParty, e *s1enb.ENB) (*lteAttachment, error) {
	h, err := p.handOverToEPS(e, false, false)
	if err != nil {
		return nil, fmt.Errorf("EPS fallback by handover: %w", err)
	}

	return h.lte, nil
}

func fallBackByRedirection(_ context.Context, p *imsParty, e *s1enb.ENB) (*lteAttachment, error) {
	nr := p.nr

	if err := nr.g.ReleaseContext(nr.u, nr.ranUEID, []uint8{imsPDUSessionID}, imsRedirectionCause, releaseTimeout); err != nil {
		return nil, fmt.Errorf("release the NR connection for redirection: %w", err)
	}

	security, guti, err := idleMobilityMaterial(nr.u)
	if err != nil {
		return nil, err
	}

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return nil, err
	}

	epsUE := e.NewUE(p.sub.IMSI, k, opc)

	res, err := e.TrackingAreaUpdateFrom5GS(epsUE, s1enb.IdleTrackingAreaUpdateOpts{
		GUTI:         guti,
		UpdateType:   eps.EPSUpdateTypeTA,
		ActiveFlag:   true,
		BearerStatus: voiceBearerStatus(),
		Security:     security,
	}, attachTimeout)
	if err != nil {
		return nil, fmt.Errorf("tracking area update after the redirection to E-UTRAN: %w", err)
	}

	lte := &lteAttachment{
		e: e, u: epsUE, mmeUEID: res.MMEUES1APID, enbUEID: res.ENBUES1APID,
		upf: res.UpfAddress, ulTEID: res.ULTEID, dlTEID: res.DLTEID,
	}

	p.guti = res.GUTI

	return lte, p.rehome(nil, lte)
}

func awaitFlowRefused(ctx context.Context, a *nrAttachment, before int) error {
	ctx, cancel := context.WithTimeout(ctx, imsFallbackRequestDeadline)
	defer cancel()

	for a.g.ModifyRequestCount(a.ranUEID) == before {
		select {
		case <-ctx.Done():
			return fmt.Errorf("the gNB was never asked to set up the voice flow: %w", ctx.Err())
		case <-time.After(imsPacketInterval):
		}
	}

	if flows := a.g.QoSFlows(a.ranUEID, int64(imsPDUSessionID)); len(flows) != 0 {
		return fmt.Errorf("the gNB holds QoS flows %v after refusing the voice flow", flows)
	}

	return nil
}

func runIMSCallEPSFallback(ctx context.Context, env scenarios.Env, callerSub, calleeSub scenarios.SubscriberSpec, fallBack epsFallback) error {
	r, err := startIMSCallRANs(env)
	if err != nil {
		return err
	}

	defer r.close()

	caller, callee, err := r.partiesOn5GS(env, callerSub, calleeSub)
	if err != nil {
		return err
	}

	defer caller.close()
	defer callee.close()

	before := r.g.ModifyRequestCount(callee.nr.ranUEID)
	r.g.RefuseNextGBRQoSFlows(callee.nr.ranUEID, imsEPSFallbackCause)

	released := make(chan error, 1)

	media := func(ctx context.Context, m scenarios.IMSMedia) error {
		callerQFI, err := caller.nr.awaitVoiceQFI(ctx)
		if err != nil {
			return err
		}

		if err := awaitFlowRefused(ctx, callee.nr, before); err != nil {
			return err
		}

		lte, err := fallBack(ctx, callee, r.e)
		if err != nil {
			return err
		}

		b, err := voiceBearerOn4G(ctx, lte.acceptVoiceBearer())
		if err != nil {
			return fmt.Errorf("the voice bearer after the EPS fallback (TS 23.502 §4.13.6.1 step 7): %w", err)
		}

		go func() { released <- lte.awaitVoiceRelease(b.ERABID) }()

		if err := sendVoiceBothWays(ctx, nrVoice{a: caller.nr, qfi: callerQFI}, lte.voice(b), m); err != nil {
			return fmt.Errorf("after the EPS fallback: %w", err)
		}

		return nil
	}

	if err := scenarios.RequireIMSCallKept(ctx, caller.ep, callee.ep, scenarios.IMSTransports[0], media); err != nil {
		return err
	}

	if err := <-released; err != nil {
		return fmt.Errorf("callee: %w", err)
	}

	return awaitQoSFlowsReleased(ctx, caller.nr)
}
