// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"net/netip"
	"slices"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/ims/sip/sdp"
	"github.com/spf13/pflag"
)

const (
	imsTunIface   = gtpInterfaceNamePrefix + "ims0"
	imsRouteTable = 100
)

var (
	imsRegistrationSubscriber = scenarios.IMSSubscriber("001018400000001", "+15558400001")
	imsNotEntitledSubscriber  = scenarios.NonIMSSubscriber("001018400000002", "+15558400002")
	imsCaller                 = scenarios.IMSSubscriber("001018400000003", "+15558400003")
	imsCallee                 = scenarios.IMSSubscriber("001018400000004", "+15558400004")
)

func init() {
	registerIMS("ims/5g_registration", imsRegistrationSubscriber, runIMSRegistration)
	registerIMS("ims/5g_not_entitled", imsNotEntitledSubscriber, runIMSNotEntitled)
	registerIMS("ims/5g_call", imsCaller, runIMSCall, imsCallee)
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
	g, registration, local, err := registerIMSUE(env, imsRegistrationSubscriber, scenarios.IMSDNN, scenarios.IMSUEIPv4Pool, scenarios.IMSUEIPv6Pool)
	if err != nil {
		return err
	}

	defer closeIMSUE(g, registration)

	if !registration.IMSVoPS {
		return fmt.Errorf("the Registration Accept does not indicate IMS voice over PS sessions")
	}

	acc, err := fgs.ParsePDUSessionEstablishmentAccept(registration.Session.Accept)
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
	g, registration, local, err := registerIMSUE(env, imsNotEntitledSubscriber, scenarios.DefaultDNN, scenarios.DefaultUEIPv4Pool, scenarios.DefaultUEIPv6Pool)
	if err != nil {
		return err
	}

	defer closeIMSUE(g, registration)

	if registration.IMSVoPS {
		return fmt.Errorf("the Registration Accept indicates IMS voice over PS sessions to a subscriber without the ims data network")
	}

	pcscf := scenarios.IMSServerAddress(!env.HasIPv4())

	return scenarios.RequireIMSRegistrationForbidden(ctx, imsNotEntitledSubscriber, pcscf, local, scenarios.IMSTransports[0])
}

func registerIMSUE(env scenarios.Env, s scenarios.SubscriberSpec, dnn, v4Pool, v6Pool string) (*gnb.GnodeB, *gnb.RegistrationResult, netip.Addr, error) {
	g, err := startGNB(env)
	if err != nil {
		return nil, nil, netip.Addr{}, err
	}

	_, registration, local, err := registerAndTunnelIMSUE(env, g, s, int64(scenarios.DefaultRANUENGAPID), imsTunIface, dnn, v4Pool, v6Pool)
	if err != nil {
		g.Close()
		return nil, nil, netip.Addr{}, err
	}

	return g, registration, local, nil
}

func registerAndTunnelIMSUE(env scenarios.Env, g *gnb.GnodeB, s scenarios.SubscriberSpec, ranUENGAPID int64, tunIface, dnn, v4Pool, v6Pool string) (*ue.UE, *gnb.RegistrationResult, netip.Addr, error) {
	sub := subscriber{IMSI: s.IMSI, Key: s.Key, OPc: s.OPc, SequenceNumber: s.SequenceNumber, ProfileName: s.ProfileName}

	newUE, err := newSubscriberUE(g, sub, dnn, env.PDUSessionType())
	if err != nil {
		return nil, nil, netip.Addr{}, err
	}

	g.AddUE(ranUENGAPID, newUE)

	registration, err := g.Register(newUE, ranUENGAPID, scenarios.DefaultPDUSessionID, registrationTimeout)
	if err != nil {
		return nil, nil, netip.Addr{}, fmt.Errorf("registration failed: %w", err)
	}

	if err := tunnelIMSSession(env, g, registration.Session, tunIface, v6Pool); err != nil {
		return nil, nil, netip.Addr{}, err
	}

	pool := v4Pool
	if !env.HasIPv4() {
		pool = v6Pool
	}

	local, err := scenarios.UEAddress(tunIface, netip.MustParsePrefix(pool))
	if err != nil {
		return nil, nil, netip.Addr{}, err
	}

	return newUE, registration, local, nil
}

