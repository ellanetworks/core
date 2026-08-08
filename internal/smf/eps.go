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

	// Precedes the supersede below, which would tear down the session being moved.
	if req.RequestType == eps.RequestTypeHandover {
		return s.transferToEPS(ctx, supi, req, policy)
	}

	// The local release is unconditional (TS 24.301 §5.5.1.2.7 f), so it precedes
	// the type negotiation, as the 5GS path does — a failed negotiation must not
	// leave the superseded session alive.
	if existing := s.currentEPSSession(supi, req.EPSBearerIdentity); existing != nil {
		s.handlePduSessionContextReplacement(ctx, existing, Access4G)
	}

	// Converted at the boundary so nothing downstream mixes the two enumerations,
	// which agree only up to IPv4v6.
	requestedType, err := pduSessionTypeFor(req.RequestedPDNType)
	if err != nil {
		return models.EPSBearer{}, &models.PDNTypeError{Cause: eps.ESMCauseUnknownPDNType}
	}

	pduType, err := s.negotiatePDUSessionType(ctx, requestedType, policy)
	if err != nil {
		return models.EPSBearer{}, &models.PDNTypeError{Cause: pdnTypeRejectCause(requestedType, policy)}
	}

	pdnType, err := pdnTypeFor(pduType)
	if err != nil {
		return models.EPSBearer{}, &models.PDNTypeError{Cause: eps.ESMCauseUnknownPDNType}
	}

	sc, addrs, err := s.establishSession(ctx, SessionRequest{
		Supi:     supi,
		Identity: SessionIdentity{PDUSessionID: req.PDUSessionID, EBI: req.EPSBearerIdentity},
		Dnn:      req.APN,
		Snssai:   req.Snssai,
		Access:   Access4G,
		PDUType:  pduType,
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
		Ref:          sc.Ref,
		PDNType:      pdnType,
		DNS:          dns.Unmap(),
		IPv4:         addrs.IPv4,
		IPv6Prefix:   addrs.IPv6Prefix,
		IPv6IID:      addrs.IPv6IID,
		PDUSessionID: sc.PDUSessionID,
		Snssai:       sc.Snssai,
	}

	// When the UE asked for IPv4v6 but the data network offers a single family,
	// the Activate Default EPS Bearer Context Request carries ESM cause #50/#51
	// (TS 24.301).
	switch narrowPDUType(requestedType, pduType) {
	case narrowIPv4Only:
		bearer.ESMCause = eps.ESMCausePDNTypeIPv4OnlyAllowed
	case narrowIPv6Only:
		bearer.ESMCause = eps.ESMCausePDNTypeIPv6OnlyAllowed
	}

	sc.Mutex.Lock()
	bearer.SGW = models.FTEID{TEID: sc.Tunnel.N3TEID, Addr: sc.Tunnel.N3IPv4}
	bearer.SGWN3IPv6 = sc.Tunnel.N3IPv6
	sc.Mutex.Unlock()

	return bearer, nil
}

// ModifyEPSSession sets the established session's downlink endpoint to the eNB
// S1-U F-TEID, so the UPF encapsulates downlink traffic toward the eNB
// (PSC-less GTP-U on S1-U).
func (s *SMF) ModifyEPSSession(ctx context.Context, ref string, enb models.FTEID) error {
	ctx, span := tracer.Start(ctx, "smf/modify_eps_session",
		trace.WithAttributes(attribute.String("smf.session_ref", ref)),
	)
	defer span.End()

	smContext := s.GetSession(ref)
	if smContext == nil {
		return fmt.Errorf("no EPS session %q", ref)
	}

	dropped, err := s.bindEPSDownlink(ctx, smContext, enb)
	if err != nil {
		return err
	}

	s.dropSourceRouting(ctx, smContext.Ref, dropped)

	return nil
}

func (s *SMF) bindEPSDownlink(ctx context.Context, smContext *SMContext, enb models.FTEID) (*droppedSource, error) {
	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	commit, err := s.beginTransferCommit(ctx, smContext, Access4G)
	if err != nil {
		return nil, fmt.Errorf("failed to move the session to EPS: %w", err)
	}

	if smContext.Access != Access4G {
		return nil, fmt.Errorf("session %q is on %s, not EPS", smContext.Ref, smContext.Access)
	}

	if smContext.Tunnel == nil || !smContext.Tunnel.Activated {
		if commit != nil {
			commit.restore()
		}

		return nil, fmt.Errorf("EPS session %q is not activated", smContext.Ref)
	}

	dl := smContext.Tunnel.DownlinkPDR
	ul := smContext.Tunnel.UplinkPDR
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

	smContext.bindAccessTunnel(an, Access4G)

	var (
		policyID string
		qers     []*QER
	)

	if smContext.PolicyData != nil {
		policyID = smContext.PolicyData.PolicyID
	}

	if commit != nil {
		policyID = commit.policy.PolicyID
		qers = commit.qers
	}

	if err := s.upf.ModifySession(ctx, BuildModifyRequest(
		smContext.PFCPContext.SEID,
		policyID,
		[]*PDR{dl, ul},
		[]*FAR{dl.FAR},
		qers,
	)); err != nil {
		if commit != nil {
			commit.restore()
		}

		return nil, err
	}

	// Register the IPv6 session so the UPF's RA responder answers the UE's Router
	// Solicitation with the /64 prefix. No-op for an IPv4-only bearer.
	s.registerIPv6SessionIfNeeded(ctx, smContext)

	if commit == nil {
		return nil, nil
	}

	return smContext.finishTransferCommit(commit), nil
}

// UpdateEPSSessionAMBR updates an established session's Session-AMBR in the UPF
// QER so the data plane enforces the new per-session rate limit. The AMBR is
// given in the "<n> <unit>" form used at session creation.
func (s *SMF) UpdateEPSSessionAMBR(ctx context.Context, ref string, ambrUplink, ambrDownlink models.BitRate) error {
	ctx, span := tracer.Start(ctx, "smf/update_eps_session_ambr",
		trace.WithAttributes(attribute.String("smf.session_ref", ref)),
	)
	defer span.End()

	smContext := s.GetSession(ref)
	if smContext == nil {
		return fmt.Errorf("no EPS session %q", ref)
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

func (s *SMF) FramedRoutesChanged(ctx context.Context, ref string) (bool, error) {
	return s.epsSubscriptionChanged(ctx, ref, s.framedRoutesChanged)
}

func (s *SMF) StaticIPChanged(ctx context.Context, ref string) (bool, error) {
	return s.epsSubscriptionChanged(ctx, ref, s.staticIPChanged)
}

func (s *SMF) epsSubscriptionChanged(ctx context.Context, ref string, changed func(context.Context, *SMContext) (bool, error)) (bool, error) {
	smContext := s.GetSession(ref)
	if smContext == nil {
		return false, nil
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	return changed(ctx, smContext)
}

// DeactivateEPSSession puts the retained 4G default bearer into buffering mode when
// the UE goes ECM-IDLE: the downlink FAR buffers packets, so downlink
// data raises a paging notification and never reaches the released eNB tunnel.
func (s *SMF) DeactivateEPSSession(ctx context.Context, ref string) error {
	return s.DeactivateSmContext(ctx, ref)
}
