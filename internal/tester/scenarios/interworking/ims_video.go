// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/s1ap"
	"github.com/ellanetworks/ims/sip/sdp"
)

const (
	imsVideoQCI = 2
	imsVoice5QI = int64(imsVoiceQCI)
	imsVideo5QI = int64(imsVideoQCI)
)

var (
	imsVideo5GCaller         = scenarios.IMSSubscriber("001018700000001", "+15558700001")
	imsVideo5GCallee         = scenarios.IMSSubscriber("001018700000002", "+15558700002")
	imsVideo4GCaller         = scenarios.IMSSubscriber("001018700000003", "+15558700003")
	imsVideo4GCallee         = scenarios.IMSSubscriber("001018700000004", "+15558700004")
	imsVideoRefused5GCaller  = scenarios.IMSSubscriber("001018700000005", "+15558700005")
	imsVideoRefused5GCallee  = scenarios.IMSSubscriber("001018700000006", "+15558700006")
	imsVideoRefused4GCaller  = scenarios.IMSSubscriber("001018700000007", "+15558700007")
	imsVideoRefused4GCallee  = scenarios.IMSSubscriber("001018700000008", "+15558700008")
	imsVideoLost5GCaller     = scenarios.IMSSubscriber("001018700000009", "+15558700009")
	imsVideoLost5GCallee     = scenarios.IMSSubscriber("001018700000010", "+15558700010")
	imsVideoLost4GCaller     = scenarios.IMSSubscriber("001018700000011", "+15558700011")
	imsVideoLost4GCallee     = scenarios.IMSSubscriber("001018700000012", "+15558700012")
	imsVideoHandoverCaller   = scenarios.IMSSubscriber("001018700000013", "+15558700013")
	imsVideoHandoverCallee   = scenarios.IMSSubscriber("001018700000014", "+15558700014")
	imsVideoFallbackCaller   = scenarios.IMSSubscriber("001018700000015", "+15558700015")
	imsVideoFallbackCallee   = scenarios.IMSSubscriber("001018700000016", "+15558700016")
	imsVideoFlowReleaseCause = s1enb.CauseRadioConnectionWithUELost
	imsVideoMediaKinds       = []string{sdp.Audio, sdp.Video}
	imsVideoTransport        = scenarios.IMSTransports[0]
	imsVideoRefusal          = ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkRadioResourcesNotAvailable}
)

func init() {
	registerIMS("ims/5g_video_call", runIMSVideoCall5G, imsVideo5GCaller, imsVideo5GCallee)
	registerIMS("ims/4g_video_call", runIMSVideoCall4G, imsVideo4GCaller, imsVideo4GCallee)
	registerIMS("ims/5g_video_refused", runIMSVideoRefused5G, imsVideoRefused5GCaller, imsVideoRefused5GCallee)
	registerIMS("ims/4g_video_refused", runIMSVideoRefused4G, imsVideoRefused4GCaller, imsVideoRefused4GCallee)
	registerIMS("ims/5g_video_lost", runIMSVideoLost5G, imsVideoLost5GCaller, imsVideoLost5GCallee)
	registerIMS("ims/4g_video_lost", runIMSVideoLost4G, imsVideoLost4GCaller, imsVideoLost4GCallee)
	registerIMS("ims/5g_video_call_handover_to_4g_and_back", runIMSVideoCallHandover, imsVideoHandoverCaller, imsVideoHandoverCallee)
	registerIMS("ims/5g_video_call_eps_fallback", runIMSVideoCallEPSFallback, imsVideoFallbackCaller, imsVideoFallbackCallee)
}