func tunnelIMSSession(env scenarios.Env, g *gnb.GnodeB, session gnb.PDUSessionResult, tunIface, v6Pool string) error {
	ipv6 := !env.HasIPv4()

	tun := &gnb.TunnelOpts{
		UpfAddress:       session.UpfAddress,
		TunInterfaceName: tunIface,
		ULTEID:           session.ULTEID,
		DLTEID:           session.DLTEID,
		QFI:              session.QFI,
		ExtraRoutes:      []string{scenarios.IMSRoute(scenarios.IMSServerAddress(ipv6))},
	}

	if ipv6 {
		tun.UEIPv6 = session.UEIPv6 + "/64"
	} else {
		tun.UEIPv4 = session.UEIPv4 + "/16"
	}

	if err := g.AddTunnel(tun); err != nil {
		return fmt.Errorf("add GTP tunnel: %w", err)
	}

	if ipv6 {
		if err := gnb.WaitForULAAddr(tunIface, v6Pool, 5*time.Second); err != nil {
			return fmt.Errorf("await SLAAC address: %w", err)
		}
	} else {
		time.Sleep(scenarios.DatapathSettleDelay)
	}

	return nil
}

type imsCallUE struct {
	ue    *ue.UE
	leg   voiceLeg
	iface string
	table int
}

func attachIMSCallUEs(env scenarios.Env, g *gnb.GnodeB, subs ...scenarios.SubscriberSpec) ([]scenarios.IMSEndpoint, []imsCallUE, func(), error) {
	var (
		endpoints []scenarios.IMSEndpoint
		ues       []imsCallUE
		cleanups  []func()
	)

	cleanup := func() {
		for _, c := range slices.Backward(cleanups) {
			c()
		}
	}

	for i, sub := range subs {
		tunIface := fmt.Sprintf("%sims%d", gtpInterfaceNamePrefix, i)
		ranUEID := int64(scenarios.DefaultRANUENGAPID + i)

		u, registration, local, err := registerAndTunnelIMSUE(env, g, sub, ranUEID, tunIface, scenarios.IMSDNN, scenarios.IMSUEIPv4Pool, scenarios.IMSUEIPv6Pool)
		if err != nil {
			cleanup()
			return nil, nil, nil, fmt.Errorf("UE %s: %w", sub.IMSI, err)
		}

		cleanups = append(cleanups, func() { g.CloseTunnel(registration.Session.DLTEID) })

		acc, err := fgs.ParsePDUSessionEstablishmentAccept(registration.Session.Accept)
		if err != nil {
			cleanup()
			return nil, nil, nil, fmt.Errorf("parse PDU Session Establishment Accept: %w", err)
		}

		if acc.ExtendedPCO == nil {
			cleanup()
			return nil, nil, nil, fmt.Errorf("the PDU Session Establishment Accept has no ePCO")
		}

		pcscf, err := scenarios.SelectPCSCF(acc.ExtendedPCO.PCSCFAddresses(), !env.HasIPv4())
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
		ues = append(ues, imsCallUE{
			ue:    u,
			leg:   voiceLeg{gnb: g, ranUEID: ranUEID, session: registration.Session},
			iface: tunIface,
			table: imsRouteTable + i,
		})
	}

	return endpoints, ues, cleanup, nil
}

func callLegs(ues []imsCallUE) []voiceLeg {
	legs := make([]voiceLeg, 0, len(ues))
	for _, u := range ues {
		legs = append(legs, u.leg)
	}

	return legs
}

func runIMSCall(ctx context.Context, env scenarios.Env) error {
	g, err := startGNB(env)
	if err != nil {
		return err
	}

	defer g.Close()

	endpoints, ues, cleanup, err := attachIMSCallUEs(env, g, imsCaller, imsCallee)
	if err != nil {
		return err
	}

	defer cleanup()

	legs := callLegs(ues)

	for _, transport := range scenarios.IMSTransports {
		if err := scenarios.RequireIMSCallWithMedia(ctx, endpoints[0], endpoints[1], transport, qosFlowMedia(legs)); err != nil {
			return err
		}

		if err := awaitQoSFlowsReleased(ctx, legs); err != nil {
			return fmt.Errorf("after the call over %s: %w", transport, err)
		}
	}

	release := func() error {
		return g.ReleasePDUSession(ues[1].ue, ues[1].leg.ranUEID, scenarios.DefaultPDUSessionID, releaseTimeout)
	}

	return scenarios.RequireIMSSignallingLoss(ctx, endpoints[0], endpoints[1], scenarios.IMSTransports[0], release)
}

func closeIMSUE(g *gnb.GnodeB, registration *gnb.RegistrationResult) {
	g.CloseTunnel(registration.Session.DLTEID)
	g.Close()
}

const (
	qosFlowTimeout      = 10 * time.Second
	voiceMediaTimeout   = 5 * time.Second
	voicePacketInterval = 20 * time.Millisecond
	gateSettleInterval  = 300 * time.Millisecond
	gatedProbes         = 3
)

var voicePayload = []byte{0x80, 0x76, 0, 1, 0, 0, 0, 0, 0, 0, 0, 1}

type voiceLeg struct {
	gnb     *gnb.GnodeB
	ranUEID int64
	session gnb.PDUSessionResult
}

