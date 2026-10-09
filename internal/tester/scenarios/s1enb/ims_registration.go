// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"github.com/ellanetworks/ims/sip"
	"github.com/ellanetworks/ims/sip/sdp"
	"github.com/spf13/pflag"
)

const (
	imsTunIface   = "s1imstun"
	imsRouteTable = 100
)

var (
	imsRegistrationSubscriber = scenarios.IMSSubscriber("001017400000001", "+15557400001")
	imsNotEntitledSubscriber  = scenarios.NonIMSSubscriber("001017400000002", "+15557400002")
	imsCaller                 = scenarios.IMSSubscriber("001017400000003", "+15557400003")
	imsCallee                 = scenarios.IMSSubscriber("001017400000004", "+15557400004")
	imsRejectingCaller        = scenarios.IMSSubscriber("001017400000005", "+15557400005")
	imsRejectedCallee         = scenarios.IMSSubscriber("001017400000006", "+15557400006")
	imsSharingCaller          = scenarios.IMSSubscriber("001017400000007", "+15557400007")
	imsSharingCallee          = scenarios.IMSSubscriber("001017400000008", "+15557400008")
	imsReleasedCaller         = scenarios.IMSSubscriber("001017400000009", "+15557400009")
	imsReleasedCallee         = scenarios.IMSSubscriber("001017400000010", "+15557400010")
	imsRadioLostCaller        = scenarios.IMSSubscriber("001017400000011", "+15557400011")
	imsRadioLostCallee        = scenarios.IMSSubscriber("001017400000012", "+15557400012")
	imsHandoverCaller         = scenarios.IMSSubscriber("001017400000013", "+15557400013")
	imsHandoverCallee         = scenarios.IMSSubscriber("001017400000014", "+15557400014")
	imsModRejectCaller        = scenarios.IMSSubscriber("001017400000015", "+15557400015")
	imsModRejectCallee        = scenarios.IMSSubscriber("001017400000016", "+15557400016")
	imsHoldCaller             = scenarios.IMSSubscriber("001017400000017", "+15557400017")
	imsHoldCallee             = scenarios.IMSSubscriber("001017400000018", "+15557400018")
)

const (
	voiceBearerTimeout  = 30 * time.Second
	voiceMediaTimeout   = 5 * time.Second
	voicePacketInterval = 20 * time.Millisecond
	gateSettleInterval  = 300 * time.Millisecond
	gatedProbes         = 3
)

var voicePayload = []byte{0x80, 0x76, 0, 1, 0, 0, 0, 0, 0, 0, 0, 1}

func init() {
	registerIMS("ims/4g_registration", imsRegistrationSubscriber, runIMSRegistration)
	registerIMS("ims/4g_not_entitled", imsNotEntitledSubscriber, runIMSNotEntitled)
	registerIMS("ims/4g_call", imsCaller, runIMSCall, imsCallee)
	registerIMS("ims/4g_call_bearer_rejected", imsRejectingCaller, runIMSCallBearerRejected, imsRejectedCallee)
	registerIMS("ims/4g_two_calls", imsSharingCaller, runIMSTwoCalls, imsSharingCallee)
	registerIMS("ims/4g_two_calls_modification_rejected", imsModRejectCaller, runIMSModificationRejected, imsModRejectCallee)
	registerIMS("ims/4g_call_bearer_released", imsReleasedCaller, func(ctx context.Context, env scenarios.Env) error {
		return runIMSCallLoss(ctx, env, imsReleasedCaller, imsReleasedCallee, releaseBearerAtENB)
	}, imsReleasedCallee)
	registerIMS("ims/4g_call_s1_handover", imsHandoverCaller, runIMSCallS1Handover, imsHandoverCallee)
	registerIMS("ims/4g_call_hold", imsHoldCaller, runIMSCallHold, imsHoldCallee)
	registerIMS("ims/4g_call_radio_lost", imsRadioLostCaller, func(ctx context.Context, env scenarios.Env) error {
		return runIMSCallLoss(ctx, env, imsRadioLostCaller, imsRadioLostCallee, loseRadioConnection)
	}, imsRadioLostCallee)
}

func registerIMS(name string, sub scenarios.SubscriberSpec, run func(context.Context, scenarios.Env) error, more ...scenarios.SubscriberSpec) {
	scenarios.Register(scenarios.Scenario{
		Name:      name,
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return run(ctx, env)
		},
		Fixture: func(env scenarios.Env) scenarios.FixtureSpec {
			return scenarios.IMSFixture(env, append([]scenarios.SubscriberSpec{sub}, more...)...)
		},
	})
}

func runIMSRegistration(ctx context.Context, env scenarios.Env) error {
	ipv6 := !env.HasIPv4()

	e, res, local, err := attachIMSUE(env, imsRegistrationSubscriber, scenarios.IMSUEIPv4Pool, scenarios.IMSUEIPv6Pool)
	if err != nil {
		return err
	}

	defer closeIMSUE(e, res)

	if res.APN != scenarios.IMSDNN {
		return fmt.Errorf("attached to APN %q, want %q", res.APN, scenarios.IMSDNN)
	}

	if !res.IMSVoPS {
		return fmt.Errorf("the Attach Accept does not indicate IMS voice over PS sessions")
	}

	pcscf, err := scenarios.SelectPCSCF(res.PCSCF, ipv6)
	if err != nil {
		return err
	}

	for _, transport := range scenarios.IMSTransports {
		if err := scenarios.RegisterIMS(ctx, imsRegistrationSubscriber, pcscf, local, transport); err != nil {
			return err
		}
	}

	if err := scenarios.RegisterIMSOutOfSync(ctx, imsRegistrationSubscriber, pcscf, local, scenarios.IMSTransports[0]); err != nil {
		return err
	}

	return scenarios.RequireIMSTerminationOnDeletion(ctx, env, imsRegistrationSubscriber, pcscf, local, scenarios.IMSTransports[0])
}

