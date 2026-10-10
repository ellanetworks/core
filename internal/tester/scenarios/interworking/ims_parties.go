// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/s1ap"
	"github.com/ellanetworks/ims/sip/sdp"
	"github.com/spf13/pflag"
)

const (
	imsPDUSessionID     = scenarios.DefaultPDUSessionID
	imsDefaultEBI       = s1ap.ERABID(5)
	imsVoiceEBI         = s1ap.ERABID(6)
	imsVoiceQCI         = 1
	imsTunIface         = "iwkims"
	imsRouteTable       = 120
	imsVoiceTimeout     = 30 * time.Second
	imsMediaTimeout     = 5 * time.Second
	imsPacketInterval   = 20 * time.Millisecond
	imsFirstGNBRANUEID  = int64(scenarios.DefaultRANUENGAPID)
	imsMovedGNBRANUEIDs = int64(90)
)

var (
	imsVoicePayload = []byte{0x80, 0x76, 0, 1, 0, 0, 0, 0, 0, 0, 0, 1}
	imsRefusal      = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkNoRadioResourcesInTargetCell}
)

func registerIMS(name string, run func(context.Context, scenarios.Env) error, subs ...scenarios.SubscriberSpec) {
	scenarios.Register(scenarios.Scenario{
		Name:      name,
		BindFlags: func(_ *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return run(ctx, env)
		},
		Fixture: func(env scenarios.Env) scenarios.FixtureSpec {
			return scenarios.IMSFixture(env, subs...)
		},
	})
}

type nrAttachment struct {
	g       *gnb.GnodeB
	u       *ue.UE
	ranUEID int64
	session gnb.PDUSessionResult
}

type lteAttachment struct {
	e       *s1enb.ENB
	u       *s1enb.UE
	mmeUEID int64
	enbUEID int64
	upf     string
	ulTEID  uint32
	dlTEID  uint32
}

type imsParty struct {
	env     scenarios.Env
	sub     scenarios.SubscriberSpec
	ep      scenarios.IMSEndpoint
	iface   string
	table   int
	ueIPv4  string
	ueIPv6  string
	nr      *nrAttachment
	lte     *lteAttachment
	guti    *eps.EPSMobileIdentity
	unroute func()
}

func (p *imsParty) close() {
	p.closeTunnel()

	if p.unroute != nil {
		p.unroute()
	}
}

func (p *imsParty) closeTunnel() {
	switch {
	case p.nr != nil:
		p.nr.g.CloseTunnel(p.nr.session.DLTEID)
	case p.lte != nil:
		p.lte.e.CloseTunnel(p.lte.dlTEID)
	}
}

func (p *imsParty) pool() netip.Prefix {
	if p.env.HasIPv4() {
		return netip.MustParsePrefix(scenarios.IMSUEIPv4Pool)
	}

	return netip.MustParsePrefix(scenarios.IMSUEIPv6Pool)
}