func (a *nrAttachment) awaitFlows(ctx context.Context, fiveQIs ...int64) (map[int64]uint8, error) {
	ctx, cancel := context.WithTimeout(ctx, imsVoiceTimeout)
	defer cancel()

	want := slices.Sorted(slices.Values(fiveQIs))

	for {
		got := make(map[int64]uint8)
		have := make([]int64, 0, len(want))

		for _, qfi := range a.g.QoSFlows(a.ranUEID, int64(imsPDUSessionID)) {
			fiveQI, ok := a.g.QoSFlowFiveQI(a.ranUEID, int64(imsPDUSessionID), qfi)
			if !ok {
				fiveQI = -1
			}

			got[fiveQI] = qfi
			have = append(have, fiveQI)
		}

		slices.Sort(have)

		if slices.Equal(have, want) {
			return got, nil
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("the gNB holds QoS flows of 5QIs %v, want %v: %w", have, want, ctx.Err())
		case <-time.After(imsPacketInterval):
		}
	}
}

func (a *nrAttachment) legs(qfis map[int64]uint8) map[int64]mediaLeg {
	out := make(map[int64]mediaLeg, len(qfis))
	for fiveQI, qfi := range qfis {
		out[fiveQI] = nrFlow{a: a, qfi: qfi}
	}

	return out
}

func requireSameFlows(before, after map[int64]uint8, fiveQIs ...int64) error {
	for _, fiveQI := range fiveQIs {
		if before[fiveQI] != after[fiveQI] {
			return fmt.Errorf("the 5QI %d flow moved from QFI %d to %d, want it kept", fiveQI, before[fiveQI], after[fiveQI])
		}
	}

	return nil
}

type bearerStep struct {
	accept  int
	reject  uint8
	release []uint8
}

type bearerState struct {
	held map[uint8]lteBearer
	err  error
}

func (a *lteAttachment) runBearers(steps ...bearerStep) <-chan bearerState {
	out := make(chan bearerState, len(steps))

	go func() {
		held := make(map[uint8]lteBearer)

		for _, st := range steps {
			err := a.runBearerStep(st, held)
			out <- bearerState{held: maps.Clone(held), err: err}

			if err != nil {
				return
			}
		}
	}()

	return out
}

func (a *lteAttachment) runBearerStep(st bearerStep, held map[uint8]lteBearer) error {
	switch {
	case st.reject != 0:
		erab, err := a.e.RejectDedicatedBearer(a.u, a.enbUEID, imsVoiceTimeout)
		if err == nil {
			held[st.reject] = lteBearer{a: a, erab: erab}
		}

		return err
	case st.accept > 0:
		for range st.accept {
			b, err := a.e.AcceptDedicatedBearer(a.u, a.enbUEID, imsVoiceTimeout)
			if err != nil {
				return err
			}

			if b.QoS.GBR == nil {
				return fmt.Errorf("the QCI %d bearer has no GBR", b.QoS.QCI)
			}

			held[uint8(b.QoS.QCI)] = a.voice(b)
		}
	default:
		for range st.release {
			erab, _, err := a.e.AwaitDedicatedDeactivation(a.u, a.enbUEID, imsVoiceTimeout)
			if err != nil {
				return err
			}

			qci := slices.IndexFunc(st.release, func(q uint8) bool { return held[q].erab == erab })
			if qci < 0 {
				return fmt.Errorf("released E-RAB %d, want the bearers of QCIs %v", erab, st.release)
			}

			delete(held, st.release[qci])
		}
	}

	return nil
}

func awaitBearers(ctx context.Context, ch <-chan bearerState, qcis ...uint8) (map[uint8]lteBearer, error) {
	select {
	case s, ok := <-ch:
		switch {
		case !ok:
			return nil, fmt.Errorf("the bearer script ended")
		case s.err != nil:
			return nil, s.err
		}

		for _, qci := range qcis {
			if _, ok := s.held[qci]; !ok {
				return nil, fmt.Errorf("the UE holds bearers of QCIs %v, want QCI %d", slices.Sorted(maps.Keys(s.held)), qci)
			}
		}

		return s.held, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("the bearers did not change: %w", ctx.Err())
	}
}

func lteLegs(held map[uint8]lteBearer) map[int64]mediaLeg {
	out := make(map[int64]mediaLeg, len(held))
	for qci, b := range held {
		out[int64(qci)] = b
	}

	return out
}

