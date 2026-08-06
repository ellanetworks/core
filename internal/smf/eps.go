// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// The SMF (combined PGW-C, TS 23.501) names each 4G PDN connection by its
// default bearer's EPS bearer identity, in the converged key space that keeps
// it disjoint from the PDU session identities a UE allocates (epsBearerKey).
// Established sessions are addressed by Ref, as on the 5G side; the EPS bearer
// identity resolves a session only where no Ref exists yet.

// validateEPSBearerRequest rejects inputs the data path would otherwise accept
// and degrade: a zero AMBR programs a zero-rate QER, and a non-IP DNS drops the
// DNS option. An EBI outside 5..15 is not a valid default bearer (TS 24.007).
// It returns the parsed AMBR so the caller does not re-read the text form.
func validateEPSBearerRequest(req models.EPSBearerRequest) (models.Ambr, error) {
	var ambr models.Ambr

	if req.EPSBearerIdentity < 5 || req.EPSBearerIdentity > 15 {
		return ambr, fmt.Errorf("EPS bearer identity %d out of range (5..15)", req.EPSBearerIdentity)
	}

	if req.AMBRUplink.Kbps() == 0 {
		return ambr, fmt.Errorf("uplink AMBR %s is below 1 Kbps", req.AMBRUplink)
	}

	if req.AMBRDownlink.Kbps() == 0 {
		return ambr, fmt.Errorf("downlink AMBR %s is below 1 Kbps", req.AMBRDownlink)
	}

	if req.DNS != "" && net.ParseIP(req.DNS) == nil {
		return ambr, fmt.Errorf("invalid DNS address %q", req.DNS)
	}

	return models.Ambr{Uplink: req.AMBRUplink, Downlink: req.AMBRDownlink}, nil
}

// acceptUEPDUSessionID vets the PDU session identity the UE allocated for a PDN
// connection and sent in the PCO (TS 23.501 §5.17.2.1). It returns 0 — the PDN
// connection is then simply not transferable to 5GS, the case TS 23.502
// §4.11.1.1 NOTE 5 already covers — when the UE sent none, sent one outside the
// range it may allocate (TS 24.007 §11.2.3.1b), or sent one another of its live
// sessions holds. Honouring a duplicate would give two PDN connections one
// session key, hence one UE address.
func (s *SMF) acceptUEPDUSessionID(ctx context.Context, supi etsi.SUPI, pduSessionID uint8) uint8 {
	if pduSessionID == 0 {
		return 0
	}

	if pduSessionID > 15 {
		logger.WithTrace(ctx, logger.SmfLog).Warn("ignoring out-of-range PDU session id from PCO",
			logger.SUPI(supi.String()), logger.PDUSessionID(pduSessionID))

		return 0
	}

	if s.currentPDUSession(supi, pduSessionID) != nil {
		logger.WithTrace(ctx, logger.SmfLog).Warn("ignoring PDU session id from PCO already held by a live session",
			logger.SUPI(supi.String()), logger.PDUSessionID(pduSessionID))

		return 0
	}

	return pduSessionID
}

