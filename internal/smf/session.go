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
	errUEAddressAllocation = errors.New("UE address allocation failed")
	errFramedRouteResolve  = errors.New("framed route resolution failed")
	errStaticIPResolve     = errors.New("static IP resolution failed")
	errUPFSession          = errors.New("UPF session establishment failed")
	errSessionIdentity     = errors.New("session identity is unusable")
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
// the data path, and establishes the UPF (PFCP) session. On failure it rolls the
// partial session back and wraps a sentinel error for the adapter to map to its
// NAS cause.
func (s *SMF) establishSession(ctx context.Context, req SessionRequest) (*SMContext, ueAddresses, error) {
	dn, err := s.store.ResolveDNN(ctx, req.Dnn)
	if err != nil {
		return nil, ueAddresses{}, fmt.Errorf("%w: %v", errUEAddressAllocation, err)
	}

	sc, err := s.NewSession(req.Supi, req.Access, req.Identity, req.Dnn, req.Snssai)
	if err != nil {
		return nil, ueAddresses{}, fmt.Errorf("%w: %v", errSessionIdentity, err)
	}

	committed := false

	defer func() {
		if !committed {
			s.abortSession(ctx, sc)
		}
	}()

	// Build under the session lock so a concurrent reader for the same key never
	// sees a half-built context.
	sc.Mutex.Lock()
	sc.PDUSessionType = req.PDUType
	sc.PolicyData = req.Policy

	dlPdrIP, addrs, err := s.allocateUEAddresses(ctx, dn, sc)
	if err != nil {
		sc.Mutex.Unlock()
		return nil, ueAddresses{}, fmt.Errorf("%w: %v", errUEAddressAllocation, err)
	}

	// Framed routes are per-subscriber subscription data (TS 23.501 §5.6.14): they
	// attach to the session context, not the profile-shared Policy. A resolution
	// failure rejects establishment, fail-closed.
	framed, err := dn.ListFramedRoutes(ctx, req.Supi.IMSI())
	if err != nil {
		sc.Mutex.Unlock()
		return nil, ueAddresses{}, fmt.Errorf("%w: %v", errFramedRouteResolve, err)
	}

	sc.FramedRoutes = framed

	// Cache the reserved static address per family so a reconcile can detect a
	// reservation change; fail-closed on error.
	if sc.PDUIPV4Address != nil {
		addr, ok, err := dn.GetStaticIP(ctx, req.Supi.IMSI(), false)
		if err != nil {
			sc.Mutex.Unlock()
			return nil, ueAddresses{}, fmt.Errorf("%w: %v", errStaticIPResolve, err)
		}

		if ok {
			sc.StaticIPv4 = addr
		}
	}

	if sc.PDUIPV6Prefix != nil {
		addr, ok, err := dn.GetStaticIP(ctx, req.Supi.IMSI(), true)
		if err != nil {
			sc.Mutex.Unlock()
			return nil, ueAddresses{}, fmt.Errorf("%w: %v", errStaticIPResolve, err)
		}

		if ok {
			sc.StaticIPv6 = addr
		}
	}

	var v6Prefix net.IP
	if sc.PDUIPV4Address != nil && sc.PDUIPV6Prefix != nil {
		v6Prefix = sc.PDUIPV6Prefix
	}

	sc.Tunnel = &UPTunnel{}
	sc.SetPFCPSession(s.AllocateSEID())
	sc.Tunnel.Activate(req.Policy, dlPdrIP, v6Prefix)

	sc.Mutex.Unlock() // establishPFCPSession re-acquires it

	if err := s.establishPFCPSession(ctx, sc); err != nil {
		return nil, ueAddresses{}, fmt.Errorf("%w: %v", errUPFSession, err)
	}

	committed = true

	return sc, addrs, nil
}

// abortSession rolls back a partially-created session sc: it releases the UPF
// session if one was established, frees whichever address leases were taken, and
// removes the context from the pool only if it still maps to sc (so a concurrent
// create that replaced the entry keeps its live session). The caller must not
// hold sc.Mutex.
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
// access (4G S1-U vs 5G N3 PSC; TS 29.281). The endpoint is always recorded on
// the tunnel; the FAR is updated only once the rules exist. Caller holds sc.Mutex.
func (sc *SMContext) bindAccessTunnel(an AnchorBinding) {
	if sc.Tunnel == nil {
		return
	}

	sc.Tunnel.AN = an

	if !sc.Tunnel.Activated {
		return
	}

	dl := sc.Tunnel.DownlinkPDR
	ul := sc.Tunnel.UplinkPDR

	if dl.FAR.ForwardingParameters == nil {
		dl.FAR.ForwardingParameters = &models.ForwardingParameters{}
	}

	s1u := sc.Access == Access4G

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

func (s *SMF) establishPFCPSession(ctx context.Context, smContext *SMContext) error {
	ctx, span := tracer.Start(ctx, "smf/send_pfcp_rules",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	tunnel := smContext.Tunnel
	if !tunnel.Activated {
		logger.WithTrace(ctx, logger.SmfLog).Debug("data path is not activated, skip sending PFCP rules")
		return nil
	}

	if smContext.PFCPContext == nil {
		span.RecordError(fmt.Errorf("PFCP context not initialized"))
		span.SetStatus(codes.Error, "PFCP context not initialized")

		return fmt.Errorf("PFCP context not initialized")
	}

	var policyID string
	if smContext.PolicyData != nil {
		policyID = smContext.PolicyData.PolicyID
	}

	if smContext.PFCPContext.Established {
		return fmt.Errorf("PFCP session already established")
	}

	req := BuildEstablishRequest(
		smContext.PFCPContext.SEID,
		smContext.Supi.IMSI(),
		policyID,
		tunnel.PDRs(), tunnel.FARs(), tunnel.QERs(), tunnel.URRs(),
	)
	req.FramedRoutes = smContext.FramedRoutes

	resp, err := s.upf.EstablishSession(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to establish PFCP session")

		return fmt.Errorf("failed to send PFCP session establishment request: %v", err)
	}

	// Before this the UPF has never heard of the SEID.
	smContext.PFCPContext.Established = true

	tunnel.N3TEID, tunnel.N3IPv4, tunnel.N3IPv6 = resp.N3TEID, resp.N3IPv4, resp.N3IPv6

	return nil
}