func exchangeMedia(ctx context.Context, call scenarios.IMSCall, caller, callee map[int64]mediaLeg, kinds ...string) error {
	for _, kind := range kinds {
		m, err := call.Media(kind)
		if err != nil {
			return err
		}

		fiveQI := imsVoice5QI
		if kind == sdp.Video {
			fiveQI = imsVideo5QI
		}

		from, to := caller[fiveQI], callee[fiveQI]
		if from == nil || to == nil {
			return fmt.Errorf("no %s bearer on both sides", kind)
		}

		if err := sendVoiceBothWays(ctx, from, to, m); err != nil {
			return fmt.Errorf("%s: %w", kind, err)
		}

		rtcp := scenarios.IMSMedia{Caller: scenarios.RTCPEndpoint(m.Caller), Callee: scenarios.RTCPEndpoint(m.Callee)}
		if err := sendVoiceBothWays(ctx, from, to, rtcp); err != nil {
			return fmt.Errorf("%s RTCP: %w", kind, err)
		}
	}

	return nil
}

func runIMSVideoCall5G(ctx context.Context, env scenarios.Env) error {
	r, err := startIMSCallRANs(env)
	if err != nil {
		return err
	}

	defer r.close()

	caller, callee, err := r.partiesOn5GS(env, imsVideo5GCaller, imsVideo5GCallee)
	if err != nil {
		return err
	}

	defer caller.close()
	defer callee.close()

	script := func(ctx context.Context, call scenarios.IMSCall) error {
		a, err := caller.nr.awaitFlows(ctx, imsVoice5QI, imsVideo5QI)
		if err != nil {
			return fmt.Errorf("caller: %w", err)
		}

		b, err := callee.nr.awaitFlows(ctx, imsVoice5QI, imsVideo5QI)
		if err != nil {
			return fmt.Errorf("callee: %w", err)
		}

		if err := exchangeMedia(ctx, call, caller.nr.legs(a), callee.nr.legs(b), imsVideoMediaKinds...); err != nil {
			return fmt.Errorf("video call (IR.94 §4.2.1; NG.114 §4.5.3): %w", err)
		}

		if err := holdVideoFlows(ctx, caller.nr, callee.nr); err != nil {
			return err
		}

		if err := call.Caller.RemoveVideo(ctx); err != nil {
			return fmt.Errorf("remove video: %w", err)
		}

		for _, p := range []struct {
			name   string
			a      *nrAttachment
			before map[int64]uint8
		}{{"caller", caller.nr, a}, {"callee", callee.nr, b}} {
			now, err := p.a.awaitFlows(ctx, imsVoice5QI)
			if err != nil {
				return fmt.Errorf("%s after removing video (TS 29.214 §5.3.11): %w", p.name, err)
			}

			if err := requireSameFlows(p.before, now, imsVoice5QI); err != nil {
				return fmt.Errorf("%s: %w", p.name, err)
			}
		}

		if err := exchangeMedia(ctx, call, caller.nr.legs(a), callee.nr.legs(b), sdp.Audio); err != nil {
			return fmt.Errorf("voice only: %w", err)
		}

		if err := call.Caller.AddVideo(ctx); err != nil {
			return fmt.Errorf("add video: %w", err)
		}

		a2, err := caller.nr.awaitFlows(ctx, imsVoice5QI, imsVideo5QI)
		if err != nil {
			return fmt.Errorf("caller after adding video: %w", err)
		}

		b2, err := callee.nr.awaitFlows(ctx, imsVoice5QI, imsVideo5QI)
		if err != nil {
			return fmt.Errorf("callee after adding video: %w", err)
		}

		if err := requireSameFlows(a, a2, imsVoice5QI); err != nil {
			return fmt.Errorf("caller: %w", err)
		}

		if err := requireSameFlows(b, b2, imsVoice5QI); err != nil {
			return fmt.Errorf("callee: %w", err)
		}

		return exchangeMedia(ctx, call, caller.nr.legs(a2), callee.nr.legs(b2), imsVideoMediaKinds...)
	}

	if err := scenarios.RequireIMSVideoCall(ctx, caller.ep, callee.ep, imsVideoTransport, true, script); err != nil {
		return err
	}

	return awaitQoSFlowsReleased(ctx, caller.nr, callee.nr)
}