func runIMSNotEntitled(ctx context.Context, env scenarios.Env) error {
	e, res, local, err := attachIMSUE(env, imsNotEntitledSubscriber, scenarios.DefaultUEIPv4Pool, scenarios.DefaultUEIPv6Pool)
	if err != nil {
		return err
	}

	defer closeIMSUE(e, res)

	if res.IMSVoPS {
		return fmt.Errorf("the Attach Accept indicates IMS voice over PS sessions to a subscriber without the ims data network")
	}

	pcscf := scenarios.IMSServerAddress(!env.HasIPv4())

	return scenarios.RequireIMSRegistrationForbidden(ctx, imsNotEntitledSubscriber, pcscf, local, scenarios.IMSTransports[0])
}

func attachIMSUE(env scenarios.Env, sub scenarios.SubscriberSpec, v4Pool, v6Pool string) (*s1enb.ENB, *s1enb.AttachResult, netip.Addr, error) {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return nil, nil, netip.Addr{}, err
	}

	e, err := startENBWithDatapath(env)
	if err != nil {
		return nil, nil, netip.Addr{}, fmt.Errorf("start S1 eNB: %w", err)
	}

	_, local, res, err := attachAndTunnelIMSUE(env, e, sub, k, opc, imsTunIface, v4Pool, v6Pool)
	if err != nil {
		_ = e.Close()
		return nil, nil, netip.Addr{}, err
	}

	return e, res, local, nil
}

func attachAndTunnelIMSUE(env scenarios.Env, e *s1enb.ENB, sub scenarios.SubscriberSpec, k, opc [16]byte, tunIface, v4Pool, v6Pool string) (*s1enb.UE, netip.Addr, *s1enb.AttachResult, error) {
	ipv6 := !env.HasIPv4()

	ue := e.NewUE(sub.IMSI, k, opc)
	ue.RequestPDNType(env.PDUSessionType())
	ue.RequestPCSCFAddresses()

	res, err := e.Attach(ue, attachTimeout)
	if err != nil {
		return nil, netip.Addr{}, nil, fmt.Errorf("attach: %w", err)
	}

	tun := &s1enb.TunnelOpts{
		UpfAddress:       res.UpfAddress,
		ULTEID:           res.ULTEID,
		DLTEID:           res.DLTEID,
		TunInterfaceName: tunIface,
		ExtraRoutes:      []string{scenarios.IMSRoute(scenarios.IMSServerAddress(ipv6))},
	}

	pool := v4Pool

	if ipv6 {
		tun.UEIPv6 = res.UEIPv6 + "/64"
		pool = v6Pool
	} else {
		tun.UEIPv4 = res.UEIPv4 + "/16"
	}

	if err := e.AddTunnel(tun); err != nil {
		return nil, netip.Addr{}, nil, fmt.Errorf("add GTP tunnel: %w", err)
	}

	if ipv6 {
		if err := s1enb.WaitForULAAddr(tunIface, pool, 5*time.Second); err != nil {
			return nil, netip.Addr{}, nil, fmt.Errorf("await SLAAC address: %w", err)
		}
	} else {
		awaitDownlinkReady()
	}

	local, err := scenarios.UEAddress(tunIface, netip.MustParsePrefix(pool))
	if err != nil {
		return nil, netip.Addr{}, nil, err
	}

	return ue, local, res, nil
}

type imsCallUE struct {
	ue  *s1enb.UE
	res *s1enb.AttachResult
}

func attachIMSCallUEs(env scenarios.Env, e *s1enb.ENB, subs ...scenarios.SubscriberSpec) ([]scenarios.IMSEndpoint, []imsCallUE, func(), error) {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return nil, nil, nil, err
	}

	var (
		endpoints []scenarios.IMSEndpoint
		ues       []imsCallUE
		cleanups  []func()
	)

	cleanup := func() {
		for _, c := range cleanups {
			c()
		}
	}

	for i, sub := range subs {
		tunIface := fmt.Sprintf("%s%d", imsTunIface, i)

		u, local, res, err := attachAndTunnelIMSUE(env, e, sub, k, opc, tunIface, scenarios.IMSUEIPv4Pool, scenarios.IMSUEIPv6Pool)
		if err != nil {
			cleanup()
			return nil, nil, nil, fmt.Errorf("UE %s: %w", sub.IMSI, err)
		}

		cleanups = append(cleanups, func() { e.CloseTunnel(res.DLTEID) })

		pcscf, err := scenarios.SelectPCSCF(res.PCSCF, !env.HasIPv4())
		if err != nil {
			cleanup()
			return nil, nil, nil, err
		}

		unroute, err := scenarios.RouteFromUE(tunIface, local, pcscf, imsRouteTable+i)
		if err != nil {
			cleanup()
			return nil, nil, nil, err
		}

		cleanups = append(cleanups, unroute)
		endpoints = append(endpoints, scenarios.IMSEndpoint{Subscriber: sub, PCSCF: pcscf, Local: local})
		ues = append(ues, imsCallUE{ue: u, res: res})
	}

	return endpoints, ues, cleanup, nil
}

type voiceBearers struct {
	active []chan *s1enb.DedicatedBearer
	wait   func() error
}

func expectVoiceBearers(e *s1enb.ENB, ues []imsCallUE) voiceBearers {
	return expectVoiceBearersReleasedBy(e, ues, e.AwaitDedicatedDeactivation)
}

func expectVoiceBearersReleasedBy(e *s1enb.ENB, ues []imsCallUE, release releaseAwaiter) voiceBearers {
	errs := make(chan error, len(ues))

	var vb voiceBearers

	for _, u := range ues {
		active := make(chan *s1enb.DedicatedBearer, 1)
		vb.active = append(vb.active, active)

		go func() { errs <- voiceBearerLifecycle(e, u, active, release) }()
	}

	vb.wait = func() error {
		var all error

		for range ues {
			all = errors.Join(all, <-errs)
		}

		return all
	}

	return vb
}

