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

// Out of range (TS 24.007 §11.2.3.1b) becomes 0. Without one the PDN connection
// cannot be correlated with a PDU session, so it is not transferable to 5GS.
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

	countEstablishment := true

	defer func() {
		if countEstablishment {
			recordSessionEstablishment(metrics.RAT4G, err)
		}
	}()

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
	// transfers a PDU session the UE holds in 5GS. It moves one session rather
	// than establishing another, so it is not counted as an establishment.
	if req.RequestType == eps.RequestTypeHandover {
		countEstablishment = false

		return s.transferToEPS(ctx, supi, req, policy)
	}

	// Ella Core serves no emergency bearer services, so there is no emergency PDN
	// connection to hand over, which TS 24.301 §6.5.1.6 e) refuses with ESM #54.
	// Request type "emergency" draws ESM #31 by local choice: the spec assumes a
	// network offering emergency service and gives no refusal.
	switch req.RequestType {
	case eps.RequestTypeHandoverOfEmergencyBearerServices:
		return models.EPSBearer{ESMCause: eps.ESMCausePDNConnectionDoesNotExist},
			fmt.Errorf("no emergency PDN connection to transfer")
	case eps.RequestTypeEmergency:
		return models.EPSBearer{ESMCause: eps.ESMCauseRequestRejectedUnspecified},
			fmt.Errorf("request type %s is not served", req.RequestType)
	}

	// TS 24.008 table 10.5.173 NOTE 3: "RLOS" is treated as "initial request" in
	// S1 mode by a network not supporting access to RLOS.

	// TS 24.301 §5.5.1.2.7 f): an ATTACH REQUEST from a UE already attached deletes
	// the EMM context and its EPS bearer contexts. The supersede precedes
	// establishSession, whose new session claims the address the superseded
	// context's release frees.
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

	pdnTypeIE, err := pdnTypeFor(pdnType)
	if err != nil {
		return models.EPSBearer{ESMCause: eps.ESMCauseUnknownPDNType}, err
	}

	bearer = models.EPSBearer{
		Ref:          sc.Ref,
		PDUSessionID: sc.PDUSessionID,
		PDNType:      pdnTypeIE,
		DNS:          ipToNetip(policy.DNS),
		IPv4:         addrs.IPv4,
		IPv6Prefix:   addrs.IPv6Prefix,
		IPv6IID:      addrs.IPv6IID,
	}

	bearer.ESMCause = narrowESMCause(req.RequestedPDNType, pdnType)

	sc.mu.Lock()
	ul := sc.Tunnel.DataPath.UpLinkTunnel
	bearer.SGW = models.FTEID{TEID: ul.TEID, Addr: ul.N3IPv4}
	bearer.SGWN3IPv6 = ul.N3IPv6
	sc.mu.Unlock()

	return bearer, nil
}

