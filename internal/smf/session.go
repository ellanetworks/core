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
	"github.com/ellanetworks/core/internal/models"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// establishSession failure classes, so an adapter can map a failure to the NAS
// cause its access uses.
var (
	errUEAddressAllocation  = errors.New("UE address allocation failed")
	errFramedRouteResolve   = errors.New("framed route resolution failed")
	errStaticIPResolve      = errors.New("static IP resolution failed")
	errDataPathActivation   = errors.New("data path activation failed")
	errUPFSession           = errors.New("UPF session establishment failed")
	errSessionIdentityInUse = errors.New("session identity is already in use")
)

// SessionRequest is the RAT-agnostic input to establishSession, common to the
// 5G and 4G paths.
type SessionRequest struct {
	Supi     etsi.SUPI
	Identity SessionIdentity
	Dnn      string
	Snssai   *models.Snssai
	Access   AccessType
	PDUType  uint8 // the negotiated PDU/PDN type
	Policy   *Policy
}

// ueAddresses is the address set allocated for a session; the IPv6 prefix is the
// /64 base. An invalid Addr means that family was not allocated.
type ueAddresses struct {
	IPv4       netip.Addr
	IPv6Prefix netip.Addr
	IPv6IID    [8]byte
}

// establishSession is the shared create core of the 5G and 4G paths (the SMF as
// combined SMF+PGW-C, TS 23.501 §4.3): it allocates the UE address(es), programs
// the data path, and establishes the UPF session. On failure it rolls the
// partial session back and wraps a sentinel error for the adapter to map to its
// NAS cause.
func (s *SMF) establishSession(ctx context.Context, req SessionRequest) (*SMContext, ueAddresses, error) {
	if !req.Identity.valid() {
		return nil, ueAddresses{}, fmt.Errorf("session identity %s names no session", req.Identity)
	}

	dn, err := s.store.ResolveDNN(ctx, req.Dnn)
	if err != nil {
		return nil, ueAddresses{}, fmt.Errorf("%w: %v", errUEAddressAllocation, err)
	}

	sc, err := s.NewSession(req.Supi, req.Access, req.Identity, req.Dnn, req.Snssai)
	if err != nil {
		return nil, ueAddresses{}, fmt.Errorf("%w: %v", errSessionIdentityInUse, err)
	}

	committed := false

	defer func() {
		if !committed {
			s.abortSession(ctx, sc)
		}
	}()

	// Build under the session lock so a concurrent reader for the same key never
	// sees a half-built context.
	sc.mu.Lock()
	sc.PDUSessionType = req.PDUType
	sc.PolicyData = req.Policy

	dlPdrIP, addrs, err := s.allocateUEAddresses(ctx, dn, sc)
	if err != nil {
		sc.mu.Unlock()
		return nil, ueAddresses{}, fmt.Errorf("%w: %v", errUEAddressAllocation, err)
	}

	// Framed routes are per-subscriber subscription data (TS 23.501 §5.6.14): they
	// attach to the session context, not the profile-shared Policy. A resolution
	// failure rejects establishment, fail-closed.
	framed, err := dn.ListFramedRoutes(ctx, req.Supi.IMSI())
	if err != nil {
		sc.mu.Unlock()
		return nil, ueAddresses{}, fmt.Errorf("%w: %v", errFramedRouteResolve, err)
	}

	sc.FramedRoutes = framed

	// Cache the reserved static address per family so a reconcile can detect a
	// reservation change; fail-closed on error.
	if sc.PDUIPV4Address != nil {
		addr, ok, err := dn.GetStaticIP(ctx, req.Supi.IMSI(), false)
		if err != nil {
			sc.mu.Unlock()
			return nil, ueAddresses{}, fmt.Errorf("%w: %v", errStaticIPResolve, err)
		}

		if ok {
			sc.StaticIPv4 = addr
		}
	}

	if sc.PDUIPV6Prefix != nil {
		addr, ok, err := dn.GetStaticIP(ctx, req.Supi.IMSI(), true)
		if err != nil {
			sc.mu.Unlock()
			return nil, ueAddresses{}, fmt.Errorf("%w: %v", errStaticIPResolve, err)
		}

		if ok {
			sc.StaticIPv6 = addr
		}
	}

	sc.Tunnel = &UPTunnel{DataPath: &DataPath{UpLinkTunnel: &GTPTunnel{}, DownLinkTunnel: &GTPTunnel{}}}

	if err := sc.Tunnel.DataPath.ActivateTunnelAndPDR(s, sc, req.Policy, dlPdrIP); err != nil {
		sc.mu.Unlock()
		return nil, ueAddresses{}, fmt.Errorf("%w: %v", errDataPathActivation, err)
	}

	sc.mu.Unlock() // applySession re-acquires it

	if err := s.applySessionLocking(ctx, sc); err != nil {
		return nil, ueAddresses{}, fmt.Errorf("%w: %v", errUPFSession, err)
	}

	committed = true

	return sc, addrs, nil
}

