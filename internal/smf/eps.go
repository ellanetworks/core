// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tracing/attrs"
	"github.com/ellanetworks/core/nas/eps"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func validateEPSBearerRequest(req models.EPSBearerRequest) error {
	if req.EPSBearerIdentity < 5 || req.EPSBearerIdentity > 15 {
		return fmt.Errorf("EPS bearer identity %d out of range (5..15)", req.EPSBearerIdentity)
	}

	return nil
}

func (s *SMF) resolveEPSPolicy(ctx context.Context, supi etsi.SUPI, apn string, snssai *models.Snssai) (*Policy, *models.Snssai, error) {
	if snssai != nil {
		policy, err := s.GetSessionPolicy(ctx, supi, snssai, apn)

		return policy, snssai, err
	}

	return s.pcf.GetEPSSessionPolicy(ctx, supi.IMSI(), apn)
}

func (s *SMF) CreateEPSSession(ctx context.Context, req models.EPSBearerRequest) (bearer models.EPSBearer, err error) {
	ctx, span := tracer.Start(ctx, "smf/create_eps_session",
		trace.WithAttributes(
			attrs.SUPIFromIMSI(req.IMSI),
			attribute.Int("eps.bearer_id", int(req.EPSBearerIdentity)),
			attribute.String("eps.apn", req.APN),
		),
	)
	defer span.End()

	supi, err := etsi.NewSUPIFromIMSI(req.IMSI)

	defer func() {
		recordSessionEstablishment(ctx, metrics.RAT4G, err, logger.DNN(req.APN))
	}()

	if err != nil {
		return models.EPSBearer{}, fmt.Errorf("invalid imsi %q: %w", req.IMSI, err)
	}

	if err := validateEPSBearerRequest(req); err != nil {
		return models.EPSBearer{}, err
	}

	policy, snssai, err := s.resolveEPSPolicy(ctx, supi, req.APN, req.Snssai)
	if err != nil {
		return models.EPSBearer{}, fmt.Errorf("no policy for APN %q: %w", req.APN, err)
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

	if _, err := pdnTypeFor(pduType); err != nil {
		return models.EPSBearer{}, &models.PDNTypeError{Cause: eps.ESMCauseUnknownPDNType}
	}

	sc, err := s.establishSession(ctx, SessionRequest{
		Supi:     supi,
		Identity: SessionIdentity{PDUSessionID: req.PDUSessionID, EBI: req.EPSBearerIdentity},
		Dnn:      req.APN,
		Snssai:   snssai,
		Access:   Access4G,
		PDUType:  pduType,
		Policy:   policy,
	})
	if err != nil {
		return models.EPSBearer{}, err
	}

	bearer, err = epsBearerForSession(sc, policy)
	if err != nil {
		return models.EPSBearer{}, err
	}

	switch narrowPDUType(requestedType, pduType) {
	case narrowIPv4Only:
		bearer.ESMCause = eps.ESMCausePDNTypeIPv4OnlyAllowed
	case narrowIPv6Only:
		bearer.ESMCause = eps.ESMCausePDNTypeIPv6OnlyAllowed
	}

	return bearer, nil
}

var errTransferRolledBack = errors.New("transfer rolled back")

func (s *SMF) ModifyEPSSession(ctx context.Context, ref string, ebi uint8, enb models.FTEID) error {
	ctx, span := tracer.Start(ctx, "smf/modify_eps_session",
		trace.WithAttributes(
			attrs.SMContextRef(ref),
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

func (s *SMF) DeactivateEPSSession(ctx context.Context, ref string) error {
	return s.deactivateSession(ctx, ref, Access4G)
}

func (s *SMF) OpenEPSForwardingTunnel(ctx context.Context, ref string, target models.FTEID) (models.ForwardingTunnel, error) {
	ctx, span := tracer.Start(ctx, "smf/open_eps_forwarding_tunnel",
		trace.WithAttributes(attrs.SMContextRef(ref)),
	)
	defer span.End()

	smContext := s.GetSession(ref)
	if smContext == nil {
		return models.ForwardingTunnel{}, fmt.Errorf("no EPS session %q", ref)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if smContext.Tunnel == nil {
		return models.ForwardingTunnel{}, fmt.Errorf("EPS session %q has no user plane", ref)
	}

	targetIP := net.IP(target.Addr.AsSlice())

	an := AnchorBinding{TEID: target.TEID}
	if targetIP.To4() == nil {
		an.IPv6 = targetIP
	} else {
		an.IPv4 = targetIP
	}

	if err := s.openForwardingTunnel(ctx, smContext, an); err != nil {
		span.RecordError(err)

		return models.ForwardingTunnel{}, err
	}

	local := models.ForwardingTunnel{
		TEID: smContext.Tunnel.ForwardingTEID,
		IPv4: smContext.Tunnel.N3IPv4,
		IPv6: smContext.Tunnel.N3IPv6,
	}

	logger.From(ctx, logger.SmfLog).Info("Opened an indirect data forwarding tunnel",
		logger.SUPI(smContext.Supi.String()), logger.TEID(local.TEID))

	return local, nil
}

func (s *SMF) CloseEPSForwardingTunnel(ctx context.Context, ref string) error {
	smContext := s.GetSession(ref)
	if smContext == nil {
		return nil
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	smContext.forwardingRelease.Stop()

	return s.closeForwardingTunnel(ctx, smContext)
}