func voiceMedia(e *s1enb.ENB, ues []imsCallUE, vb voiceBearers) func(context.Context, scenarios.IMSMedia) error {
	return func(ctx context.Context, m scenarios.IMSMedia) error {
		bearers := make([]*s1enb.DedicatedBearer, len(vb.active))

		for i, active := range vb.active {
			select {
			case b, ok := <-active:
				if !ok {
					return fmt.Errorf("UE %s voice bearer was not set up", ues[i].ue.IMSI)
				}

				bearers[i] = b
			case <-ctx.Done():
				return fmt.Errorf("UE %s has no active voice bearer: %w", ues[i].ue.IMSI, ctx.Err())
			}
		}

		if err := sendVoice(ctx, e, ues[0], bearers[0], bearers[1], m.Caller, m.Callee); err != nil {
			return fmt.Errorf("caller to callee: %w", err)
		}

		if err := sendVoice(ctx, e, ues[1], bearers[1], bearers[0], m.Callee, m.Caller); err != nil {
			return fmt.Errorf("callee to caller: %w", err)
		}

		return nil
	}
}

func sendVoice(ctx context.Context, e *s1enb.ENB, from imsCallUE, ul, dl *s1enb.DedicatedBearer, src, dst sdp.Endpoint) error {
	return sendVoiceBetween(ctx, e, e, from, ul, dl, src, dst)
}

func sendVoiceBetween(ctx context.Context, e, peer *s1enb.ENB, from imsCallUE, ul, dl *s1enb.DedicatedBearer, src, dst sdp.Endpoint) error {
	upf, err := netip.ParseAddr(from.res.UpfAddress)
	if err != nil {
		return fmt.Errorf("UE %s S1-U peer %q: %w", from.ue.IMSI, from.res.UpfAddress, err)
	}

	peer.WatchTEID(dl.DLTEID)
	before := peer.WatchedTEIDCount(dl.DLTEID)

	packet := scenarios.UDPPacket(netip.AddrPortFrom(src.Addr, src.Port), netip.AddrPortFrom(dst.Addr, dst.Port), voicePayload)

	ctx, cancel := context.WithTimeout(ctx, voiceMediaTimeout)
	defer cancel()

	tick := time.NewTicker(voicePacketInterval)
	defer tick.Stop()

	for {
		if err := e.SendGPDU(ul.ULTEID, upf, packet); err != nil {
			return err
		}

		select {
		case <-tick.C:
		case <-ctx.Done():
			return fmt.Errorf("UDP %s -> %s sent on voice bearer TEID %#x never reached the peer's voice bearer TEID %#x: %w", src, dst, ul.ULTEID, dl.DLTEID, ctx.Err())
		}

		if peer.WatchedTEIDCount(dl.DLTEID) > before {
			return nil
		}
	}
}

type releaseAwaiter func(ue *s1enb.UE, enbUEID int64, timeout time.Duration) (s1ap.ERABID, *eps.DeactivateEPSBearerContextRequest, error)

func voiceBearerLifecycle(e *s1enb.ENB, u imsCallUE, active chan<- *s1enb.DedicatedBearer, release releaseAwaiter) error {
	signalled := false

	defer func() {
		if !signalled {
			close(active)
		}
	}()

	b, err := e.AcceptDedicatedBearer(u.ue, u.res.ENBUES1APID, voiceBearerTimeout)
	if err != nil {
		return fmt.Errorf("UE %s voice bearer: %w", u.ue.IMSI, err)
	}

	rates, ok := b.EPSQoS.GBRBitRates()

	switch {
	case b.QoS.QCI != 1 || b.EPSQoS.QCI != 1:
		return fmt.Errorf("UE %s voice bearer at QCI %d/%d, want 1", u.ue.IMSI, b.QoS.QCI, b.EPSQoS.QCI)
	case b.LinkedEBI != uint8(u.res.ERABID):
		return fmt.Errorf("UE %s voice bearer linked to EBI %d, want the ims default bearer %d", u.ue.IMSI, b.LinkedEBI, u.res.ERABID)
	case b.QoS.GBR == nil || b.QoS.GBR.GuaranteedBitrateDL == 0 || b.QoS.GBR.GuaranteedBitrateUL == 0:
		return fmt.Errorf("UE %s voice E-RAB has no guaranteed bit rate: %+v", u.ue.IMSI, b.QoS.GBR)
	case !ok || rates.GuaranteedDownlinkKbps == 0 || rates.GuaranteedUplinkKbps == 0:
		return fmt.Errorf("UE %s voice EPS QoS has no guaranteed bit rate", u.ue.IMSI)
	case b.QoS.ARP.PriorityLevel != 2 || b.QoS.ARP.PreemptionCapability != s1ap.PreemptionMayTrigger || b.QoS.ARP.PreemptionVulnerability != s1ap.PreemptionNotPreemptable:
		return fmt.Errorf("UE %s voice bearer ARP %+v, want priority 2, may pre-empt, not pre-emptable", u.ue.IMSI, b.QoS.ARP)
	case b.TFT.Operation != eps.TFTCreate || len(b.TFT.Filters) == 0:
		return fmt.Errorf("UE %s voice bearer TFT %+v, want new packet filters", u.ue.IMSI, b.TFT)
	}

	active <- b

	signalled = true

	erab, deact, err := release(u.ue, u.res.ENBUES1APID, voiceBearerTimeout)

	switch {
	case err != nil:
		return fmt.Errorf("UE %s voice bearer release: %w", u.ue.IMSI, err)
	case erab != b.ERABID:
		return fmt.Errorf("UE %s released E-RAB %d, want the voice bearer %d", u.ue.IMSI, erab, b.ERABID)
	case deact == nil || deact.Cause != eps.ESMCauseRegularDeactivation:
		return fmt.Errorf("UE %s voice bearer deactivated with %+v, want cause #36", u.ue.IMSI, deact)
	}

	return nil
}

