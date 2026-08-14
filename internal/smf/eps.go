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
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

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

	if req.RequestType == eps.RequestTypeHandover {
		return s.transferToEPS(ctx, supi, req, policy)
	}

	// §5.5.1.2.7 f)
	s.supersedeIdentityHolders(ctx, supi, SessionIdentity{PDUSessionID: req.PDUSessionID, EBI: req.EPSBearerIdentity}, Access4G)

	requestedType, err := pduSessionTypeFor(req.RequestedPDNType)
	if err != nil {
		return models.EPSBearer{}, &models.PDNTypeError{Cause: allowedPDNTypeCause(policy)}
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

var errTransferRolledBack = errors.New("transfer rolled back")

func (s *SMF) ModifyEPSSession(ctx context.Context, ref string, ebi uint8, enb models.FTEID) error {
	ctx, span := tracer.Start(ctx, "smf/modify_eps_session",
		trace.WithAttributes(
			attribute.String("smf.session_ref", ref),
			attribute.Int("eps.bearer_id", int(ebi)),
		),
	)
	defer span.End()

	smContext := s.GetSession(ref)
	if smContext == nil {
		return fmt.Errorf("no EPS session %q", ref)
	}

	dropped, err := s.bindEPSDownlink(ctx, smContext, enb)
	if err != nil {
		span.RecordError(err)
	}

	return s.finishBinding(ctx, smContext, dropped, err)
}

func (s *SMF) bindEPSDownlink(ctx context.Context, smContext *SMContext, enb models.FTEID) (*droppedSource, error) {
	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if smContext.Tunnel == nil {
		return nil, fmt.Errorf("EPS session %q has no user plane", smContext.Ref)
	}

	enbIP := net.IP(enb.Addr.AsSlice())

	an := AnchorBinding{TEID: enb.TEID}
	if enbIP.To4() == nil {
		an.IPv6 = enbIP
	} else {
		an.IPv4 = enbIP
	}

	dropped, err := s.bindDownlink(ctx, smContext, Access4G, an)
	if err != nil {
		return nil, err
	}

	s.registerIPv6SessionIfNeeded(ctx, smContext, Access4G)

	return dropped, nil
}

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

	if smContext.PolicyData != nil {
		updated := *smContext.PolicyData
		updated.Ambr.Uplink = ambrUplink
		updated.Ambr.Downlink = ambrDownlink
		smContext.PolicyData = &updated
	}

	return nil
}

func (s *SMF) ReleaseEPSSession(ctx context.Context, ref string) error {
	if s.dropHalf(ref, Access4G) {
		return nil
	}

	return s.releaseSession(ctx, ref)
}

func (s *SMF) dropHalf(ref string, by AccessType) bool {
	sc := s.GetSession(ref)
	if sc == nil {
		return false
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if sc.Access == by {
		sc.releasing = true
		return false
	}

	if sc.pending != nil && sc.pending.to == by {
		sc.abandonPendingLocked()
	}

	return true
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

func (s *SMF) DeactivateEPSSession(ctx context.Context, ref string) error {
	return s.deactivateSession(ctx, ref, Access4G)
}