// The UE address, the UPF session and its uplink F-TEID survive the move
// (TS 23.502 §4.11.2.2 step 13), so the bearer the MME needs is readable from
// the session as it stands on the access serving it. The downlink switches when
// the eNB binds (TS 23.401 §5.10.2 step 13a).
func (s *SMF) transferToEPS(ctx context.Context, supi etsi.SUPI, req models.EPSBearerRequest, policy *Policy) (models.EPSBearer, error) {
	transfer := transferRequest{
		Access: Access4G,
		EBI:    req.EPSBearerIdentity,
		Dnn:    req.APN,
		Policy: policy,
	}

	sc, err := s.findTransferable(supi, req.PDUSessionID, transfer)
	if err != nil {
		// TS 24.301 §6.5.1.6 b): a request type "handover" naming a PDN connection the
		// network has no information about draws #54, whatever the reason it cannot be
		// transferred as described. The UE ignores the back-off timer for #54 and
		// retries with request type "initial request" (§6.5.1.4.3); #27 and #31 both
		// hold the APN down for 12 minutes.
		return models.EPSBearer{ESMCause: eps.ESMCausePDNConnectionDoesNotExist}, err
	}

	// The default bearer identity the MME assigned may still name a stale PDN
	// connection, whose release frees the identity this move validates.
	if existing := s.currentEPSSession(supi, req.EPSBearerIdentity); existing != nil && existing != sc {
		s.handlePduSessionContextReplacement(ctx, existing)
	}

	if err := s.prepareTransfer(sc, transfer); err != nil {
		return models.EPSBearer{ESMCause: eps.ESMCauseInsufficientResources}, err
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	pdnTypeIE, err := pdnTypeFor(sc.PDUSessionType)
	if err != nil {
		sc.pending = nil

		return models.EPSBearer{ESMCause: eps.ESMCauseUnknownPDNType}, err
	}

	// A teardown whose user-plane release failed leaves the tunnel in place, so
	// pool membership is what says the session is still there to report.
	if s.GetSession(sc.Ref) != sc {
		return models.EPSBearer{ESMCause: eps.ESMCauseInsufficientResources},
			fmt.Errorf("session %q left the pool during the move", sc.Ref)
	}

	if sc.Tunnel == nil || sc.Tunnel.DataPath == nil || sc.Tunnel.DataPath.UpLinkTunnel == nil {
		sc.pending = nil

		return models.EPSBearer{ESMCause: eps.ESMCauseInsufficientResources},
			fmt.Errorf("session %q has no uplink tunnel", sc.Ref)
	}

	ul := sc.Tunnel.DataPath.UpLinkTunnel

	bearer := models.EPSBearer{
		Ref:          sc.Ref,
		PDUSessionID: sc.PDUSessionID,
		Snssai:       sc.Snssai,
		PDNType:      pdnTypeIE,
		IPv4:         ipToNetip(sc.PDUIPV4Address),
		IPv6Prefix:   ipToNetip(sc.PDUIPV6Prefix),
		IPv6IID:      sc.IPv6IID,
		SGW:          models.FTEID{TEID: ul.TEID, Addr: ul.N3IPv4},
		SGWN3IPv6:    ul.N3IPv6,
	}

	if policy.DNS != nil {
		bearer.DNS = ipToNetip(policy.DNS)
	}

	bearer.ESMCause = narrowESMCause(req.RequestedPDNType, sc.PDUSessionType)

	return bearer, nil
}

// ModifyEPSSession binds the session's downlink to the eNB's S1-U endpoint, which
// also completes a transfer onto EPS.
func (s *SMF) ModifyEPSSession(ctx context.Context, ref string, enb models.FTEID) error {
	dropped, err := s.bindEPSDownlink(ctx, ref, enb)
	if err != nil {
		return err
	}

	// The eNB downlink is bound, so the access the session came from can stop
	// routing it (TS 23.502 §4.11.2.2 step 14).
	if dropped != nil {
		s.dropSourceRouting(ctx, dropped.supi, ref, dropped.access, dropped.id)
	}

	return nil
}

// bindEPSDownlink points the downlink at the eNB, which also commits a move onto
// EPS: TS 23.401 §5.10.2 step 13a is what prompts the PDN GW to start routing to
// the Serving GW. It returns the access the move left, nil when none committed.
//
// An S1AP response for a session on 5GS with no move toward EPS would point the
// downlink back at the eNB.
func (s *SMF) bindEPSDownlink(ctx context.Context, ref string, enb models.FTEID) (*droppedSource, error) {
	smContext, unlock, err := s.sessionBinding(ref, Access4G)
	if err != nil {
		return nil, err
	}

	defer unlock()

	ctx, span := tracer.Start(ctx, "smf/modify_eps_session", epsSessionAttributes(smContext))
	defer span.End()

	if smContext.Tunnel == nil || !smContext.Tunnel.DataPath.Activated {
		return nil, fmt.Errorf("EPS session %q is not activated", smContext.Ref)
	}

	commit, err := s.beginTransferCommit(ctx, smContext, Access4G)
	if err != nil {
		return nil, err
	}

	dl := smContext.Tunnel.DataPath.DownLinkTunnel.PDR
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

	// The move's QoS is already staged on the session's rules, so it travels in
	// the bind's own statement and costs no round trip of its own.
	policy := smContext.PolicyData
	if commit != nil {
		policy = commit.policy
	}

	if err := s.applySession(ctx, smContext, policy); err != nil {
		if commit != nil {
			commit.restore()
		}

		return nil, err
	}

	var dropped *droppedSource
	if commit != nil {
		dropped = smContext.finishTransferCommit(commit)
	}

	// Register the IPv6 session so the UPF's RA responder answers the UE's Router
	// Solicitation with the /64 prefix. No-op for an IPv4-only bearer.
	s.registerIPv6SessionIfNeeded(ctx, smContext)

	return dropped, nil
}

// UpdateEPSSessionAMBR updates an established session's Session-AMBR in the UPF
// QER so the data plane enforces the new per-session rate limit. The AMBR is
// given in the "<n> <unit>" form used at session creation.
func (s *SMF) UpdateEPSSessionAMBR(ctx context.Context, ref string, ambrUplink, ambrDownlink models.BitRate) error {
	// On 5GS the APN-AMBR this carries is not the session's rate limit.
	smContext, unlock, err := s.sessionFor(ref, Access4G)
	if err != nil {
		return err
	}

	defer unlock()

	ctx, span := tracer.Start(ctx, "smf/update_eps_session_ambr", epsSessionAttributes(smContext))
	defer span.End()

	var qfi uint8
	if smContext.PolicyData != nil {
		qfi = smContext.PolicyData.QosData.QFI
	}

	if err := s.applySessionQERs(ctx, smContext, smContext.PolicyData, qfi, ambrUplink, ambrDownlink); err != nil {
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
	return s.releaseSmContext(ctx, ref, Access4G)
}

// ServesEPS reports whether EPS holds the session: the access serving it, or the
// target of a move whose eNB bind is still to come. An unknown session is
// neither, so a caller holding a stale reference does not signal for it.
func (s *SMF) ServesEPS(_ context.Context, ref string) bool {
	_, unlock, err := s.sessionBinding(ref, Access4G)
	if err != nil {
		return false
	}

	unlock()

	return true
}

// FramedRoutesChanged reports whether the subscriber's provisioned framed routes
// for the EPS session differ from those installed at establishment (TS 23.501
// §5.6.14). An unknown session reports no change.
func (s *SMF) FramedRoutesChanged(ctx context.Context, ref string) (bool, error) {
	return s.epsSubscriptionChanged(ctx, ref, s.framedRoutesChanged)
}

// StaticIPChanged reports whether the subscriber's reserved static IP for the
// EPS session changed since establishment. An unknown session reports no change.
func (s *SMF) StaticIPChanged(ctx context.Context, ref string) (bool, error) {
	return s.epsSubscriptionChanged(ctx, ref, s.staticIPChanged)
}

// A session unknown to the pool or moved off EPS has nothing the MME can act on,
// so neither is reported as a change.
func (s *SMF) epsSubscriptionChanged(ctx context.Context, ref string, changed func(context.Context, *SMContext) (bool, error)) (bool, error) {
	smContext, unlock, err := s.sessionFor(ref, Access4G)
	if err != nil {
		return false, nil
	}

	defer unlock()

	return changed(ctx, smContext)
}

// DeactivateEPSSession puts the retained 4G default bearer into buffering mode when
// the UE goes ECM-IDLE: the downlink FAR buffers packets, so downlink
// data raises a paging notification and never reaches the released eNB tunnel.
func (s *SMF) DeactivateEPSSession(ctx context.Context, ref string) error {
	return s.deactivateSmContext(ctx, ref, Access4G)
}

// A move reassigns the bearer identity, so the caller holds sc.mu.
func epsSessionAttributes(smContext *SMContext) trace.SpanStartEventOption {
	return trace.WithAttributes(
		attribute.String("ue.imsi", smContext.Supi.IMSI()),
		attribute.Int("eps.bearer_id", int(smContext.EBI)),
	)
}

// Unmapping the IPv4-in-IPv6 form keeps the address in the family it was
// allocated in.
func ipToNetip(ip net.IP) netip.Addr {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return netip.Addr{}
	}

	return addr.Unmap()
}