// CreateEPSSession programs the user plane for a 4G default EPS bearer with the
// SMF as converged anchor (SMF+PGW-C, TS 23.401). For IPv6/IPv4v6 the delegated
// /64 prefix reaches the UE via Router Advertisement only once ModifyEPSSession
// registers the IPv6 session. The returned S-GW S1-U F-TEID carries uplink.
func (s *SMF) CreateEPSSession(ctx context.Context, req models.EPSBearerRequest) (bearer models.EPSBearer, err error) {
	ctx, span := tracer.Start(ctx, "smf/create_eps_session",
		trace.WithAttributes(
			attribute.String("ue.imsi", req.IMSI),
			attribute.Int("eps.bearer_id", int(req.EPSBearerIdentity)),
			attribute.String("eps.apn", req.APN),
		),
	)
	defer span.End()

	defer func() { recordSessionEstablishment(metrics.RAT4G, err) }()

	supi, err := etsi.NewSUPIFromIMSI(req.IMSI)
	if err != nil {
		return models.EPSBearer{}, fmt.Errorf("invalid imsi %q: %w", req.IMSI, err)
	}

	ambr, err := validateEPSBearerRequest(req)
	if err != nil {
		return models.EPSBearer{}, err
	}

	policy := &Policy{
		PolicyID: req.PolicyID,
		Ambr:     ambr,
		IPv4Pool: req.IPv4Pool,
		IPv6Pool: req.IPv6Pool,
		DNS:      net.ParseIP(req.DNS),
		MTU:      req.MTU,
	}

	// The UE transfers a PDU session it holds in 5GS rather than asking for a new
	// PDN connection (TS 24.301 §6.5.1.2, TS 23.502 §4.11.2.2 step 13).
	if req.RequestType == eps.RequestTypeHandover {
		return s.transferToEPS(ctx, supi, req, policy)
	}

	pdnType, err := s.negotiatePDUSessionType(ctx, req.RequestedPDNType, policy)
	if err != nil {
		return models.EPSBearer{}, fmt.Errorf("negotiate PDN type: %w", err)
	}

	// Must precede establishSession: the superseded context's release frees the address by
	// (imsi, dnn, session key), which the new session would already hold (TS 24.301 §5.5.1.2.4 case f).
	if existing := s.currentEPSSession(supi, req.EPSBearerIdentity); existing != nil {
		s.handlePduSessionContextReplacement(ctx, existing)
	}

	sc, addrs, err := s.establishSession(ctx, SessionRequest{
		Supi:     supi,
		Identity: SessionIdentity{PDUSessionID: s.acceptUEPDUSessionID(ctx, supi, req.PDUSessionID), EBI: req.EPSBearerIdentity},
		Dnn:      req.APN,
		Snssai:   req.Snssai,
		Access:   Access4G,
		PDUType:  pdnType,
		Policy:   policy,
	})
	if err != nil {
		return models.EPSBearer{}, err
	}

	var dns netip.Addr
	if policy.DNS != nil {
		dns, _ = netip.AddrFromSlice(policy.DNS)
	}

	bearer = models.EPSBearer{
		Ref:        sc.Ref,
		PDNType:    eps.PDNType(pdnType),
		DNS:        dns.Unmap(),
		IPv4:       addrs.IPv4,
		IPv6Prefix: addrs.IPv6Prefix,
		IPv6IID:    addrs.IPv6IID,
	}

	// When the UE asked for IPv4v6 but the data network offers a single family,
	// the Activate Default EPS Bearer Context Request carries ESM cause #50/#51
	// (TS 24.301).
	switch narrowPDUType(req.RequestedPDNType, pdnType) {
	case narrowIPv4Only:
		bearer.ESMCause = eps.ESMCausePDNTypeIPv4OnlyAllowed
	case narrowIPv6Only:
		bearer.ESMCause = eps.ESMCausePDNTypeIPv6OnlyAllowed
	}

	sc.Mutex.Lock()
	ul := sc.Tunnel.DataPath.UpLinkTunnel
	bearer.SGW = models.FTEID{TEID: ul.TEID, Addr: ul.N3IPv4}
	bearer.SGWN3IPv6 = ul.N3IPv6
	sc.Mutex.Unlock()

	return bearer, nil
}

