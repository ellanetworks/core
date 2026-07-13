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
	errDataPathActivation  = errors.New("data path activation failed")
	errUPFSession          = errors.New("UPF session establishment failed")
)

// SessionRequest is the RAT-agnostic input to establishSession, common to the
// 5G and 4G paths.
type SessionRequest struct {
	Supi    etsi.SUPI
	Key     uint8 // PDU Session ID (5G) or default-bearer EBI (4G)
	Dnn     string
	Snssai  *models.Snssai // nil for 4G
	Access  AccessType
	PDUType uint8 // the negotiated PDU/PDN type
	Policy  *Policy
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
	sc := s.NewSession(req.Supi, req.Key, req.Dnn, req.Snssai)

	committed := false

	defer func() {
		if !committed {
			s.abortSession(ctx, sc)
		}
	}()

	// Build under the session lock so a concurrent reader for the same key never
	// sees a half-built context.
	sc.Mutex.Lock()
	sc.Access = req.Access
	sc.PDUSessionType = req.PDUType
	sc.PolicyData = req.Policy

	dlPdrIP, addrs, err := s.allocateUEAddresses(ctx, sc)
	if err != nil {
		sc.Mutex.Unlock()
		return nil, ueAddresses{}, fmt.Errorf("%w: %v", errUEAddressAllocation, err)
	}

	// Framed routes are per-subscriber subscription data (TS 23.501 §5.6.14): they
	// attach to the session context, not the profile-shared Policy. A resolution
	// failure rejects establishment, fail-closed.
	framed, err := s.store.ListFramedRoutes(ctx, req.Supi.IMSI(), req.Dnn)
	if err != nil {
		sc.Mutex.Unlock()
		return nil, ueAddresses{}, fmt.Errorf("%w: %v", errFramedRouteResolve, err)
	}

	sc.FramedRoutes = framed

	sc.Tunnel = &UPTunnel{DataPath: &DataPath{UpLinkTunnel: &GTPTunnel{}, DownLinkTunnel: &GTPTunnel{}}}

	if err := sc.Tunnel.DataPath.ActivateTunnelAndPDR(s, sc, req.Policy, dlPdrIP); err != nil {
		sc.Mutex.Unlock()
		return nil, ueAddresses{}, fmt.Errorf("%w: %v", errDataPathActivation, err)
	}

	sc.Mutex.Unlock() // sendPFCPRules re-acquires it

	if err := s.sendPFCPRules(ctx, sc); err != nil {
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
			logger.SmfLog.Warn("failed to release tunnel for aborted session", zap.String("imsi", imsi), zap.Error(err))
		}
	}

	if sc.PDUIPV4Address != nil {
		if _, err := s.store.ReleaseIP(ctx, imsi, sc.Dnn, sc.PDUSessionID); err != nil {
			logger.SmfLog.Warn("failed to release UE IPv4 after aborted session", zap.String("imsi", imsi), zap.Error(err))
		}
	}

	if sc.PDUIPV6Prefix != nil {
		if _, err := s.store.ReleaseIPv6(ctx, imsi, sc.Dnn, sc.PDUSessionID); err != nil {
			logger.SmfLog.Warn("failed to release UE IPv6 after aborted session", zap.String("imsi", imsi), zap.Error(err))
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
// holds sc.Mutex and marks rule State.
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

func (s *SMF) sendPFCPRules(ctx context.Context, smContext *SMContext) error {
	ctx, span := tracer.Start(ctx, "smf/send_pfcp_rules",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	dataPath := smContext.Tunnel.DataPath
	if !dataPath.Activated {
		logger.WithTrace(ctx, logger.SmfLog).Debug("DataPath is not activated, skip sending PFCP rules")
		return nil
	}

	pdrList := make([]*PDR, 0, 3)
	farList := make([]*FAR, 0, 2)
	qerList := make([]*QER, 0, 2)
	urrList := make([]*URR, 0, 2)

	if dataPath.UpLinkTunnel != nil && dataPath.UpLinkTunnel.PDR != nil {
		pdrList = append(pdrList, dataPath.UpLinkTunnel.PDR)
		farList = append(farList, dataPath.UpLinkTunnel.PDR.FAR)

		if dataPath.UpLinkTunnel.PDR.QER != nil {
			qerList = append(qerList, dataPath.UpLinkTunnel.PDR.QER)
		}

		if dataPath.UpLinkTunnel.PDR.URR != nil {
			urrList = append(urrList, dataPath.UpLinkTunnel.PDR.URR)
		}
	}

	if dataPath.DownLinkTunnel != nil && dataPath.DownLinkTunnel.PDR != nil {
		pdrList = append(pdrList, dataPath.DownLinkTunnel.PDR)
		farList = append(farList, dataPath.DownLinkTunnel.PDR.FAR)

		if dataPath.DownLinkTunnel.PDR.QER != nil {
			qerList = append(qerList, dataPath.DownLinkTunnel.PDR.QER)
		}

		if dataPath.DownLinkTunnel.PDR.URR != nil {
			urrList = append(urrList, dataPath.DownLinkTunnel.PDR.URR)
		}
	}

	if dataPath.SecondPDR != nil {
		pdrList = append(pdrList, dataPath.SecondPDR)

		if dataPath.SecondPDR.QER != nil {
			qerList = append(qerList, dataPath.SecondPDR.QER)
		}

		if dataPath.SecondPDR.URR != nil {
			urrList = append(urrList, dataPath.SecondPDR.URR)
		}
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

	if smContext.PFCPContext.RemoteSEID == 0 {
		req := BuildEstablishRequest(
			smContext.PFCPContext.LocalSEID,
			smContext.Supi.IMSI(),
			policyID,
			pdrList, farList, qerList, urrList,
		)
		req.FramedRoutes = smContext.FramedRoutes

		resp, err := s.upf.EstablishSession(ctx, req)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to establish PFCP session")

			return fmt.Errorf("failed to send PFCP session establishment request: %v", err)
		}

		smContext.PFCPContext.RemoteSEID = resp.RemoteSEID

		for _, cp := range resp.CreatedPDRs {
			if cp.TEID != 0 {
				smContext.Tunnel.DataPath.UpLinkTunnel.TEID = cp.TEID
				smContext.Tunnel.DataPath.UpLinkTunnel.N3IPv4 = cp.N3IPv4
				smContext.Tunnel.DataPath.UpLinkTunnel.N3IPv6 = cp.N3IPv6

				break
			}
		}

		if dataPath.DownLinkTunnel != nil && dataPath.DownLinkTunnel.PDR != nil {
			dataPath.DownLinkTunnel.PDR.State = RuleCreate
		}

		if dataPath.SecondPDR != nil {
			dataPath.SecondPDR.State = RuleCreate
		}

		return nil
	}

	err := s.upf.ModifySession(ctx, BuildModifyRequest(
		smContext.PFCPContext.RemoteSEID,
		policyID,
		pdrList, farList, qerList,
	))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to modify PFCP session")

		return fmt.Errorf("failed to send PFCP session modification request: %v", err)
	}

	logger.WithTrace(ctx, logger.SmfLog).Info("Sent PFCP session modification request to upf")

	return nil
}