func videoCall4GSteps() []bearerStep {
	return []bearerStep{{accept: 2}, {release: []uint8{imsVideoQCI}}, {accept: 1}, {release: []uint8{imsVoiceQCI, imsVideoQCI}}}
}

func runIMSVideoCall4G(ctx context.Context, env scenarios.Env) error {
	r, err := startIMSCallRANs(env)
	if err != nil {
		return err
	}

	defer r.close()

	caller, callee, err := r.partiesOn4G(env, imsVideo4GCaller, imsVideo4GCallee)
	if err != nil {
		return err
	}

	defer caller.close()
	defer callee.close()

	ac, bc := caller.lte.runBearers(videoCall4GSteps()...), callee.lte.runBearers(videoCall4GSteps()...)

	script := func(ctx context.Context, call scenarios.IMSCall) error {
		a, err := awaitBearers(ctx, ac, imsVoiceQCI, imsVideoQCI)
		if err != nil {
			return fmt.Errorf("caller: %w", err)
		}

		b, err := awaitBearers(ctx, bc, imsVoiceQCI, imsVideoQCI)
		if err != nil {
			return fmt.Errorf("callee: %w", err)
		}

		if err := exchangeMedia(ctx, call, lteLegs(a), lteLegs(b), imsVideoMediaKinds...); err != nil {
			return fmt.Errorf("video call (IR.94 §4.2.1): %w", err)
		}

		if err := holdVideoBearers(ctx, ac, bc); err != nil {
			return err
		}

		if err := call.Caller.RemoveVideo(ctx); err != nil {
			return fmt.Errorf("remove video: %w", err)
		}

		if _, err := awaitBearers(ctx, ac, imsVoiceQCI); err != nil {
			return fmt.Errorf("caller after removing video (TS 23.401 §5.4.4.1): %w", err)
		}

		if _, err := awaitBearers(ctx, bc, imsVoiceQCI); err != nil {
			return fmt.Errorf("callee after removing video (TS 23.401 §5.4.4.1): %w", err)
		}

		if err := exchangeMedia(ctx, call, lteLegs(a), lteLegs(b), sdp.Audio); err != nil {
			return fmt.Errorf("voice only: %w", err)
		}

		if err := call.Caller.AddVideo(ctx); err != nil {
			return fmt.Errorf("add video: %w", err)
		}

		a2, err := awaitBearers(ctx, ac, imsVoiceQCI, imsVideoQCI)
		if err != nil {
			return fmt.Errorf("caller after adding video: %w", err)
		}

		b2, err := awaitBearers(ctx, bc, imsVoiceQCI, imsVideoQCI)
		if err != nil {
			return fmt.Errorf("callee after adding video: %w", err)
		}

		if a2[imsVoiceQCI].erab != a[imsVoiceQCI].erab || b2[imsVoiceQCI].erab != b[imsVoiceQCI].erab {
			return fmt.Errorf("the voice bearers moved from E-RABs %d/%d to %d/%d, want them kept", a[imsVoiceQCI].erab, b[imsVoiceQCI].erab, a2[imsVoiceQCI].erab, b2[imsVoiceQCI].erab)
		}

		return exchangeMedia(ctx, call, lteLegs(a2), lteLegs(b2), imsVideoMediaKinds...)
	}

	if err := scenarios.RequireIMSVideoCall(ctx, caller.ep, callee.ep, imsVideoTransport, true, script); err != nil {
		return err
	}

	if _, err := awaitBearers(ctx, ac); err != nil {
		return fmt.Errorf("caller bearers after the call: %w", err)
	}

	if _, err := awaitBearers(ctx, bc); err != nil {
		return fmt.Errorf("callee bearers after the call: %w", err)
	}

	return nil
}