func newIMSUEOn5GS(g *gnb.GnodeB, env scenarios.Env, sub scenarios.SubscriberSpec, autoSession bool) (*ue.UE, error) {
	u, err := ue.NewUE(&ue.UEOpts{
		GnodeB:           g,
		PDUSessionID:     imsPDUSessionID,
		PDUSessionType:   fgs.PDUSessionType(env.PDUSessionType()),
		NoAutoPDUSession: !autoSession,
		Msin:             sub.IMSI[5:],
		K:                sub.Key,
		OpC:              sub.OPc,
		Amf:              scenarios.DefaultAMF,
		Sqn:              scenarios.DefaultSequenceNumber,
		Mcc:              scenarios.DefaultMCC,
		Mnc:              scenarios.DefaultMNC,
		HomeNetworkPublicKey: sidf.HomeNetworkPublicKey{
			ProtectionScheme: sidf.NullScheme,
			PublicKeyID:      "0",
		},
		RoutingIndicator: scenarios.DefaultRoutingIndicator,
		DNN:              scenarios.IMSDNN,
		Sst:              scenarios.DefaultSST,
		Sd:               scenarios.DefaultSD,
		IMEISV:           scenarios.DefaultIMEISV,
		UeSecurityCapability: testutil.GetUESecurityCapability(&testutil.UeSecurityCapability{
			Integrity: testutil.IntegrityAlgorithms{Nia2: true},
			Ciphering: testutil.CipheringAlgorithms{Nea0: true, Nea2: true},
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("create UE %s: %w", sub.IMSI, err)
	}

	return u, nil
}

func registerIMSPartyOn5GS(env scenarios.Env, g *gnb.GnodeB, sub scenarios.SubscriberSpec, ranUEID int64, index int) (*imsParty, error) {
	u, err := newIMSUEOn5GS(g, env, sub, true)
	if err != nil {
		return nil, err
	}

	g.AddUE(ranUEID, u)

	reg, err := g.Register(u, ranUEID, imsPDUSessionID, registrationTimeout)
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
		env:    env,
		sub:    sub,
		iface:  fmt.Sprintf("%s%d", imsTunIface, index),
		table:  imsRouteTable + index,
		ueIPv4: reg.Session.UEIPv4,
		ueIPv6: reg.Session.UEIPv6,
		nr:     &nrAttachment{g: g, u: u, ranUEID: ranUEID, session: reg.Session},
	}

	pcscf, err := scenarios.SelectPCSCF(acc.ExtendedPCO.PCSCFAddresses(), !env.HasIPv4())
	if err != nil {
		return nil, err
	}

	return p, p.route(pcscf)
}

func attachIMSPartyOn4G(env scenarios.Env, e *s1enb.ENB, sub scenarios.SubscriberSpec, index int) (*imsParty, error) {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return nil, err
	}

	u := e.NewUE(sub.IMSI, k, opc)
	u.RequestPDNType(env.PDUSessionType())
	u.RequestPCSCFAddresses()
	u.AnnounceN1Mode(imsPDUSessionID)

	res, err := e.Attach(u, attachTimeout)
	if err != nil {
		return nil, fmt.Errorf("UE %s: attach over E-UTRAN: %w", sub.IMSI, err)
	}

	p := &imsParty{
		env:    env,
		sub:    sub,
		iface:  fmt.Sprintf("%s%d", imsTunIface, index),
		table:  imsRouteTable + index,
		ueIPv4: res.UEIPv4,
		ueIPv6: res.UEIPv6,
		guti:   res.GUTI,
		lte: &lteAttachment{
			e: e, u: u, mmeUEID: res.MMEUES1APID, enbUEID: res.ENBUES1APID,
			upf: res.UpfAddress, ulTEID: res.ULTEID, dlTEID: res.DLTEID,
		},
	}

	pcscf, err := scenarios.SelectPCSCF(res.PCSCF, !env.HasIPv4())
	if err != nil {
		return nil, err
	}

	return p, p.route(pcscf)
}

func (p *imsParty) route(pcscf netip.Addr) error {
	if err := p.tunnel(); err != nil {
		return err
	}

	local, err := scenarios.UEAddress(p.iface, p.pool())
	if err != nil {
		return err
	}

	unroute, err := scenarios.RouteFromUE(p.iface, local, pcscf, p.table)
	if err != nil {
		return err
	}

	p.unroute = unroute
	p.ep = scenarios.IMSEndpoint{Subscriber: p.sub, PCSCF: pcscf, Local: local}

	return nil
}

func (p *imsParty) tunnel() error {
	if err := p.openTunnel(); err != nil {
		return err
	}

	if !p.env.HasIPv4() {
		if p.nr != nil {
			return gnb.WaitForULAAddr(p.iface, scenarios.IMSUEIPv6Pool, slaacTimeout)
		}

		return s1enb.WaitForULAAddr(p.iface, scenarios.IMSUEIPv6Pool, slaacTimeout)
	}

	time.Sleep(scenarios.DatapathSettleDelay)

	return nil
}

func (p *imsParty) openTunnel() error {
	ipv6 := !p.env.HasIPv4()
	extra := []string{scenarios.IMSRoute(scenarios.IMSServerAddress(ipv6))}

	var v4, v6 string

	if ipv6 {
		v6 = p.ueIPv6 + ipv6TunPrefix
	} else {
		v4 = p.ueIPv4 + ipv4TunPrefix
	}

	switch {
	case p.nr != nil:
		s := p.nr.session
		if err := p.nr.g.AddTunnel(&gnb.TunnelOpts{
			UpfAddress: s.UpfAddress, TunInterfaceName: p.iface, ULTEID: s.ULTEID, DLTEID: s.DLTEID, QFI: s.QFI,
			UEIPv4: v4, UEIPv6: v6, ExtraRoutes: extra,
		}); err != nil {
			return fmt.Errorf("UE %s: add the N3 tunnel: %w", p.sub.IMSI, err)
		}
	case p.lte != nil:
		l := p.lte
		if err := l.e.AddTunnel(&s1enb.TunnelOpts{
			UpfAddress: l.upf, TunInterfaceName: p.iface, ULTEID: l.ulTEID, DLTEID: l.dlTEID,
			UEIPv4: v4, UEIPv6: v6, ExtraRoutes: extra,
		}); err != nil {
			return fmt.Errorf("UE %s: add the S1-U tunnel: %w", p.sub.IMSI, err)
		}
	}

	return nil
}

func (p *imsParty) rehome(nr *nrAttachment, lte *lteAttachment) error {
	p.closeTunnel()

	p.nr, p.lte = nr, lte

	if err := p.openTunnel(); err != nil {
		return err
	}

	if err := scenarios.RerouteToIMS(p.iface, p.ep.Local, p.ep.PCSCF, p.table); err != nil {
		return err
	}

	time.Sleep(scenarios.DatapathSettleDelay)

	return nil
}

type mediaLeg interface {
	send(packet []byte) error
	delivered() int
	String() string
}

type nrFlow struct {
	a   *nrAttachment
	qfi uint8
}

func (l nrFlow) send(packet []byte) error {
	upf, err := netip.ParseAddr(l.a.session.UpfAddress)
	if err != nil {
		return fmt.Errorf("N3 peer %q: %w", l.a.session.UpfAddress, err)
	}

	return l.a.g.SendGPDUWithQFI(l.a.session.ULTEID, upf, l.qfi, packet)
}

func (l nrFlow) delivered() int {
	l.a.g.WatchTEID(l.a.session.DLTEID)
	return l.a.g.DownlinkQFICount(l.a.session.DLTEID, l.qfi)
}

func (l nrFlow) String() string { return fmt.Sprintf("QFI %d", l.qfi) }

type lteBearer struct {
	a      *lteAttachment
	erab   s1ap.ERABID
	ulTEID uint32
	dlTEID uint32
}

func (l lteBearer) send(packet []byte) error {
	upf, err := netip.ParseAddr(l.a.upf)
	if err != nil {
		return fmt.Errorf("S1-U peer %q: %w", l.a.upf, err)
	}

	return l.a.e.SendGPDU(l.ulTEID, upf, packet)
}

func (l lteBearer) delivered() int {
	l.a.e.WatchTEID(l.dlTEID)
	return l.a.e.WatchedTEIDCount(l.dlTEID)
}

func (l lteBearer) String() string { return fmt.Sprintf("E-RAB %d", l.erab) }

func sendVoice(ctx context.Context, from, to mediaLeg, src, dst sdp.Endpoint) error {
	before := to.delivered()
	packet := scenarios.UDPPacket(netip.AddrPortFrom(src.Addr, src.Port), netip.AddrPortFrom(dst.Addr, dst.Port), imsVoicePayload)

	ctx, cancel := context.WithTimeout(ctx, imsMediaTimeout)
	defer cancel()

	for {
		if err := from.send(packet); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("UDP %s -> %s sent on %s never reached the peer's %s: %w", src, dst, from, to, ctx.Err())
		case <-time.After(imsPacketInterval):
		}

		if to.delivered() > before {
			return nil
		}
	}
}

