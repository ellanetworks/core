// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"
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

// Out of range (TS 24.007 §11.2.3.1b) becomes 0, and a PDN connection without
// one is not transferable to 5GS (TS 23.502 §4.11.1.1 NOTE 5).
func ueAllocatedPDUSessionID(ctx context.Context, supi etsi.SUPI, pduSessionID uint8) uint8 {
	if pduSessionID == 0 {
		return 0
	}

	if pduSessionID > 15 {
		logger.WithTrace(ctx, logger.SmfLog).Warn("ignoring out-of-range PDU session id from PCO",
			logger.SUPI(supi.String()), logger.PDUSessionID(pduSessionID))

		return 0
	}

	return pduSessionID
}

func establishESMCause(err error) eps.ESMCause {
	if errors.Is(err, errSessionIdentityInUse) {
		return eps.ESMCauseInsufficientResources
	}

	return 0
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

	// TS 24.301 §6.5.1.2, TS 23.502 §4.11.2.2 step 13: a handover request type
	// transfers a PDU session the UE holds in 5GS.
	if req.RequestType == eps.RequestTypeHandover {
		return s.transferToEPS(ctx, supi, req, policy)
	}

	// Ella Core serves no emergency bearer services and no RLOS (§1), so the
	// anchor holds no emergency PDN connection to transfer (TS 24.301 §6.5.1.6 e).
	switch req.RequestType {
	case eps.RequestTypeHandoverOfEmergencyBearerServices:
		return models.EPSBearer{ESMCause: eps.ESMCausePDNConnectionDoesNotExist},
			fmt.Errorf("no emergency PDN connection to transfer")
	case eps.RequestTypeEmergency, eps.RequestTypeRLOS:
		return models.EPSBearer{ESMCause: eps.ESMCauseRequestRejectedUnspecified},
			fmt.Errorf("request type %s is not served", req.RequestType)
	}

	// TS 24.301 §5.5.1.2.4 case f: the request supersedes the stale context. It
	// precedes establishSession, whose new session would already hold the address
	// the superseded context's release frees.
	if existing := s.currentEPSSession(supi, req.EPSBearerIdentity); existing != nil {
		s.handlePduSessionContextReplacement(ctx, existing)
	}

	pdnType, err := s.negotiatePDUSessionType(ctx, req.RequestedPDNType, policy)
	if err != nil {
		return models.EPSBearer{ESMCause: pdnTypeRejectCause(req.RequestedPDNType, policy)},
			fmt.Errorf("negotiate PDN type: %w", err)
	}

	sc, addrs, err := s.establishSession(ctx, SessionRequest{
		Supi:     supi,
		Identity: SessionIdentity{PDUSessionID: ueAllocatedPDUSessionID(ctx, supi, req.PDUSessionID), EBI: req.EPSBearerIdentity},
		Dnn:      req.APN,
		Snssai:   req.Snssai,
		Access:   Access4G,
		PDUType:  pdnType,
		Policy:   policy,
	})
	if err != nil {
		return models.EPSBearer{ESMCause: establishESMCause(err)}, err
	}

	var dns netip.Addr
	if policy.DNS != nil {
		dns, _ = netip.AddrFromSlice(policy.DNS)
	}

	pdnTypeIE, err := pdnTypeFor(pdnType)
	if err != nil {
		return models.EPSBearer{ESMCause: eps.ESMCauseUnknownPDNType}, err
	}

	bearer = models.EPSBearer{
		Ref:        sc.Ref,
		PDNType:    pdnTypeIE,
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

// The UE address and the UPF session survive the move (TS 23.502 §4.11.2.2
// step 13).
func (s *SMF) transferToEPS(ctx context.Context, supi etsi.SUPI, req models.EPSBearerRequest, policy *Policy) (models.EPSBearer, error) {
	transfer := transferRequest{
		Access: Access4G,
		EBI:    req.EPSBearerIdentity,
		Dnn:    req.APN,
		Snssai: req.Snssai,
		Policy: policy,
	}

	sc, err := s.findTransferable(supi, req.PDUSessionID, transfer)
	if err != nil {
		return models.EPSBearer{ESMCause: eps.ESMCausePDNConnectionDoesNotExist}, err
	}

	// The default bearer identity the MME assigned may still name a stale PDN
	// connection.
	if existing := s.currentEPSSession(supi, req.EPSBearerIdentity); existing != nil && existing != sc {
		s.handlePduSessionContextReplacement(ctx, existing)
	}

	if err := s.transferSession(ctx, sc, transfer); err != nil {
		return models.EPSBearer{ESMCause: eps.ESMCauseInsufficientResources}, err
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	pdnTypeIE, err := pdnTypeFor(sc.PDUSessionType)
	if err != nil {
		return models.EPSBearer{ESMCause: eps.ESMCauseUnknownPDNType}, err
	}

	ul := sc.Tunnel.DataPath.UpLinkTunnel

	bearer := models.EPSBearer{
		Ref:        sc.Ref,
		PDNType:    pdnTypeIE,
		IPv4:       ipToNetip(sc.PDUIPV4Address),
		IPv6Prefix: ipToNetip(sc.PDUIPV6Prefix),
		IPv6IID:    sc.IPv6IID,
		SGW:        models.FTEID{TEID: ul.TEID, Addr: ul.N3IPv4},
		SGWN3IPv6:  ul.N3IPv6,
	}

	if policy.DNS != nil {
		bearer.DNS = ipToNetip(policy.DNS)
	}

	// TS 24.301 §6.5.1.3: the UE learns which family the transferred session has.
	switch narrowPDUType(req.RequestedPDNType, sc.PDUSessionType) {
	case narrowIPv4Only:
		bearer.ESMCause = eps.ESMCausePDNTypeIPv4OnlyAllowed
	case narrowIPv6Only:
		bearer.ESMCause = eps.ESMCausePDNTypeIPv6OnlyAllowed
	}

	return bearer, nil
}

// S1-U carries PSC-less GTP-U.
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
// for the EPS session differ from those installed at establishment (TS 23.501
// §5.6.14). An unknown session reports no change.
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
// EPS session changed since establishment. An unknown session reports no change.
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

// A transfer reassigns the bearer identity under the session lock.
func epsSessionAttributes(smContext *SMContext) trace.SpanStartEventOption {
	smContext.Mutex.Lock()
	ebi := smContext.EBI
	smContext.Mutex.Unlock()

	return trace.WithAttributes(
		attribute.String("ue.imsi", smContext.Supi.IMSI()),
		attribute.Int("eps.bearer_id", int(ebi)),
	)
}