func runIMSVideoRefused5G(ctx context.Context, env scenarios.Env) error {
	r, err := startIMSCallRANs(env)
	if err != nil {
		return err
	}

	defer r.close()

	caller, callee, err := r.partiesOn5GS(env, imsVideoRefused5GCaller, imsVideoRefused5GCallee)
	if err != nil {
		return err
	}

	defer caller.close()
	defer callee.close()

	script := func(ctx context.Context, call scenarios.IMSCall) error {
		a, err := caller.nr.awaitFlows(ctx, imsVoice5QI)
		if err != nil {
			return fmt.Errorf("caller: %w", err)
		}

		b, err := callee.nr.awaitFlows(ctx, imsVoice5QI)
		if err != nil {
			return fmt.Errorf("callee: %w", err)
		}

		before := r.g.ModifyRequestCount(callee.nr.ranUEID)
		r.g.RefuseNextGBRQoSFlows(callee.nr.ranUEID, imsVideoRefusal)

		if err := call.Caller.AddVideo(ctx); err != nil {
			return fmt.Errorf("add video: %w", err)
		}

		if err := awaitModifyRequest(ctx, callee.nr, before); err != nil {
			return err
		}

		if err := call.Callee.BearerLost(ctx, sdp.Video); err != nil {
			return fmt.Errorf("drop the refused video (IR.94 §2.4.1): %w", err)
		}

		if _, err := caller.nr.awaitFlows(ctx, imsVoice5QI); err != nil {
			return fmt.Errorf("caller after the video was dropped: %w", err)
		}

		now, err := callee.nr.awaitFlows(ctx, imsVoice5QI)
		if err != nil {
			return fmt.Errorf("callee after the video was dropped: %w", err)
		}

		if err := requireSameFlows(b, now, imsVoice5QI); err != nil {
			return err
		}

		return exchangeMedia(ctx, call, caller.nr.legs(a), callee.nr.legs(b), sdp.Audio)
	}

	if err := scenarios.RequireIMSVideoCall(ctx, caller.ep, callee.ep, imsVideoTransport, false, script); err != nil {
		return err
	}

	return awaitQoSFlowsReleased(ctx, caller.nr, callee.nr)
}

func runIMSVideoRefused4G(ctx context.Context, env scenarios.Env) error {
	r, err := startIMSCallRANs(env)
	if err != nil {
		return err
	}

	defer r.close()

	caller, callee, err := r.partiesOn4G(env, imsVideoRefused4GCaller, imsVideoRefused4GCallee)
	if err != nil {
		return err
	}

	defer caller.close()
	defer callee.close()

	ac := caller.lte.runBearers(bearerStep{accept: 1}, bearerStep{accept: 1}, bearerStep{release: []uint8{imsVideoQCI}}, bearerStep{release: []uint8{imsVoiceQCI}})
	bc := callee.lte.runBearers(bearerStep{accept: 1}, bearerStep{reject: imsVideoQCI}, bearerStep{release: []uint8{imsVideoQCI}}, bearerStep{release: []uint8{imsVoiceQCI}})

	script := func(ctx context.Context, call scenarios.IMSCall) error {
		a, err := awaitBearers(ctx, ac, imsVoiceQCI)
		if err != nil {
			return fmt.Errorf("caller: %w", err)
		}

		b, err := awaitBearers(ctx, bc, imsVoiceQCI)
		if err != nil {
			return fmt.Errorf("callee: %w", err)
		}

		if err := call.Caller.AddVideo(ctx); err != nil {
			return fmt.Errorf("add video: %w", err)
		}

		if _, err := awaitBearers(ctx, ac, imsVoiceQCI, imsVideoQCI); err != nil {
			return fmt.Errorf("caller video bearer: %w", err)
		}

		if _, err := awaitBearers(ctx, bc, imsVoiceQCI, imsVideoQCI); err != nil {
			return fmt.Errorf("callee refusing its video bearer: %w", err)
		}

		if _, err := awaitBearers(ctx, bc, imsVoiceQCI); err != nil {
			return fmt.Errorf("the refused video E-RAB not released at the eNB (TS 23.401 §5.4.1): %w", err)
		}

		if err := call.Callee.BearerLost(ctx, sdp.Video); err != nil {
			return fmt.Errorf("drop the refused video (IR.94 §2.4.1): %w", err)
		}

		if _, err := awaitBearers(ctx, ac, imsVoiceQCI); err != nil {
			return fmt.Errorf("caller after the video was dropped: %w", err)
		}

		return exchangeMedia(ctx, call, lteLegs(a), lteLegs(b), sdp.Audio)
	}

	if err := scenarios.RequireIMSVideoCall(ctx, caller.ep, callee.ep, imsVideoTransport, false, script); err != nil {
		return err
	}

	if _, err := awaitBearers(ctx, ac); err != nil {
		return fmt.Errorf("caller bearers after the call: %w", err)
	}

	if _, err := awaitBearers(ctx, bc); err != nil {
		return fmt.Errorf("callee bearers after the call: %w", err)
	}

	return nil
}