// transferToEPS moves a PDU session the UE holds in 5GS onto the default bearer
// it just asked for, keeping the UE address and the UPF session
// (TS 23.502 §4.11.2.2 step 13). The returned bearer describes the session as
// CreateEPSSession describes a new one, so the MME's Activate Default and the
// eNB's E-RAB setup follow unchanged.
func (s *SMF) transferToEPS(ctx context.Context, supi etsi.SUPI, req models.EPSBearerRequest, policy *Policy) (models.EPSBearer, error) {
	transfer := transferRequest{
		Access: Access4G,
		EBI:    req.EPSBearerIdentity,
		Dnn:    req.APN,
		Policy: policy,
	}

	sc, err := s.findTransferable(supi, req.PDUSessionID, transfer)
	if err != nil {
		// The MME rejects the PDN connectivity procedure with ESM cause #54, so the
		// UE knows to establish the connection afresh (TS 24.301 §6.5.1.4 b).
		return models.EPSBearer{ESMCause: eps.ESMCausePDNConnectionDoesNotExist}, err
	}

	// The default bearer identity the MME assigned may still name a stale PDN
	// connection, as it may on an initial request.
	if existing := s.currentEPSSession(supi, req.EPSBearerIdentity); existing != nil && existing != sc {
		s.handlePduSessionContextReplacement(ctx, existing)
	}

	if err := s.transferSession(ctx, sc, transfer); err != nil {
		return models.EPSBearer{}, err
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	ul := sc.Tunnel.DataPath.UpLinkTunnel

	bearer := models.EPSBearer{
		Ref:        sc.Ref,
		PDNType:    eps.PDNType(sc.PDUSessionType),
		IPv4:       ipToNetip(sc.PDUIPV4Address),
		IPv6Prefix: ipToNetip(sc.PDUIPV6Prefix),
		IPv6IID:    sc.IPv6IID,
		SGW:        models.FTEID{TEID: ul.TEID, Addr: ul.N3IPv4},
		SGWN3IPv6:  ul.N3IPv6,
	}

	if policy.DNS != nil {
		bearer.DNS = ipToNetip(policy.DNS)
	}

	// The transferred session keeps the type it was established with. A UE that
	// mapped its PDU session type to a wider PDN type is told which family it
	// actually has (TS 24.301 §6.5.1.3).
	switch narrowPDUType(req.RequestedPDNType, sc.PDUSessionType) {
	case narrowIPv4Only:
		bearer.ESMCause = eps.ESMCausePDNTypeIPv4OnlyAllowed
	case narrowIPv6Only:
		bearer.ESMCause = eps.ESMCausePDNTypeIPv6OnlyAllowed
	}

	return bearer, nil
}

// ModifyEPSSession sets the established session's downlink endpoint to the eNB
// S1-U F-TEID, so the UPF encapsulates downlink traffic toward the eNB
// (PSC-less GTP-U on S1-U).
func (s *SMF) ModifyEPSSession(ctx context.Context, ref string, enb models.FTEID) error {
	smContext := s.GetSession(ref)
	if smContext == nil {
		return fmt.Errorf("no EPS session %q", ref)
	}

	ctx, span := tracer.Start(ctx, "smf/modify_eps_session", epsSessionAttributes(smContext))
	defer span.End()

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if smContext.Tunnel == nil || !smContext.Tunnel.DataPath.Activated {
		return fmt.Errorf("EPS session %q is not activated", ref)
	}

	dl := smContext.Tunnel.DataPath.DownLinkTunnel.PDR
	ul := smContext.Tunnel.DataPath.UpLinkTunnel.PDR
	dl.FAR.ApplyAction = models.ApplyAction{Forw: true}

	// bindAccessTunnel aligns the uplink OuterHeaderRemoval, which defaults to IPv4
	// at session creation, to the eNB's address family.
	enbIP := net.IP(enb.Addr.AsSlice())

	an := AnchorBinding{TEID: enb.TEID}
	if enbIP.To4() == nil {
		an.IPv6 = enbIP
	} else {
		an.IPv4 = enbIP
	}

	smContext.bindAccessTunnel(an)

	dl.State = RuleUpdate
	dl.FAR.State = RuleUpdate
	ul.State = RuleUpdate

	var policyID string
	if smContext.PolicyData != nil {
		policyID = smContext.PolicyData.PolicyID
	}

	if err := s.upf.ModifySession(ctx, BuildModifyRequest(
		smContext.PFCPContext.RemoteSEID,
		policyID,
		[]*PDR{dl, ul},
		[]*FAR{dl.FAR},
		nil,
	)); err != nil {
		return err
	}

	// Register the IPv6 session so the UPF's RA responder answers the UE's Router
	// Solicitation with the /64 prefix. No-op for an IPv4-only bearer.
	s.registerIPv6SessionIfNeeded(ctx, smContext)

	return nil
}

// UpdateEPSSessionAMBR updates an established session's Session-AMBR in the UPF
// QER so the data plane enforces the new per-session rate limit. The AMBR is
// given in the "<n> <unit>" form used at session creation.
func (s *SMF) UpdateEPSSessionAMBR(ctx context.Context, ref string, ambrUplink, ambrDownlink models.BitRate) error {
	smContext := s.GetSession(ref)
	if smContext == nil {
		return fmt.Errorf("no EPS session %q", ref)
	}

	ctx, span := tracer.Start(ctx, "smf/update_eps_session_ambr", epsSessionAttributes(smContext))
	defer span.End()

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	var (
		policyID string
		qfi      uint8
	)

	if smContext.PolicyData != nil {
		policyID = smContext.PolicyData.PolicyID
		qfi = smContext.PolicyData.QosData.QFI
	}

	if err := s.applySessionQERs(ctx, smContext, policyID, qfi, ambrUplink, ambrDownlink); err != nil {
		return fmt.Errorf("update Session-AMBR for %q: %w", ref, err)
	}

	// Cache the new rate only after the data plane has accepted it.
	if smContext.PolicyData != nil {
		smContext.PolicyData.Ambr.Uplink = ambrUplink
		smContext.PolicyData.Ambr.Downlink = ambrDownlink
	}

	return nil
}

// ReleaseEPSSession tears down the 4G default bearer identified by its unique
// session ref, freeing the UPF session (PDRs/FARs/QER + TEID) and the UE IP lease.
// Releasing by ref targets the exact instance, so superseding an old context cannot
// tear down a newer session that reused the (IMSI, EBI) slot.
func (s *SMF) ReleaseEPSSession(ctx context.Context, ref string) error {
	return s.ReleaseSmContext(ctx, ref)
}

// FramedRoutesChanged reports whether the subscriber's provisioned framed routes
// for the EPS session differ from those installed at establishment. The MME
// reconciler reactivates the bearer on a change (TS 23.501 §5.6.14). An unknown
// session reports no change.
func (s *SMF) FramedRoutesChanged(ctx context.Context, ref string) (bool, error) {
	smContext := s.GetSession(ref)
	if smContext == nil {
		return false, nil
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	return s.framedRoutesChanged(ctx, smContext)
}

// StaticIPChanged reports whether the subscriber's reserved static IP for the
// EPS session changed since establishment; an unknown session reports no change.
func (s *SMF) StaticIPChanged(ctx context.Context, ref string) (bool, error) {
	smContext := s.GetSession(ref)
	if smContext == nil {
		return false, nil
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	return s.staticIPChanged(ctx, smContext)
}

// DeactivateEPSSession puts the retained 4G default bearer into buffering mode when
// the UE goes ECM-IDLE: the downlink FAR buffers packets, so downlink
// data raises a paging notification and never reaches the released eNB tunnel.
func (s *SMF) DeactivateEPSSession(ctx context.Context, ref string) error {
	return s.DeactivateSmContext(ctx, ref)
}

// epsSessionAttributes labels a span with the session's EPS identity, read
// without the session lock: SUPI and EBI are assigned at creation and immutable.
func epsSessionAttributes(smContext *SMContext) trace.SpanStartEventOption {
	return trace.WithAttributes(
		attribute.String("ue.imsi", smContext.Supi.IMSI()),
		attribute.Int("eps.bearer_id", int(smContext.EBI)),
	)
}