func runIMSCall(ctx context.Context, env scenarios.Env) error {
	e, err := startENBWithDatapath(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	endpoints, ues, cleanup, err := attachIMSCallUEs(env, e, imsCaller, imsCallee)
	if err != nil {
		return err
	}

	defer cleanup()

	for _, transport := range scenarios.IMSTransports {
		bearers := expectVoiceBearers(e, ues)

		if err := scenarios.RequireIMSCallWithMedia(ctx, endpoints[0], endpoints[1], transport, voiceMedia(e, ues, bearers)); err != nil {
			return errors.Join(err, bearers.wait())
		}

		if err := bearers.wait(); err != nil {
			return fmt.Errorf("call over %s: %w", transport, err)
		}
	}

	callee := ues[1]

	release := func() error {
		return e.Detach(callee.ue, callee.res.MMEUES1APID, callee.res.ENBUES1APID, releaseTimeout)
	}

	return scenarios.RequireIMSSignallingLoss(ctx, endpoints[0], endpoints[1], scenarios.IMSTransports[0], release)
}

func runIMSCallHold(ctx context.Context, env scenarios.Env) error {
	e, err := startENBWithDatapath(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	endpoints, ues, cleanup, err := attachIMSCallUEs(env, e, imsHoldCaller, imsHoldCallee)
	if err != nil {
		return err
	}

	defer cleanup()

	vb := expectVoiceBearersReleasedBy(e, ues, e.AwaitUnmodifiedDedicatedDeactivation)

	var bearers []*s1enb.DedicatedBearer

	steps := scenarios.IMSHoldSteps{
		Up: func(ctx context.Context, m scenarios.IMSMedia) error {
			for i, active := range vb.active {
				select {
				case b, ok := <-active:
					if !ok {
						return fmt.Errorf("UE %s voice bearer was not set up", ues[i].ue.IMSI)
					}

					bearers = append(bearers, b)
				case <-ctx.Done():
					return fmt.Errorf("UE %s has no active voice bearer: %w", ues[i].ue.IMSI, ctx.Err())
				}
			}

			return sendVoiceBothWays(ctx, e, ues, bearers, m)
		},
		Held: func(ctx context.Context, m scenarios.IMSMedia) error {
			if err := requireVoiceGated(ctx, e, ues[1], bearers[1], bearers[0], m.Callee, m.Caller); err != nil {
				return fmt.Errorf("callee to held caller (TS 29.214 §5.3.11): %w", err)
			}

			if err := sendVoice(ctx, e, ues[0], bearers[0], bearers[1], m.Caller, m.Callee); err != nil {
				return fmt.Errorf("held caller to callee: %w", err)
			}

			rtcp := scenarios.IMSMedia{Caller: scenarios.RTCPEndpoint(m.Caller), Callee: scenarios.RTCPEndpoint(m.Callee)}

			if err := sendVoiceBothWays(ctx, e, ues, bearers, rtcp); err != nil {
				return fmt.Errorf("RTCP on hold (TS 29.214 §4.4.3): %w", err)
			}

			return nil
		},
		Resumed: func(ctx context.Context, m scenarios.IMSMedia) error {
			return sendVoiceBothWays(ctx, e, ues, bearers, m)
		},
	}

	if err := scenarios.RequireIMSCallHeld(ctx, endpoints[0], endpoints[1], scenarios.IMSTransports[0], steps); err != nil {
		return errors.Join(err, vb.wait())
	}

	return vb.wait()
}

func requireVoiceGated(ctx context.Context, e *s1enb.ENB, from imsCallUE, ul, dl *s1enb.DedicatedBearer, src, dst sdp.Endpoint) error {
	upf, err := netip.ParseAddr(from.res.UpfAddress)
	if err != nil {
		return fmt.Errorf("UE %s S1-U peer %q: %w", from.ue.IMSI, from.res.UpfAddress, err)
	}

	e.WatchTEID(dl.DLTEID)

	packet := scenarios.UDPPacket(netip.AddrPortFrom(src.Addr, src.Port), netip.AddrPortFrom(dst.Addr, dst.Port), voicePayload)

	ctx, cancel := context.WithTimeout(ctx, voiceMediaTimeout)
	defer cancel()

	dropped := 0

	for dropped < gatedProbes {
		before := e.WatchedTEIDCount(dl.DLTEID)

		if err := e.SendGPDU(ul.ULTEID, upf, packet); err != nil {
			return err
		}

		select {
		case <-time.After(gateSettleInterval):
		case <-ctx.Done():
			return fmt.Errorf("UDP %s -> %s still reaches voice bearer TEID %#x: %w", src, dst, dl.DLTEID, ctx.Err())
		}

		dropped++
		if e.WatchedTEIDCount(dl.DLTEID) != before {
			dropped = 0
		}
	}

	return nil
}

func runIMSCallBearerRejected(ctx context.Context, env scenarios.Env) error {
	e, err := startENBWithDatapath(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	endpoints, ues, cleanup, err := attachIMSCallUEs(env, e, imsRejectingCaller, imsRejectedCallee)
	if err != nil {
		return err
	}

	defer cleanup()

	caller := ues[0]
	rejected := make(chan error, 1)

	go func() {
		_, err := e.RejectDedicatedBearer(caller.ue, caller.res.ENBUES1APID, voiceBearerTimeout)
		rejected <- err
	}()

	if err := scenarios.RequireIMSCallEndedByNetwork(ctx, endpoints[0], endpoints[1], scenarios.IMSTransports[0]); err != nil {
		return err
	}

	if err := <-rejected; err != nil {
		return fmt.Errorf("reject the voice bearer: %w", err)
	}

	return nil
}

func closeIMSUE(e *s1enb.ENB, res *s1enb.AttachResult) {
	e.CloseTunnel(res.DLTEID)
	_ = e.Close()
}

type sharedBearer struct {
	active  chan *s1enb.DedicatedBearer
	added   chan *s1enb.DedicatedBearer
	removed chan *s1enb.DedicatedBearer
}

func newSharedBearer() sharedBearer {
	return sharedBearer{
		active:  make(chan *s1enb.DedicatedBearer, 1),
		added:   make(chan *s1enb.DedicatedBearer, 1),
		removed: make(chan *s1enb.DedicatedBearer, 1),
	}
}

func (sb sharedBearer) closeAll() {
	close(sb.active)
	close(sb.added)
	close(sb.removed)
}

func sharedBearerLifecycle(e *s1enb.ENB, u imsCallUE, sb sharedBearer) error {
	defer sb.closeAll()

	b, err := e.AcceptDedicatedBearer(u.ue, u.res.ENBUES1APID, voiceBearerTimeout)
	if err != nil {
		return fmt.Errorf("UE %s voice bearer: %w", u.ue.IMSI, err)
	}

	if b.QoS.QCI != 1 || b.QoS.GBR == nil {
		return fmt.Errorf("UE %s voice bearer at QCI %d with GBR %+v, want a QCI 1 GBR bearer", u.ue.IMSI, b.QoS.QCI, b.QoS.GBR)
	}

	firstGBR, firstFilters := b.QoS.GBR.GuaranteedBitrateDL, len(b.TFT.Filters)
	sb.active <- b

	add, err := e.AcceptDedicatedModification(u.ue, u.res.ENBUES1APID, b, voiceBearerTimeout)

	switch {
	case err != nil:
		return fmt.Errorf("UE %s second call on the voice bearer: %w", u.ue.IMSI, err)
	case add.TFT == nil || add.TFT.Operation != eps.TFTAddFilters || len(b.TFT.Filters) != 2*firstFilters:
		return fmt.Errorf("UE %s second call modification %+v leaves %d filters, want %d added to the shared bearer (IR.92 §4.4)", u.ue.IMSI, add.TFT, len(b.TFT.Filters), firstFilters)
	case b.QoS.GBR == nil || b.QoS.GBR.GuaranteedBitrateDL <= firstGBR:
		return fmt.Errorf("UE %s voice E-RAB GBR %+v after the second call, want more than %d", u.ue.IMSI, b.QoS.GBR, firstGBR)
	}

	sb.added <- b

	del, err := e.AcceptDedicatedModification(u.ue, u.res.ENBUES1APID, b, voiceBearerTimeout)

	switch {
	case err != nil:
		return fmt.Errorf("UE %s first call ending: %w", u.ue.IMSI, err)
	case del.TFT == nil || del.TFT.Operation != eps.TFTDeleteFilters || len(b.TFT.Filters) != firstFilters:
		return fmt.Errorf("UE %s first call ending: modification %+v leaves %d filters, want %d", u.ue.IMSI, del.TFT, len(b.TFT.Filters), firstFilters)
	case b.QoS.GBR == nil || b.QoS.GBR.GuaranteedBitrateDL != firstGBR:
		return fmt.Errorf("UE %s voice E-RAB GBR %+v after the first call ended, want %d", u.ue.IMSI, b.QoS.GBR, firstGBR)
	}

	sb.removed <- b

	erab, _, err := e.AwaitDedicatedDeactivation(u.ue, u.res.ENBUES1APID, voiceBearerTimeout)

	switch {
	case err != nil:
		return fmt.Errorf("UE %s voice bearer release: %w", u.ue.IMSI, err)
	case erab != b.ERABID:
		return fmt.Errorf("UE %s released E-RAB %d, want the voice bearer %d", u.ue.IMSI, erab, b.ERABID)
	}

	return nil
}

func awaitShared(ctx context.Context, ues []imsCallUE, chans []chan *s1enb.DedicatedBearer, what string) ([]*s1enb.DedicatedBearer, error) {
	out := make([]*s1enb.DedicatedBearer, len(chans))

	for i, ch := range chans {
		select {
		case b, ok := <-ch:
			if !ok {
				return nil, fmt.Errorf("UE %s voice bearer was not %s", ues[i].ue.IMSI, what)
			}

			out[i] = b
		case <-ctx.Done():
			return nil, fmt.Errorf("UE %s voice bearer not %s: %w", ues[i].ue.IMSI, what, ctx.Err())
		}
	}

	return out, nil
}

func sendVoiceBothWays(ctx context.Context, e *s1enb.ENB, ues []imsCallUE, bearers []*s1enb.DedicatedBearer, m scenarios.IMSMedia) error {
	if err := sendVoice(ctx, e, ues[0], bearers[0], bearers[1], m.Caller, m.Callee); err != nil {
		return fmt.Errorf("caller to callee: %w", err)
	}

	if err := sendVoice(ctx, e, ues[1], bearers[1], bearers[0], m.Callee, m.Caller); err != nil {
		return fmt.Errorf("callee to caller: %w", err)
	}

	return nil
}

func runIMSTwoCalls(ctx context.Context, env scenarios.Env) error {
	e, err := startENBWithDatapath(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	endpoints, ues, cleanup, err := attachIMSCallUEs(env, e, imsSharingCaller, imsSharingCallee)
	if err != nil {
		return err
	}

	defer cleanup()

	shared := []sharedBearer{newSharedBearer(), newSharedBearer()}
	errs := make(chan error, len(ues))

	for i, u := range ues {
		go func() { errs <- sharedBearerLifecycle(e, u, shared[i]) }()
	}

	wait := func() error {
		var all error

		for range ues {
			all = errors.Join(all, <-errs)
		}

		return all
	}

	steps := scenarios.IMSTwoCallSteps{
		BothUp: func(ctx context.Context, first, second scenarios.IMSMedia) error {
			bearers, err := awaitShared(ctx, ues, []chan *s1enb.DedicatedBearer{shared[0].added, shared[1].added}, "extended for the second call")
			if err != nil {
				return err
			}

			if err := sendVoiceBothWays(ctx, e, ues, bearers, first); err != nil {
				return fmt.Errorf("first call: %w", err)
			}

			return sendVoiceBothWays(ctx, e, ues, bearers, second)
		},
		FirstEnded: func(ctx context.Context, second scenarios.IMSMedia) error {
			bearers, err := awaitShared(ctx, ues, []chan *s1enb.DedicatedBearer{shared[0].removed, shared[1].removed}, "reduced after the first call")
			if err != nil {
				return err
			}

			return sendVoiceBothWays(ctx, e, ues, bearers, second)
		},
	}

	if err := scenarios.RequireIMSTwoCalls(ctx, endpoints[0], endpoints[1], scenarios.IMSTransports[0], steps); err != nil {
		return errors.Join(err, wait())
	}

	return wait()
}

func runIMSModificationRejected(ctx context.Context, env scenarios.Env) error {
	e, err := startENBWithDatapath(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	endpoints, ues, cleanup, err := attachIMSCallUEs(env, e, imsModRejectCaller, imsModRejectCallee)
	if err != nil {
		return err
	}

	defer cleanup()

	caller, callee := ues[0], ues[1]
	active := []chan *s1enb.DedicatedBearer{make(chan *s1enb.DedicatedBearer, 1), make(chan *s1enb.DedicatedBearer, 1)}
	realigned := make(chan *s1enb.DedicatedBearer, 1)
	results := make(chan error, 2)

	go func() {
		b, err := e.AcceptDedicatedBearer(caller.ue, caller.res.ENBUES1APID, voiceBearerTimeout)
		if err != nil {
			close(active[0])

			results <- fmt.Errorf("UE %s voice bearer: %w", caller.ue.IMSI, err)

			return
		}

		active[0] <- b

		if _, err := e.AcceptDedicatedChangesUntilRelease(caller.ue, caller.res.ENBUES1APID, b, voiceBearerTimeout); err != nil {
			results <- fmt.Errorf("UE %s voice bearer: %w", caller.ue.IMSI, err)
			return
		}

		results <- nil
	}()

	go func() {
		results <- rejectSecondCall(e, callee, active[1], realigned)
	}()

	steps := scenarios.IMSRejectedCallSteps{
		FirstUp: func(ctx context.Context, first scenarios.IMSMedia) error {
			bearers, err := awaitShared(ctx, ues, active, "set up")
			if err != nil {
				return err
			}

			active[0] <- bearers[0]

			return sendVoiceBothWays(ctx, e, ues, bearers, first)
		},
		AfterRejection: func(ctx context.Context, first scenarios.IMSMedia) error {
			bearers, err := awaitShared(ctx, ues, []chan *s1enb.DedicatedBearer{active[0], realigned}, "realigned after the rejection")
			if err != nil {
				return err
			}

			return sendVoiceBothWays(ctx, e, ues, bearers, first)
		},
	}

	callErr := scenarios.RequireIMSSecondCallRejected(ctx, endpoints[0], endpoints[1], scenarios.IMSTransports[0], steps)

	return errors.Join(callErr, <-results, <-results)
}

func rejectSecondCall(e *s1enb.ENB, u imsCallUE, active, realigned chan<- *s1enb.DedicatedBearer) error {
	b, err := e.AcceptDedicatedBearer(u.ue, u.res.ENBUES1APID, voiceBearerTimeout)
	if err != nil {
		close(active)
		close(realigned)

		return fmt.Errorf("UE %s voice bearer: %w", u.ue.IMSI, err)
	}

	firstGBR, firstFilters := b.QoS.GBR.GuaranteedBitrateDL, len(b.TFT.Filters)
	active <- b

	if _, err := e.RejectDedicatedModification(u.ue, u.res.ENBUES1APID, b, eps.ESMCauseInsufficientResources, voiceBearerTimeout); err != nil {
		close(realigned)
		return fmt.Errorf("UE %s second call on the voice bearer: %w", u.ue.IMSI, err)
	}

	realign, err := e.AcceptDedicatedModification(u.ue, u.res.ENBUES1APID, b, voiceBearerTimeout)

	switch {
	case err != nil:
		close(realigned)
		return fmt.Errorf("UE %s realignment after the rejection: %w", u.ue.IMSI, err)
	case realign.TFT != nil || len(b.TFT.Filters) != firstFilters:
		close(realigned)
		return fmt.Errorf("UE %s realignment %+v leaves %d filters, want the first call's %d unchanged", u.ue.IMSI, realign.TFT, len(b.TFT.Filters), firstFilters)
	case b.QoS.GBR == nil || b.QoS.GBR.GuaranteedBitrateDL != firstGBR:
		close(realigned)
		return fmt.Errorf("UE %s voice E-RAB GBR %+v after the realignment, want the first call's %d (TS 24.301 §6.4.3.4)", u.ue.IMSI, b.QoS.GBR, firstGBR)
	}

	realigned <- b

	erab, _, err := e.AwaitDedicatedDeactivation(u.ue, u.res.ENBUES1APID, voiceBearerTimeout)

	switch {
	case err != nil:
		return fmt.Errorf("UE %s voice bearer release: %w", u.ue.IMSI, err)
	case erab != b.ERABID:
		return fmt.Errorf("UE %s released E-RAB %d, want the voice bearer %d", u.ue.IMSI, erab, b.ERABID)
	}

	return nil
}

type bearerLoss func(e *s1enb.ENB, u imsCallUE, b *s1enb.DedicatedBearer) error

func releaseBearerAtENB(e *s1enb.ENB, u imsCallUE, b *s1enb.DedicatedBearer) error {
	return e.SendERABReleaseIndication(u.res.MMEUES1APID, u.res.ENBUES1APID, b.ERABID, s1enb.CauseRadioConnectionWithUELost)
}

func loseRadioConnection(e *s1enb.ENB, u imsCallUE, _ *s1enb.DedicatedBearer) error {
	if err := e.SendUEContextReleaseRequest(u.res.MMEUES1APID, u.res.ENBUES1APID, s1enb.CauseRadioConnectionWithUELost); err != nil {
		return err
	}

	cmd, err := e.WaitForUEContextReleaseCommand(u.res.ENBUES1APID, releaseTimeout)
	if err != nil {
		return fmt.Errorf("await UE Context Release Command: %w", err)
	}

	return e.SendUEContextReleaseComplete(int64(cmd.UES1APIDs.MMEUES1APID), u.res.ENBUES1APID)
}

func runIMSCallLoss(ctx context.Context, env scenarios.Env, caller, callee scenarios.SubscriberSpec, lose bearerLoss) error {
	e, err := startENBWithDatapath(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	endpoints, ues, cleanup, err := attachIMSCallUEs(env, e, caller, callee)
	if err != nil {
		return err
	}

	defer cleanup()

	lost, survivor := ues[1], ues[0]
	lostBearer := make(chan *s1enb.DedicatedBearer, 1)
	survived := make(chan error, 1)

	go func() {
		b, err := e.AcceptDedicatedBearer(lost.ue, lost.res.ENBUES1APID, voiceBearerTimeout)
		if err != nil {
			close(lostBearer)
			return
		}

		lostBearer <- b
	}()

	go func() {
		b, err := e.AcceptDedicatedBearer(survivor.ue, survivor.res.ENBUES1APID, voiceBearerTimeout)
		if err != nil {
			survived <- fmt.Errorf("UE %s voice bearer: %w", survivor.ue.IMSI, err)
			return
		}

		erab, _, err := e.AwaitDedicatedDeactivation(survivor.ue, survivor.res.ENBUES1APID, voiceBearerTimeout)

		switch {
		case err != nil:
			survived <- fmt.Errorf("UE %s voice bearer release after the peer lost its bearer: %w", survivor.ue.IMSI, err)
		case erab != b.ERABID:
			survived <- fmt.Errorf("UE %s released E-RAB %d, want the voice bearer %d", survivor.ue.IMSI, erab, b.ERABID)
		default:
			survived <- nil
		}
	}()

	loseBearer := func(ctx context.Context) error {
		select {
		case b, ok := <-lostBearer:
			if !ok {
				return fmt.Errorf("UE %s voice bearer was not set up", lost.ue.IMSI)
			}

			return lose(e, lost, b)
		case <-ctx.Done():
			return fmt.Errorf("UE %s has no voice bearer: %w", lost.ue.IMSI, ctx.Err())
		}
	}

	if err := scenarios.RequireIMSCallLost(ctx, endpoints[0], endpoints[1], scenarios.IMSTransports[0], loseBearer); err != nil {
		return err
	}

	return <-survived
}

type handedOver struct {
	enbUEID     int64
	mmeUEID     int64
	defaultTEID uint32
	voiceTEID   uint32
}

func startTargetENB(env scenarios.Env) (*s1enb.ENB, error) {
	g := env.FirstGNB()
	if g.N3Secondary == "" {
		return nil, fmt.Errorf("an S1 handover needs a secondary N3 address for the target eNB")
	}

	s1mme, err := s1mmeAddress(env.FirstCore())
	if err != nil {
		return nil, err
	}

	enbID, err := strconv.ParseUint(scenarios.DefaultGNBID, 16, 32)
	if err != nil {
		return nil, fmt.Errorf("parse eNB ID %q: %w", scenarios.DefaultGNBID, err)
	}

	return s1enb.Start(&s1enb.StartOpts{
		ENBID: uint32(enbID) + 1, MCC: scenarios.DefaultMCC, MNC: scenarios.DefaultMNC, TAC: scenarios.DefaultTAC,
		Name: "Target-S1eNB", CoreS1MMEAddress: s1mme,
		ENBAddress: g.N2Address, ENBN3Address: g.N3Secondary, EnableDatapath: true,
	})
}

func handOverVoiceUE(source, target *s1enb.ENB, u imsCallUE, voice *s1enb.DedicatedBearer) (handedOver, error) {
	if err := source.SendHandoverRequired(u.res.ENBUES1APID, u.res.MMEUES1APID, target.GlobalENBID(), false); err != nil {
		return handedOver{}, fmt.Errorf("send Handover Required: %w", err)
	}

	req, err := target.WaitForHandoverRequest(10 * time.Second)
	if err != nil {
		return handedOver{}, fmt.Errorf("await Handover Request: %w", err)
	}

	i := slices.IndexFunc(req.ERABToBeSetup, func(it s1ap.ERABToBeSetupItemHOReq) bool { return it.ERABID == voice.ERABID })
	if i < 0 || len(req.ERABToBeSetup) != 2 {
		return handedOver{}, fmt.Errorf("the Handover Request E-RABs are %+v, want the default and the voice bearer (TS 23.401 §5.5.1.2.2)", req.ERABToBeSetup)
	}

	if qos := req.ERABToBeSetup[i].QoS; qos.QCI != 1 || qos.GBR == nil {
		return handedOver{}, fmt.Errorf("voice E-RAB QoS %+v in the Handover Request, want QCI 1 with GBR QoS Information (TS 36.413 §8.4.2.4)", qos)
	}

	out := handedOver{enbUEID: target.AllocateENBUEID(), mmeUEID: int64(req.MMEUES1APID)}

	teids, err := target.SendHandoverRequestAcknowledgePartial(out.enbUEID, out.mmeUEID, []s1ap.ERABID{u.res.ERABID, voice.ERABID}, nil, s1ap.Cause{})
	if err != nil {
		return handedOver{}, fmt.Errorf("send Handover Request Acknowledge: %w", err)
	}

	out.defaultTEID, out.voiceTEID = teids[u.res.ERABID], teids[voice.ERABID]

	if _, err := source.WaitForHandoverCommand(u.res.ENBUES1APID, 10*time.Second); err != nil {
		return handedOver{}, fmt.Errorf("await Handover Command: %w", err)
	}

	if err := target.SendHandoverNotify(out.enbUEID, out.mmeUEID); err != nil {
		return handedOver{}, fmt.Errorf("send Handover Notify: %w", err)
	}

	if _, err := source.WaitForUEContextReleaseCommand(u.res.ENBUES1APID, releaseTimeout); err != nil {
		return handedOver{}, fmt.Errorf("await the source UE Context Release Command: %w", err)
	}

	if err := source.SendUEContextReleaseComplete(u.res.MMEUES1APID, u.res.ENBUES1APID); err != nil {
		return handedOver{}, err
	}

	return out, nil
}

func rehomeIMSTunnel(env scenarios.Env, source, target *s1enb.ENB, u imsCallUE, ep scenarios.IMSEndpoint, iface string, table int, dlTEID uint32) error {
	ipv6 := !env.HasIPv4()

	source.CloseTunnel(u.res.DLTEID)

	tun := &s1enb.TunnelOpts{
		UpfAddress:       u.res.UpfAddress,
		ULTEID:           u.res.ULTEID,
		DLTEID:           dlTEID,
		TunInterfaceName: iface,
		ExtraRoutes:      []string{scenarios.IMSRoute(scenarios.IMSServerAddress(ipv6))},
	}

	pool := scenarios.IMSUEIPv4Pool

	if ipv6 {
		tun.UEIPv6 = u.res.UEIPv6 + "/64"
		pool = scenarios.IMSUEIPv6Pool
	} else {
		tun.UEIPv4 = u.res.UEIPv4 + "/16"
	}

	if err := target.AddTunnel(tun); err != nil {
		return fmt.Errorf("add the target GTP tunnel: %w", err)
	}

	if ipv6 {
		if err := s1enb.WaitForULAAddr(iface, pool, 5*time.Second); err != nil {
			return fmt.Errorf("await SLAAC address: %w", err)
		}
	} else {
		awaitDownlinkReady()
	}

	return scenarios.RerouteToIMS(iface, ep.Local, ep.PCSCF, table)
}

func runIMSCallS1Handover(ctx context.Context, env scenarios.Env) error {
	source, err := startENBWithDatapath(env)
	if err != nil {
		return fmt.Errorf("start the source eNB: %w", err)
	}

	defer func() { _ = source.Close() }()

	target, err := startTargetENB(env)
	if err != nil {
		return fmt.Errorf("start the target eNB: %w", err)
	}

	defer func() { _ = target.Close() }()

	endpoints, ues, cleanup, err := attachIMSCallUEs(env, source, imsHandoverCaller, imsHandoverCallee)
	if err != nil {
		return err
	}

	defer cleanup()

	caller, callee := ues[0], ues[1]
	active := []chan *s1enb.DedicatedBearer{make(chan *s1enb.DedicatedBearer, 1), make(chan *s1enb.DedicatedBearer, 1)}
	moved := make(chan handedOver, 1)
	results := make(chan error, 2)

	go func() {
		b, err := source.AcceptDedicatedBearer(caller.ue, caller.res.ENBUES1APID, voiceBearerTimeout)
		if err != nil {
			close(active[0])

			results <- fmt.Errorf("UE %s voice bearer: %w", caller.ue.IMSI, err)

			return
		}

		active[0] <- b

		_, _, err = source.AwaitDedicatedDeactivation(caller.ue, caller.res.ENBUES1APID, voiceBearerTimeout)
		results <- err
	}()

	go func() {
		b, err := source.AcceptDedicatedBearer(callee.ue, callee.res.ENBUES1APID, voiceBearerTimeout)
		if err != nil {
			close(active[1])

			results <- fmt.Errorf("UE %s voice bearer: %w", callee.ue.IMSI, err)

			return
		}

		active[1] <- b

		h, ok := <-moved
		if !ok {
			results <- nil
			return
		}

		_, _, err = target.AwaitDedicatedDeactivation(callee.ue, h.enbUEID, voiceBearerTimeout)
		if err != nil {
			err = fmt.Errorf("UE %s voice bearer release after the handover: %w", callee.ue.IMSI, err)
		}

		results <- err
	}()

	media := func(ctx context.Context, m scenarios.IMSMedia) error {
		bearers, err := awaitShared(ctx, ues, active, "set up")
		if err != nil {
			return err
		}

		if err := sendVoiceBothWays(ctx, source, ues, bearers, m); err != nil {
			return fmt.Errorf("before the handover: %w", err)
		}

		h, err := handOverVoiceUE(source, target, callee, bearers[1])
		if err != nil {
			close(moved)
			return fmt.Errorf("S1 handover of %s: %w", callee.ue.IMSI, err)
		}

		moved <- h

		if err := rehomeIMSTunnel(env, source, target, callee, endpoints[1], fmt.Sprintf("%s%d", imsTunIface, 1), imsRouteTable+1, h.defaultTEID); err != nil {
			return err
		}

		after := *bearers[1]
		after.DLTEID = h.voiceTEID

		if err := sendVoiceBetween(ctx, source, target, caller, bearers[0], &after, m.Caller, m.Callee); err != nil {
			return fmt.Errorf("after the handover, caller to callee: %w", err)
		}

		if err := sendVoiceBetween(ctx, target, source, callee, &after, bearers[0], m.Callee, m.Caller); err != nil {
			return fmt.Errorf("after the handover, callee to caller: %w", err)
		}

		return nil
	}

	callErr := scenarios.RequireIMSCallWithMedia(ctx, endpoints[0], endpoints[1], sip.UDP, media)

	return errors.Join(callErr, <-results, <-results)
}