func runIMSVideoLost5G(ctx context.Context, env scenarios.Env) error {
	r, err := startIMSCallRANs(env)
	if err != nil {
		return err
	}

	defer r.close()

	caller, callee, err := r.partiesOn5GS(env, imsVideoLost5GCaller, imsVideoLost5GCallee)
	if err != nil {
		return err
	}

	defer caller.close()
	defer callee.close()

	script := func(ctx context.Context, call scenarios.IMSCall) error {
		a, err := caller.nr.awaitFlows(ctx, imsVoice5QI, imsVideo5QI)
		if err != nil {
			return fmt.Errorf("caller: %w", err)
		}

		b, err := callee.nr.awaitFlows(ctx, imsVoice5QI, imsVideo5QI)
		if err != nil {
			return fmt.Errorf("callee: %w", err)
		}

		if err := holdVideoFlows(ctx, caller.nr, callee.nr); err != nil {
			return err
		}

		n := callee.nr
		if err := n.g.SendQoSFlowsReleased(n.g.GetAMFUENGAPID(n.ranUEID), n.ranUEID, int64(imsPDUSessionID), []uint8{b[imsVideo5QI]}); err != nil {
			return fmt.Errorf("release the video flow at the gNB: %w", err)
		}

		if err := call.Callee.BearerLost(ctx, sdp.Video); err != nil {
			return fmt.Errorf("drop the lost video (IR.94 §2.4.1): %w", err)
		}

		now, err := caller.nr.awaitFlows(ctx, imsVoice5QI)
		if err != nil {
			return fmt.Errorf("caller after the video was lost: %w", err)
		}

		if err := requireSameFlows(a, now, imsVoice5QI); err != nil {
			return err
		}

		return exchangeMedia(ctx, call, caller.nr.legs(a), callee.nr.legs(b), sdp.Audio)
	}

	if err := scenarios.RequireIMSVideoCall(ctx, caller.ep, callee.ep, imsVideoTransport, true, script); err != nil {
		return err
	}

	return awaitQoSFlowsReleased(ctx, caller.nr, callee.nr)
}

func runIMSVideoLost4G(ctx context.Context, env scenarios.Env) error {
	r, err := startIMSCallRANs(env)
	if err != nil {
		return err
	}

	defer r.close()

	caller, callee, err := r.partiesOn4G(env, imsVideoLost4GCaller, imsVideoLost4GCallee)
	if err != nil {
		return err
	}

	defer caller.close()
	defer callee.close()

	ac := caller.lte.runBearers(bearerStep{accept: 2}, bearerStep{release: []uint8{imsVideoQCI}}, bearerStep{release: []uint8{imsVoiceQCI}})
	bc := callee.lte.runBearers(bearerStep{accept: 2}, bearerStep{release: []uint8{imsVoiceQCI}})

	script := func(ctx context.Context, call scenarios.IMSCall) error {
		a, err := awaitBearers(ctx, ac, imsVoiceQCI, imsVideoQCI)
		if err != nil {
			return fmt.Errorf("caller: %w", err)
		}

		b, err := awaitBearers(ctx, bc, imsVoiceQCI, imsVideoQCI)
		if err != nil {
			return fmt.Errorf("callee: %w", err)
		}

		if err := holdVideoBearers(ctx, ac, bc); err != nil {
			return err
		}

		l := callee.lte
		if err := l.e.SendERABReleaseIndication(l.mmeUEID, l.enbUEID, b[imsVideoQCI].erab, imsVideoFlowReleaseCause); err != nil {
			return fmt.Errorf("release the video bearer at the eNB: %w", err)
		}

		if err := call.Callee.BearerLost(ctx, sdp.Video); err != nil {
			return fmt.Errorf("drop the lost video (IR.94 §2.4.1): %w", err)
		}

		if _, err := awaitBearers(ctx, ac, imsVoiceQCI); err != nil {
			return fmt.Errorf("caller after the video was lost: %w", err)
		}

		delete(b, imsVideoQCI)

		return exchangeMedia(ctx, call, lteLegs(a), lteLegs(b), sdp.Audio)
	}

	if err := scenarios.RequireIMSVideoCall(ctx, caller.ep, callee.ep, imsVideoTransport, true, script); err != nil {
		return err
	}

	if _, err := awaitBearers(ctx, ac); err != nil {
		return fmt.Errorf("caller bearers after the call: %w", err)
	}

	if _, err := awaitBearers(ctx, bc); err != nil {
		return fmt.Errorf("callee bearers after the call: %w", err)
	}

	return nil
}