// abortSession rolls back a partially-created session sc: it releases the UPF
// session if one was established, frees whichever address leases were taken, and
// removes the context from the pool only if it still maps to sc (so a concurrent
// create that replaced the entry keeps its live session). The caller must not
// hold sc.mu.
func (s *SMF) abortSession(ctx context.Context, sc *SMContext) {
	if sc == nil {
		return
	}

	imsi := sc.Supi.IMSI()

	if sc.Tunnel != nil {
		if err := s.releaseTunnel(ctx, sc); err != nil {
			// Keep the lease so the address is not reused before its NAT conntrack is
			// purged (see releaseUserPlaneThenAddresses).
			logger.SmfLog.Warn("failed to release tunnel for aborted session; keeping IP lease", zap.String("imsi", imsi), zap.Error(err))
			s.dropFromPool(sc)

			return
		}
	}

	if sc.PDUIPV4Address != nil || sc.PDUIPV6Prefix != nil {
		dn, err := s.store.ResolveDNN(ctx, sc.Dnn)
		if err != nil {
			logger.SmfLog.Warn("failed to resolve data network to release UE addresses after aborted session", zap.String("imsi", imsi), zap.Error(err))
		} else {
			if sc.PDUIPV4Address != nil {
				if _, err := dn.ReleaseIP(ctx, imsi, sc.sessionKey()); err != nil {
					logger.SmfLog.Warn("failed to release UE IPv4 after aborted session", zap.String("imsi", imsi), zap.Error(err))
				}
			}

			if sc.PDUIPV6Prefix != nil {
				if _, err := dn.ReleaseIPv6(ctx, imsi, sc.sessionKey()); err != nil {
					logger.SmfLog.Warn("failed to release UE IPv6 after aborted session", zap.String("imsi", imsi), zap.Error(err))
				}
			}
		}
	}

	s.dropFromPool(sc)
}

// AnchorBinding is the access-network tunnel endpoint learned from the RAN: the
// eNB S1-U endpoint (4G, via the MME) or the gNB N3 endpoint (5G, from an N2
// transfer). Exactly one of IPv4/IPv6 is set.
type AnchorBinding struct {
	TEID uint32
	IPv4 net.IP
	IPv6 net.IP
}

// bindAccessTunnel points the downlink FAR at the AN tunnel endpoint and aligns
// the uplink OuterHeaderRemoval to its IP family, marking the downlink S1U flag by
// access (4G S1-U vs 5G N3 PSC; TS 29.281). The endpoint is always recorded in
// ANInformation; the FAR is updated only once the data path is activated. Caller
// holds sc.mu and marks rule State.
func (sc *SMContext) bindAccessTunnel(an AnchorBinding) {
	sc.Tunnel.ANInformation.TEID = an.TEID
	sc.Tunnel.ANInformation.IPv4Address = an.IPv4
	sc.Tunnel.ANInformation.IPv6Address = an.IPv6

	if !sc.Tunnel.DataPath.Activated {
		return
	}

	dl := sc.Tunnel.DataPath.DownLinkTunnel.PDR
	ul := sc.Tunnel.DataPath.UpLinkTunnel.PDR

	if dl.FAR.ForwardingParameters == nil {
		dl.FAR.ForwardingParameters = &models.ForwardingParameters{}
	}

	s1u := sc.IsEPS()

	if an.IPv6 != nil {
		dl.FAR.ForwardingParameters.OuterHeaderCreation = &models.OuterHeaderCreation{
			Description: models.OuterHeaderCreationGtpUUdpIpv6,
			TEID:        an.TEID,
			IPv6Address: an.IPv6,
			S1U:         s1u,
		}
		ohr := models.OuterHeaderRemovalGtpUUdpIpv6
		ul.OuterHeaderRemoval = &ohr
	} else {
		dl.FAR.ForwardingParameters.OuterHeaderCreation = &models.OuterHeaderCreation{
			Description: models.OuterHeaderCreationGtpUUdpIpv4,
			TEID:        an.TEID,
			IPv4Address: an.IPv4.To4(),
			S1U:         s1u,
		}
		ohr := models.OuterHeaderRemovalGtpUUdpIpv4
		ul.OuterHeaderRemoval = &ohr
	}
}

// applySessionLocking is applySession for a caller that does not hold
// smContext.mu.
func (s *SMF) applySessionLocking(ctx context.Context, smContext *SMContext) error {
	smContext.mu.Lock()
	defer smContext.mu.Unlock()

	return s.applySession(ctx, smContext, nil)
}

// applySession states the session's whole user plane to the UPF and adopts the
// resources the UPF answers with. policy names the policy whose SDF filters the
// session's PDRs use; nil takes the one the session holds, which is every
// caller but a move, where the target access's policy is being staged.
//
// A session with no activated data path states nothing: there are no rules for
// the UPF to converge to. Caller holds smContext.mu.
func (s *SMF) applySession(ctx context.Context, smContext *SMContext, policy *Policy) error {
	ctx, span := tracer.Start(ctx, "smf/apply_session",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	if smContext.Tunnel == nil || smContext.Tunnel.DataPath == nil || !smContext.Tunnel.DataPath.Activated {
		logger.WithTrace(ctx, logger.SmfLog).Debug("data path is not activated, nothing to state to the UPF")
		return nil
	}

	if smContext.UPFSession == nil {
		err := fmt.Errorf("UPF session context not initialized")
		span.RecordError(err)
		span.SetStatus(codes.Error, "UPF session context not initialized")

		return err
	}

	if policy == nil {
		policy = smContext.PolicyData
	}

	applied, err := s.upf.Apply(ctx, BuildSessionState(smContext, policy))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to apply the session state")

		return fmt.Errorf("failed to apply the UPF session state: %v", err)
	}

	smContext.UPFSession.Established = true

	// The local F-TEID is the UPF's, restated on every apply, so the endpoint the
	// SMF advertises to the RAN follows it.
	for _, up := range applied.UplinkPDRs {
		if up.PDRID != pdrIDUplink {
			continue
		}

		smContext.Tunnel.DataPath.UpLinkTunnel.TEID = up.TEID
		smContext.Tunnel.DataPath.UpLinkTunnel.N3IPv4 = up.N3IPv4
		smContext.Tunnel.DataPath.UpLinkTunnel.N3IPv6 = up.N3IPv6

		break
	}

	logger.WithTrace(ctx, logger.SmfLog).Debug("Applied the session state to the UPF",
		logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	return nil
}