func sendVoiceBothWays(ctx context.Context, caller, callee mediaLeg, m scenarios.IMSMedia) error {
	if err := sendVoice(ctx, caller, callee, m.Caller, m.Callee); err != nil {
		return fmt.Errorf("caller to callee: %w", err)
	}

	if err := sendVoice(ctx, callee, caller, m.Callee, m.Caller); err != nil {
		return fmt.Errorf("callee to caller: %w", err)
	}

	return nil
}

func (a *nrAttachment) awaitVoiceQFI(ctx context.Context) (uint8, error) {
	ctx, cancel := context.WithTimeout(ctx, imsVoiceTimeout)
	defer cancel()

	for {
		if flows := a.g.QoSFlows(a.ranUEID, int64(imsPDUSessionID)); len(flows) == 1 {
			return flows[0], nil
		}

		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("the gNB holds QoS flows %v, want one voice flow: %w", a.g.QoSFlows(a.ranUEID, int64(imsPDUSessionID)), ctx.Err())
		case <-time.After(imsPacketInterval):
		}
	}
}

type acceptedBearer struct {
	bearer *s1enb.DedicatedBearer
	err    error
}

func (a *lteAttachment) acceptVoiceBearer() <-chan acceptedBearer {
	out := make(chan acceptedBearer, 1)

	go func() {
		b, err := a.e.AcceptDedicatedBearer(a.u, a.enbUEID, imsVoiceTimeout)
		if err == nil && (b.QoS.QCI != imsVoiceQCI || b.QoS.GBR == nil) {
			err = fmt.Errorf("the voice bearer has QCI %d and GBR %+v, want a QCI 1 GBR bearer", b.QoS.QCI, b.QoS.GBR)
		}

		out <- acceptedBearer{bearer: b, err: err}
	}()

	return out
}