func (l voiceLeg) flows() []uint8 {
	return l.gnb.QoSFlows(l.ranUEID, int64(l.session.PDUSessionID))
}

func awaitVoiceQFIs(ctx context.Context, legs []voiceLeg) ([]uint8, error) {
	ctx, cancel := context.WithTimeout(ctx, qosFlowTimeout)
	defer cancel()

	for {
		qfis := make([]uint8, 0, len(legs))

		for _, l := range legs {
			if flows := l.flows(); len(flows) == 1 {
				qfis = append(qfis, flows[0])
			}
		}

		if len(qfis) == len(legs) {
			return qfis, nil
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("the gNB holds %d of %d voice QoS flows: %w", len(qfis), len(legs), ctx.Err())
		case <-time.After(voicePacketInterval):
		}
	}
}

func awaitQoSFlowsReleased(ctx context.Context, legs []voiceLeg) error {
	ctx, cancel := context.WithTimeout(ctx, qosFlowTimeout)
	defer cancel()

	for {
		held := 0

		for _, l := range legs {
			held += len(l.flows())
		}

		if held == 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("the gNB still holds %d voice QoS flows: %w", held, ctx.Err())
		case <-time.After(voicePacketInterval):
		}
	}
}

func qosFlowMedia(legs []voiceLeg) func(context.Context, scenarios.IMSMedia) error {
	return func(ctx context.Context, m scenarios.IMSMedia) error {
		qfis, err := awaitVoiceQFIs(ctx, legs)
		if err != nil {
			return err
		}

		return sendVoiceBothWays(ctx, legs, qfis, m)
	}
}

func sendVoiceBothWays(ctx context.Context, legs []voiceLeg, qfis []uint8, m scenarios.IMSMedia) error {
	if err := sendVoiceOnQFI(ctx, legs[0], qfis[0], legs[1], qfis[1], m.Caller, m.Callee); err != nil {
		return fmt.Errorf("caller to callee: %w", err)
	}

	if err := sendVoiceOnQFI(ctx, legs[1], qfis[1], legs[0], qfis[0], m.Callee, m.Caller); err != nil {
		return fmt.Errorf("callee to caller: %w", err)
	}

	return nil
}

func sendVoiceOnQFI(ctx context.Context, from voiceLeg, fromQFI uint8, to voiceLeg, toQFI uint8, src, dst sdp.Endpoint) error {
	upf, err := netip.ParseAddr(from.session.UpfAddress)
	if err != nil {
		return fmt.Errorf("N3 peer %q: %w", from.session.UpfAddress, err)
	}

	to.gnb.WatchTEID(to.session.DLTEID)

	before := to.gnb.DownlinkQFICount(to.session.DLTEID, toQFI)
	packet := scenarios.UDPPacket(netip.AddrPortFrom(src.Addr, src.Port), netip.AddrPortFrom(dst.Addr, dst.Port), voicePayload)

	ctx, cancel := context.WithTimeout(ctx, voiceMediaTimeout)
	defer cancel()

	for {
		if err := from.gnb.SendGPDUWithQFI(from.session.ULTEID, upf, fromQFI, packet); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("UDP %s -> %s sent on QFI %d never reached the peer's QFI %d (TS 38.415 §5.5.2): %w", src, dst, fromQFI, toQFI, ctx.Err())
		case <-time.After(voicePacketInterval):
		}

		if to.gnb.DownlinkQFICount(to.session.DLTEID, toQFI) > before {
			return nil
		}
	}
}

func requireVoiceGated(ctx context.Context, from voiceLeg, fromQFI uint8, to voiceLeg, src, dst sdp.Endpoint) error {
	upf, err := netip.ParseAddr(from.session.UpfAddress)
	if err != nil {
		return fmt.Errorf("N3 peer %q: %w", from.session.UpfAddress, err)
	}

	to.gnb.WatchTEID(to.session.DLTEID)

	packet := scenarios.UDPPacket(netip.AddrPortFrom(src.Addr, src.Port), netip.AddrPortFrom(dst.Addr, dst.Port), voicePayload)

	ctx, cancel := context.WithTimeout(ctx, voiceMediaTimeout)
	defer cancel()

	dropped := 0

	for dropped < gatedProbes {
		before := to.gnb.DownlinkCount(to.session.DLTEID)

		if err := from.gnb.SendGPDUWithQFI(from.session.ULTEID, upf, fromQFI, packet); err != nil {
			return err
		}

		select {
		case <-time.After(gateSettleInterval):
		case <-ctx.Done():
			return fmt.Errorf("UDP %s -> %s still reaches the peer: %w", src, dst, ctx.Err())
		}

		dropped++
		if to.gnb.DownlinkCount(to.session.DLTEID) != before {
			dropped = 0
		}
	}

	return nil
}