func runIMSVideoCallHandover(ctx context.Context, env scenarios.Env) error {
	r, err := startIMSCallRANs(env)
	if err != nil {
		return err
	}

	defer r.close()

	caller, callee, err := r.partiesOn5GS(env, imsVideoHandoverCaller, imsVideoHandoverCallee)
	if err != nil {
		return err
	}

	defer caller.close()
	defer callee.close()

	script := func(ctx context.Context, call scenarios.IMSCall) error {
		a, err := caller.nr.awaitFlows(ctx, imsVoice5QI, imsVideo5QI)
		if err != nil {
			return fmt.Errorf("caller: %w", err)
		}

		b, err := callee.nr.awaitFlows(ctx, imsVoice5QI, imsVideo5QI)
		if err != nil {
			return fmt.Errorf("callee: %w", err)
		}

		callerLegs := caller.nr.legs(a)

		if err := exchangeMedia(ctx, call, callerLegs, callee.nr.legs(b), imsVideoMediaKinds...); err != nil {
			return fmt.Errorf("on 5GS: %w", err)
		}

		if err := holdVideoFlows(ctx, caller.nr, callee.nr); err != nil {
			return err
		}

		onEPS, err := callee.handOverToEPSWith(r.e, epsHandoverOpts{carried: map[uint8]s1ap.ERABID{imsVoiceQCI: 0, imsVideoQCI: 0}})
		if err != nil {
			return fmt.Errorf("handover to EPS: %w", err)
		}

		held := make(map[uint8]lteBearer, len(onEPS.bearers))
		for qci, bearer := range onEPS.bearers {
			held[qci] = *bearer
		}

		if err := exchangeMedia(ctx, call, callerLegs, lteLegs(held), imsVideoMediaKinds...); err != nil {
			return fmt.Errorf("after the handover to EPS: %w", err)
		}

		back, err := callee.handOverTo5GSWith(r.g, imsMovedGNBRANUEIDs, fiveGSHandoverOpts{carried: map[int64]carriedFlow{
			imsVoice5QI: {ebi: held[imsVoiceQCI].erab, qfi: b[imsVoice5QI]},
			imsVideo5QI: {ebi: held[imsVideoQCI].erab, qfi: b[imsVideo5QI]},
		}})
		if err != nil {
			return fmt.Errorf("handover back to 5GS: %w", err)
		}

		calleeLegs := make(map[int64]mediaLeg, len(back.flows))
		for fiveQI, flow := range back.flows {
			calleeLegs[fiveQI] = *flow
		}

		if err := exchangeMedia(ctx, call, callerLegs, calleeLegs, imsVideoMediaKinds...); err != nil {
			return fmt.Errorf("back on 5GS: %w", err)
		}

		return nil
	}

	if err := scenarios.RequireIMSVideoCall(ctx, caller.ep, callee.ep, imsVideoTransport, true, script); err != nil {
		return err
	}

	return awaitQoSFlowsReleased(ctx, caller.nr, callee.nr)
}