func awaitBearer(ctx context.Context, ch <-chan acceptedBearer) (*s1enb.DedicatedBearer, error) {
	select {
	case r := <-ch:
		return r.bearer, r.err
	case <-ctx.Done():
		return nil, fmt.Errorf("no voice bearer was set up: %w", ctx.Err())
	}
}

func (a *lteAttachment) voice(b *s1enb.DedicatedBearer) lteBearer {
	return lteBearer{a: a, erab: b.ERABID, ulTEID: b.ULTEID, dlTEID: b.DLTEID}
}

func (a *lteAttachment) awaitVoiceRelease(erab s1ap.ERABID) error {
	got, _, err := a.e.AwaitDedicatedDeactivation(a.u, a.enbUEID, imsVoiceTimeout)

	switch {
	case err != nil:
		return fmt.Errorf("await the voice bearer release: %w", err)
	case got != erab:
		return fmt.Errorf("released E-RAB %d, want the voice bearer %d", got, erab)
	}

	return nil
}

type epsHandover struct {
	lte     *lteAttachment
	voice   *lteBearer
	bearers map[uint8]*lteBearer
}

type epsHandoverOpts struct {
	carried   map[uint8]s1ap.ERABID
	refuseQCI uint8
}

func (p *imsParty) handOverToEPS(e *s1enb.ENB, wantVoice, refuseVoice bool) (epsHandover, error) {
	opts := epsHandoverOpts{}

	if wantVoice {
		opts.carried = map[uint8]s1ap.ERABID{imsVoiceQCI: imsVoiceEBI}
	}

	if refuseVoice {
		opts.refuseQCI = imsVoiceQCI
	}

	h, err := p.handOverToEPSWith(e, opts)
	if err == nil {
		h.voice = h.bearers[imsVoiceQCI]
	}

	return h, err
}

