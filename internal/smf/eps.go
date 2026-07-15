// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// The SMF (combined PGW-C, TS 23.501) keys each 4G PDN connection by its
// default bearer's EPS bearer identity (5..15) as the PDU session id. A
// subscriber is never on 4G and 5G at once, so the EBI cannot collide with a
// live 5G PDU session id.

// validateEPSBearerRequest rejects inputs the data path would otherwise accept
// and degrade: a zero AMBR programs a zero-rate QER, and a non-IP DNS drops the
// DNS option. An EBI outside 5..15 is not a valid default bearer (TS 24.007).
func validateEPSBearerRequest(req models.EPSBearerRequest) error {
	if req.EPSBearerIdentity < 5 || req.EPSBearerIdentity > 15 {
		return fmt.Errorf("EPS bearer identity %d out of range (5..15)", req.EPSBearerIdentity)
	}

	if bitRateTokbps(req.AMBRUplink) == 0 {
		return fmt.Errorf("invalid uplink AMBR %q", req.AMBRUplink)
	}

	if bitRateTokbps(req.AMBRDownlink) == 0 {
		return fmt.Errorf("invalid downlink AMBR %q", req.AMBRDownlink)
	}

	if req.DNS != "" && net.ParseIP(req.DNS) == nil {
		return fmt.Errorf("invalid DNS address %q", req.DNS)
	}

	return nil
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

	if err = validateEPSBearerRequest(req); err != nil {
		return models.EPSBearer{}, err
	}

	policy := &Policy{
		PolicyID: req.PolicyID,
		Ambr:     models.Ambr{Uplink: req.AMBRUplink, Downlink: req.AMBRDownlink},
		IPv4Pool: req.IPv4Pool,
		IPv6Pool: req.IPv6Pool,
		DNS:      net.ParseIP(req.DNS),
		MTU:      req.MTU,
	}

	pdnType, err := s.negotiatePDUSessionType(ctx, req.RequestedPDNType, policy)
	if err != nil {
		return models.EPSBearer{}, fmt.Errorf("negotiate PDN type: %w", err)
	}

	// Must precede establishSession: the superseded context's release frees the address by
	// (imsi, dnn, ebi), which the new session would already hold (TS 24.301 §5.5.1.2.4 case f).
	if existing := s.currentSession(supi, req.EPSBearerIdentity); existing != nil {
		s.handlePduSessionContextReplacement(ctx, existing)
	}

	sc, addrs, err := s.establishSession(ctx, SessionRequest{
		Supi:    supi,
		Key:     req.EPSBearerIdentity,
		Dnn:     req.APN,
		Access:  Access4G,
		PDUType: pdnType,
		Policy:  policy,
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
		PDNType:    pdnType,
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

// ModifyEPSSession sets the established session's downlink endpoint to the eNB
// S1-U F-TEID, so the UPF encapsulates downlink traffic toward the eNB
// (PSC-less GTP-U on S1-U).
func (s *SMF) ModifyEPSSession(ctx context.Context, imsi string, ebi uint8, enb models.FTEID) error {
	ctx, span := tracer.Start(ctx, "smf/modify_eps_session",
		trace.WithAttributes(
			attribute.String("ue.imsi", imsi),
			attribute.Int("eps.bearer_id", int(ebi)),
		),
	)
	defer span.End()

	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		return fmt.Errorf("invalid imsi %q: %w", imsi, err)
	}

	smContext := s.currentSession(supi, ebi)
	if smContext == nil {
		return fmt.Errorf("no EPS session for %s", imsi)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if smContext.Tunnel == nil || !smContext.Tunnel.DataPath.Activated {
		return fmt.Errorf("EPS session for %s is not activated", imsi)
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
func (s *SMF) UpdateEPSSessionAMBR(ctx context.Context, imsi string, ebi uint8, ambrUplink, ambrDownlink string) error {
	ctx, span := tracer.Start(ctx, "smf/update_eps_session_ambr",
		trace.WithAttributes(
			attribute.String("ue.imsi", imsi),
			attribute.Int("eps.bearer_id", int(ebi)),
		),
	)
	defer span.End()

	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		return fmt.Errorf("invalid imsi %q: %w", imsi, err)
	}

	smContext := s.currentSession(supi, ebi)
	if smContext == nil {
		return fmt.Errorf("no EPS session for %s", imsi)
	}

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
		return fmt.Errorf("update Session-AMBR for %s: %w", imsi, err)
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
// for the EPS session (imsi, ebi) differ from those installed at establishment.
// The MME reconciler reactivates the bearer on a change (TS 23.501 §5.6.14). An
// unknown session reports no change.
func (s *SMF) FramedRoutesChanged(ctx context.Context, imsi string, ebi uint8) (bool, error) {
	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		return false, fmt.Errorf("invalid imsi %q: %w", imsi, err)
	}

	smContext := s.currentSession(supi, ebi)
	if smContext == nil {
		return false, nil
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	return s.framedRoutesChanged(ctx, smContext)
}

// StaticIPChanged reports whether the subscriber's reserved static IP for the
// EPS session (imsi, ebi) changed since establishment; an unknown session
// reports no change.
func (s *SMF) StaticIPChanged(ctx context.Context, imsi string, ebi uint8) (bool, error) {
	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		return false, fmt.Errorf("invalid imsi %q: %w", imsi, err)
	}

	smContext := s.currentSession(supi, ebi)
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
func (s *SMF) DeactivateEPSSession(ctx context.Context, imsi string, ebi uint8) error {
	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		return fmt.Errorf("invalid imsi %q: %w", imsi, err)
	}

	smContext := s.currentSession(supi, ebi)
	if smContext == nil {
		return fmt.Errorf("no EPS session for %s", imsi)
	}

	return s.DeactivateSmContext(ctx, smContext.Ref)
}