func runIMSVideoCallEPSFallback(ctx context.Context, env scenarios.Env) error {
	r, err := startIMSCallRANs(env)
	if err != nil {
		return err
	}

	defer r.close()

	caller, callee, err := r.partiesOn5GS(env, imsVideoFallbackCaller, imsVideoFallbackCallee)
	if err != nil {
		return err
	}

	defer caller.close()
	defer callee.close()

	before := r.g.ModifyRequestCount(callee.nr.ranUEID)
	r.g.RefuseNextGBRQoSFlows(callee.nr.ranUEID, imsEPSFallbackCause)

	var bc <-chan bearerState

	script := func(ctx context.Context, call scenarios.IMSCall) error {
		a, err := caller.nr.awaitFlows(ctx, imsVoice5QI, imsVideo5QI)
		if err != nil {
			return fmt.Errorf("caller: %w", err)
		}

		if err := awaitFlowRefused(ctx, callee.nr, before); err != nil {
			return err
		}

		lte, err := fallBackByHandover(ctx, callee, r.e)
		if err != nil {
			return err
		}

		bc = lte.runBearers(bearerStep{accept: 2}, bearerStep{release: []uint8{imsVoiceQCI, imsVideoQCI}})

		b, err := awaitBearers(ctx, bc, imsVoiceQCI, imsVideoQCI)
		if err != nil {
			return fmt.Errorf("voice and video bearers after the EPS fallback (TS 23.502 §4.13.6.1 step 7): %w", err)
		}

		if err := holdBearers(ctx, bc); err != nil {
			return fmt.Errorf("callee: %w", err)
		}

		return exchangeMedia(ctx, call, caller.nr.legs(a), lteLegs(b), imsVideoMediaKinds...)
	}

	if err := scenarios.RequireIMSVideoCall(ctx, caller.ep, callee.ep, imsVideoTransport, true, script); err != nil {
		return err
	}

	if _, err := awaitBearers(ctx, bc); err != nil {
		return fmt.Errorf("callee bearers after the call: %w", err)
	}

	return awaitQoSFlowsReleased(ctx, caller.nr)
}

const imsVideoSettle = 2 * time.Second

func (a *nrAttachment) holdFlows(ctx context.Context, fiveQIs ...int64) error {
	deadline := time.Now().Add(imsVideoSettle)

	for time.Now().Before(deadline) {
		if _, err := a.awaitFlowsNow(fiveQIs...); err != nil {
			return fmt.Errorf("the QoS flows changed while the call was unchanged: %w", err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(imsPacketInterval):
		}
	}

	return nil
}

func (a *nrAttachment) awaitFlowsNow(fiveQIs ...int64) (map[int64]uint8, error) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	return a.awaitFlows(ctx, fiveQIs...)
}

func holdBearers(ctx context.Context, ch <-chan bearerState) error {
	select {
	case s := <-ch:
		return fmt.Errorf("the bearers changed to QCIs %v while the call was unchanged (err %v)", slices.Sorted(maps.Keys(s.held)), s.err)
	case <-time.After(imsVideoSettle):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func holdVideoFlows(ctx context.Context, caller, callee *nrAttachment) error {
	if err := caller.holdFlows(ctx, imsVoice5QI, imsVideo5QI); err != nil {
		return fmt.Errorf("caller: %w", err)
	}

	if _, err := callee.awaitFlowsNow(imsVoice5QI, imsVideo5QI); err != nil {
		return fmt.Errorf("callee: the QoS flows changed while the call was unchanged: %w", err)
	}

	return nil
}

func holdVideoBearers(ctx context.Context, caller, callee <-chan bearerState) error {
	if err := holdBearers(ctx, caller); err != nil {
		return fmt.Errorf("caller: %w", err)
	}

	select {
	case s := <-callee:
		return fmt.Errorf("callee: the bearers changed to QCIs %v while the call was unchanged (err %v)", slices.Sorted(maps.Keys(s.held)), s.err)
	default:
		return nil
	}
}