func (p *imsParty) handOverToEPSWith(e *s1enb.ENB, opts epsHandoverOpts) (epsHandover, error) {
	nr := p.nr

	enbID, err := interworkingENBID()
	if err != nil {
		return epsHandover{}, err
	}

	if err := nr.g.SendHandoverRequiredToEPS(&gnb.HandoverToEPSOpts{
		AMFUENGAPID:   nr.g.GetAMFUENGAPID(nr.ranUEID),
		RANUENGAPID:   nr.ranUEID,
		TargetMcc:     scenarios.DefaultMCC,
		TargetMnc:     scenarios.DefaultMNC,
		TargetTac:     scenarios.DefaultTAC,
		TargetENBID:   enbID,
		PDUSessionIDs: []int64{int64(imsPDUSessionID)},
	}); err != nil {
		return epsHandover{}, fmt.Errorf("send Handover Required to EPS: %w", err)
	}

	req, err := e.WaitForHandoverRequest(handoverTimeout)
	if err != nil {
		return epsHandover{}, fmt.Errorf("the target eNB got no Handover Request: %w", err)
	}

	def := slices.IndexFunc(req.ERABToBeSetup, func(it s1ap.ERABToBeSetupItemHOReq) bool { return it.ERABID == imsDefaultEBI })
	if def < 0 {
		return epsHandover{}, fmt.Errorf("the Handover Request E-RABs %+v lack the default bearer %d", req.ERABToBeSetup, imsDefaultEBI)
	}

	if len(req.ERABToBeSetup) != 1+len(opts.carried) {
		return epsHandover{}, fmt.Errorf("the Handover Request E-RABs are %+v, want the default bearer and dedicated bearers %v (TS 23.502 §4.11.1.2.1 step 2)", req.ERABToBeSetup, opts.carried)
	}

	admit := []s1ap.ERABID{imsDefaultEBI}

	var refuse []s1ap.ERABID

	for i, it := range req.ERABToBeSetup {
		if i == def {
			continue
		}

		qci := uint8(it.QoS.QCI)

		ebi, ok := opts.carried[qci]

		switch {
		case !ok:
			return epsHandover{}, fmt.Errorf("the Handover Request carries E-RAB %d at QCI %d, want dedicated bearers %v", it.ERABID, qci, opts.carried)
		case ebi != 0 && it.ERABID != ebi:
			return epsHandover{}, fmt.Errorf("the QCI %d bearer is E-RAB %d, want the flow's EBI %d kept (TS 23.502 §4.11.1.1)", qci, it.ERABID, ebi)
		case it.QoS.GBR == nil:
			return epsHandover{}, fmt.Errorf("the QCI %d E-RAB %+v has no GBR QoS Information", qci, it.QoS)
		case qci == opts.refuseQCI:
			refuse = append(refuse, it.ERABID)
		default:
			admit = append(admit, it.ERABID)
		}
	}

	mmeUEID := int64(req.MMEUES1APID)
	enbUEID := e.AllocateENBUEID()

	teids, err := e.SendHandoverRequestAcknowledgePartial(enbUEID, mmeUEID, admit, refuse, imsRefusal)
	if err != nil {
		return epsHandover{}, fmt.Errorf("send Handover Request Acknowledge: %w", err)
	}

	cmd, err := nr.g.WaitForHandoverToEPSCommand(handoverTimeout)
	if err != nil {
		return epsHandover{}, fmt.Errorf("the source gNB got no Handover Command: %w", err)
	}

	mapped, err := installMappedContext(nr.u, cmd)
	if err != nil {
		return epsHandover{}, err
	}

	if err := e.SendHandoverNotify(enbUEID, mmeUEID); err != nil {
		return epsHandover{}, fmt.Errorf("send Handover Notify: %w", err)
	}

	anchor, err := e.AnchorAddress(req.ERABToBeSetup[def].TransportLayerAddress)
	if err != nil {
		return epsHandover{}, fmt.Errorf("read the anchor S1-U address: %w", err)
	}

	if nr.u.UeSecurity.Guti == nil || nr.u.UeSecurity.Guti.GUTI == nil {
		return epsHandover{}, errors.New("the UE holds no 5G-GUTI to map into a tracking area update")
	}

	guti, err := e.TrackingAreaUpdateAfterHandover(mapped, mmeUEID, enbUEID, etsi.MapGUTI5GToEPS(*nr.u.UeSecurity.Guti.GUTI), handoverTimeout)
	if err != nil {
		return epsHandover{}, fmt.Errorf("tracking area update after the handover: %w", err)
	}

	lte := &lteAttachment{
		e: e, u: mapped, mmeUEID: mmeUEID, enbUEID: enbUEID, upf: anchor,
		ulTEID: uint32(req.ERABToBeSetup[def].GTPTEID), dlTEID: teids[imsDefaultEBI],
	}

	out := epsHandover{lte: lte, bearers: make(map[uint8]*lteBearer)}

	for i, it := range req.ERABToBeSetup {
		if i == def || slices.Contains(refuse, it.ERABID) {
			continue
		}

		out.bearers[uint8(it.QoS.QCI)] = &lteBearer{a: lte, erab: it.ERABID, ulTEID: uint32(it.GTPTEID), dlTEID: teids[it.ERABID]}
	}

	p.guti = guti

	return out, p.rehome(nil, lte)
}

type fiveGSHandover struct {
	nr    *nrAttachment
	voice *nrFlow
	flows map[int64]*nrFlow
}

type carriedFlow struct {
	ebi s1ap.ERABID
	qfi uint8
}

type fiveGSHandoverOpts struct {
	carried      map[int64]carriedFlow
	refuseFiveQI int64
}

func (p *imsParty) handOverTo5GS(g *gnb.GnodeB, ranUEID int64, wantQFI uint8, refuseVoice bool) (fiveGSHandover, error) {
	opts := fiveGSHandoverOpts{carried: map[int64]carriedFlow{imsVoiceQCI: {ebi: imsVoiceEBI, qfi: wantQFI}}}

	if refuseVoice {
		opts.refuseFiveQI = imsVoiceQCI
	}

	h, err := p.handOverTo5GSWith(g, ranUEID, opts)
	if err == nil {
		h.voice = h.flows[imsVoiceQCI]
	}

	return h, err
}

func (p *imsParty) handOverTo5GSWith(g *gnb.GnodeB, ranUEID int64, opts fiveGSHandoverOpts) (fiveGSHandover, error) {
	lte := p.lte

	if err := handoverRequiredToFiveGS(lte.e, &s1enb.AttachResult{ENBUES1APID: lte.enbUEID, MMEUES1APID: lte.mmeUEID}); err != nil {
		return fiveGSHandover{}, err
	}

	req, err := g.WaitForHandoverRequest(handoverTimeout)
	if err != nil {
		return fiveGSHandover{}, fmt.Errorf("the target gNB got no Handover Request: %w", err)
	}

	container, err := checkArrivingHandoverRequest(req)
	if err != nil {
		return fiveGSHandover{}, err
	}

	transfer, err := ngap.ParsePDUSessionResourceSetupRequestTransfer(req.PDUSessionResourceSetupListHOReq[0].Transfer)
	if err != nil {
		return fiveGSHandover{}, fmt.Errorf("parse the Handover Request's session transfer: %w", err)
	}

	qfis := make(map[int64]uint8)

	var refused []uint8

	for _, flow := range transfer.QosFlowSetupRequest {
		if flow.QosFlowLevelQosParameters.GBRQosInformation == nil {
			continue
		}

		fiveQI := int64(flow.QosFlowLevelQosParameters.QosCharacteristics.NonDynamic5QI.FiveQI)
		qfi := uint8(flow.QosFlowIdentifier)
		want, ok := opts.carried[fiveQI]

		switch {
		case !ok:
			return fiveGSHandover{}, fmt.Errorf("the Handover Request carries QFI %d at 5QI %d, want flows %v", qfi, fiveQI, opts.carried)
		case flow.ERABID == nil || want.ebi != 0 && s1ap.ERABID(*flow.ERABID) != want.ebi:
			return fiveGSHandover{}, fmt.Errorf("the 5QI %d flow carries E-RAB ID %v, want the bearer's EBI %d (TS 38.413 §9.3.4.1)", fiveQI, flow.ERABID, want.ebi)
		case want.qfi != 0 && qfi != want.qfi:
			return fiveGSHandover{}, fmt.Errorf("the 5QI %d flow is QFI %d, want the QFI %d it held on 5GS (TS 23.502 §4.11.1.1)", fiveQI, qfi, want.qfi)
		}

		qfis[fiveQI] = qfi

		if fiveQI == opts.refuseFiveQI {
			refused = append(refused, qfi)
		}
	}

	if len(qfis) != len(opts.carried) {
		return fiveGSHandover{}, fmt.Errorf("the Handover Request flows %+v, want flows %v (TS 23.502 §4.11.1.2.2.2 step 6)", transfer.QosFlowSetupRequest, opts.carried)
	}

	nasc, err := container.MarshalBinary()
	if err != nil {
		return fiveGSHandover{}, fmt.Errorf("re-encode the NAS container: %w", err)
	}

	admitted, err := g.AdmitHandover(&gnb.HandoverAdmissionOpts{Request: req, RANUENGAPID: ranUEID, TargetToSource: nasc, RefusedQFIs: refused})
	if err != nil {
		return fiveGSHandover{}, fmt.Errorf("admit the handover at the target gNB: %w", err)
	}

	cmd, err := lte.e.WaitForHandoverCommand(lte.enbUEID, handoverTimeout)
	if err != nil {
		return fiveGSHandover{}, fmt.Errorf("the source eNB got no Handover Command: %w", err)
	}

	if len(cmd.ERABToRelease) != len(refused) {
		return fiveGSHandover{}, fmt.Errorf("the Handover Command releases E-RABs %+v, want only the bearers of refused flows %v (TS 36.413 §8.4.1.2)", cmd.ERABToRelease, refused)
	}

	if p.guti == nil || p.guti.GUTI == nil {
		return fiveGSHandover{}, errors.New("the UE holds no EPS GUTI to map into a 5G-GUTI")
	}

	u, err := newIMSUEOn5GS(g, p.env, p.sub, false)
	if err != nil {
		return fiveGSHandover{}, err
	}

	material, err := lte.u.SecurityContextForHandoverToFiveGS(container.NCC)
	if err != nil {
		return fiveGSHandover{}, fmt.Errorf("rebuild the EPS key material: %w", err)
	}

	if err := u.InstallMappedSecurityContextFromEPS(ue.MappedFromEPS{KASME: material.KASME, NH: material.NH, Container: container, EPS: epsAlgorithms(material)}); err != nil {
		return fiveGSHandover{}, fmt.Errorf("derive the mapped 5G security context: %w", err)
	}

	mappedGUTI := fgs.GUTIIdentity(etsi.MapGUTIEPSTo5G(*p.guti.GUTI))
	u.Set5gGuti(&mappedGUTI)
	g.AddUE(ranUEID, u)

	if err := g.SendHandoverNotify(&gnb.HandoverNotifyOpts{AMFUENGAPID: int64(req.AMFUENGAPID), RANUENGAPID: ranUEID}); err != nil {
		return fiveGSHandover{}, fmt.Errorf("send Handover Notify: %w", err)
	}

	if err := u.SendMobilityRegistrationUpdate(int64(req.AMFUENGAPID), ranUEID); err != nil {
		return fiveGSHandover{}, fmt.Errorf("mobility registration update over NR: %w", err)
	}

	if _, err := u.WaitForNASGMMMessage(uint8(fgs.MsgRegistrationAccept), attachTimeout); err != nil {
		return fiveGSHandover{}, fmt.Errorf("registration accept after the handover: %w", err)
	}

	if _, err := lte.e.WaitForUEContextReleaseCommand(lte.enbUEID, handoverTimeout); err != nil {
		return fiveGSHandover{}, fmt.Errorf("the source eNB was not told to release the UE: %w", err)
	}

	if err := lte.e.SendUEContextReleaseComplete(lte.mmeUEID, lte.enbUEID); err != nil {
		return fiveGSHandover{}, fmt.Errorf("send UE Context Release Complete: %w", err)
	}

	nr := &nrAttachment{g: g, u: u, ranUEID: ranUEID, session: admitted[0]}
	out := fiveGSHandover{nr: nr, flows: make(map[int64]*nrFlow)}

	for fiveQI, qfi := range qfis {
		if !slices.Contains(refused, qfi) {
			out.flows[fiveQI] = &nrFlow{a: nr, qfi: qfi}
		}
	}

	return out, p.rehome(nr, nil)
}

func interworkingENBID() (uint32, error) {
	id, err := strconv.ParseUint(scenarios.DefaultGNBID, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("parse eNB ID %q: %w", scenarios.DefaultGNBID, err)
	}

	return uint32(id), nil
}
