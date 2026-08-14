// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/internal/kernel"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/upf")

// ohcNoPSC mirrors the C OHC_NO_PSC modifier bit (utils/pdr.h): ORed onto a
// GTP-U outer-header-creation value to emit a plain G-PDU with no PDU Session
// Container, as required on 4G S1-U.
const ohcNoPSC uint8 = 0x10

// EstablishSession creates a new UPF session from typed Go structs,
// bypassing PFCP message encoding/decoding.
func (conn *SessionEngine) EstablishSession(ctx context.Context, req *models.EstablishRequest) (*models.EstablishResponse, error) {
	ctx, span := tracer.Start(ctx, "upf/establish_session",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("models.operation", "establish"),
			attribute.Int64("models.seid", int64(req.SEID)),
			attribute.String("ue.imsi", req.IMSI),
		),
	)
	defer span.End()

	seid := req.SEID

	// Defensive: a re-establish over a live SEID would orphan the old session's
	// datapath state. Not normally reached (SMF allocates a fresh SEID per session).
	if conn.GetSession(seid) != nil {
		if err := conn.DeleteSession(ctx, &models.DeleteRequest{SEID: seid}); err != nil {
			logger.WithTrace(ctx, logger.UpfLog).Warn("could not tear down existing session before re-establish",
				logger.SEID(seid), zap.Error(err))
		}
	}

	sess := NewSession(seid)
	sess.SetIMSI(req.IMSI)
	span.AddEvent("session_created", trace.WithAttributes(attribute.Int64("models.seid", int64(seid))))

	logger.WithTrace(ctx, logger.UpfLog).Debug("Tracking new session", logger.SEID(seid))

	var createdPDRs []SPDRInfo

	pdrContext := NewPDRCreationContext(sess, conn.FteIDResourceManager)

	farMap := make(map[uint32]ebpf.FarInfo)
	qerMap := make(map[uint32]ebpf.QerInfo)

	bpfObjects := conn.BpfObjects

	var txn sessionTxn

	for _, far := range req.FARs {
		farInfo := farInfoFromMerge(far, conn.n3AddressIPv4, conn.n3AddressIPv6, ebpf.FarInfo{})

		go addRemoteIPToNeigh(ctx, farInfo.RemoteIP)

		sess.PutFar(far.FARID, farInfo)
		farMap[far.FARID] = farInfo

		logger.WithTrace(ctx, logger.UpfLog).Info("Created Forwarding Action Rule",
			logger.FARID(far.FARID), zap.Any("farInfo", farInfo))
	}

	for _, qer := range req.QERs {
		qerInfo := qerInfoFromMerge(qer, ebpf.QerInfo{})

		sess.PutQer(qer.QERID, qerInfo)
		qerMap[qer.QERID] = qerInfo

		logger.WithTrace(ctx, logger.UpfLog).Info("Created QoS Enforcement Rule",
			logger.QERID(qer.QERID), zap.Any("qerInfo", qerInfo))
	}

	for _, urr := range req.URRs {
		if err := bpfObjects.NewUrr(seid, urr.URRID); err != nil {
			txn.rollback(ctx)
			span.RecordError(err)

			return nil, fmt.Errorf("can't put URR: %w", err)
		}

		txn.onRollback(func() error { return bpfObjects.DeleteUrr(seid, urr.URRID) })

		logger.WithTrace(ctx, logger.UpfLog).Debug("Created Usage Reporting Rule",
			logger.URRID(urr.URRID),
		)
	}

	// Hold filterMu across resolve → apply → register (below) so the filter slot
	// can't be released and reassigned to another policy before this session is
	// visible to propagateFilterIndex. UpdateFilters holds filterMu for writing
	// across its own propagation, so it either waits for this session to be
	// registered or completes before this resolves the index.
	if req.PolicyID != "" {
		conn.filterMu.RLock()
		defer conn.filterMu.RUnlock()
	}

	// Pre-scan for UE addresses: the IPv6 /64 arrives on a separate downlink PDR
	// that may be ordered after the uplink PDR, so per-PDR capture would miss it.
	var ueV4, ueV6 netip.Addr

	for _, pdr := range req.PDRs {
		if pdr.PDI.LocalFTEID != nil || !pdr.PDI.UEIPAddress.IsValid() {
			continue
		}

		if pdr.PDI.UEIPAddress.Is4() {
			ueV4 = pdr.PDI.UEIPAddress
		} else {
			ueV6 = pdr.PDI.UEIPAddress
		}
	}

	sess.SetUEAddresses(ueV4, ueV6)

	for _, pdr := range req.PDRs {
		spdrInfo := SPDRInfo{
			PdrID: uint32(pdr.PDRID),
			PdrInfo: ebpf.PdrInfo{
				SEID:  seid,
				PdrID: uint32(pdr.PDRID),
				IMSI:  req.IMSI,
			},
		}

		if err := pdrContext.ExtractPDR(pdr, &spdrInfo, farMap, qerMap); err != nil {
			txn.rollback(ctx)
			span.RecordError(err)

			return nil, fmt.Errorf("couldn't extract PDR info: %w", err)
		}

		if spdrInfo.Allocated {
			txn.onRollback(func() error {
				pdrContext.FteIDResourceManager.ReleaseTEID(sess.SEID, spdrInfo.TeID)
				return nil
			})
		}

		if req.PolicyID != "" {
			dir := models.DirectionUplink
			if spdrInfo.UEIP.IsValid() {
				dir = models.DirectionDownlink
			}

			spdrInfo.PdrInfo.FilterMapIndex = conn.resolveFilterIndexLocked(req.PolicyID, dir)
		}

		sess.PutPDR(spdrInfo.PdrID, spdrInfo)

		if err := applyPDR(spdrInfo, sess, bpfObjects); err != nil {
			txn.rollback(ctx)
			span.RecordError(err)

			return nil, fmt.Errorf("couldn't apply PDR: %w", err)
		}

		txn.onRollback(func() error { return unapplyPDR(spdrInfo, bpfObjects) })

		logger.WithTrace(ctx, logger.UpfLog).Info("Applied packet detection rule",
			logger.PDRID(spdrInfo.PdrID))

		createdPDRs = append(createdPDRs, spdrInfo)

		bpfObjects.ClearNotified(seid, pdr.PDRID)
	}

	span.AddEvent("pdrs_processed", trace.WithAttributes(attribute.Int("count", len(createdPDRs))))
	span.AddEvent("ebpf_maps_updated")

	// Framed routes (TS 23.501 §5.6.14, TS 29.244 §5.16) redirect to the
	// session's downlink PDR, matched by LPM.
	if len(req.FramedRoutes) > 0 {
		installed := make([]netip.Prefix, 0, len(req.FramedRoutes))

		for _, fr := range req.FramedRoutes {
			ueAddr := ueV6
			if fr.Addr().Is4() {
				ueAddr = ueV4
			}

			if !ueAddr.IsValid() {
				// No same-family downlink PDR (e.g. an IPv6 framed route on an
				// IPv4-only session): the route cannot apply here, so skip it. A
				// dormant route must not deny the UE all connectivity.
				logger.WithTrace(ctx, logger.UpfLog).Warn("Skipping framed route with no same-family downlink PDR",
					logger.SEID(seid), zap.String("prefix", fr.String()))

				continue
			}

			if err := bpfObjects.PutFramedDownlink(fr, ueAddr); err != nil {
				txn.rollback(ctx)
				span.RecordError(err)

				return nil, fmt.Errorf("couldn't apply framed route %s: %w", fr, err)
			}

			txn.onRollback(func() error { return bpfObjects.DeleteFramedDownlink(fr) })

			installed = append(installed, fr)
		}

		sess.SetFramedRoutes(installed)

		logger.WithTrace(ctx, logger.UpfLog).Info("Applied framed routes",
			logger.SEID(seid), zap.Int("count", len(installed)))
	}

	if req.PolicyID != "" {
		sess.SetPolicyID(req.PolicyID)
	}

	conn.mu.Lock()
	conn.sessions[seid] = sess
	conn.registerPolicy(req.PolicyID, seid)
	conn.mu.Unlock()

	logger.WithTrace(ctx, logger.UpfLog).Debug("Accepted Session Establishment Request")

	return &models.EstablishResponse{
		N3TEID: uplinkTEID(createdPDRs),
		N3IPv4: conn.GetAdvertisedN3Address(),
		N3IPv6: conn.GetAdvertisedN3AddressIPv6(),
	}, nil
}

// The apply-action bits the eBPF data plane reads (enum far_action_mask in
// utils/pdr.h), named for the flags TS 29.244 §8.2.26 defines.
const (
	farDrop uint8 = 1 << iota
	farForward
	farBuffer
	farNotifyCP
	farDuplicate
)

// encodeApplyAction packs ApplyAction bools into the uint8 bit layout
// expected by the eBPF data plane.
func encodeApplyAction(a models.ApplyAction) uint8 {
	var v uint8
	if a.Drop {
		v |= farDrop
	}

	if a.Forw {
		v |= farForward
	}

	if a.Buff {
		v |= farBuffer
	}

	if a.Nocp {
		v |= farNotifyCP
	}

	if a.Dupl {
		v |= farDuplicate
	}

	return v
}

// A session has exactly one uplink PDR that asks for an F-TEID; 0 means none
// was requested.
func uplinkTEID(createdPDRs []SPDRInfo) uint32 {
	for _, pdr := range createdPDRs {
		if pdr.Allocated && !pdr.UEIP.IsValid() {
			return pdr.TeID
		}
	}

	return 0
}

// addRemoteIPToNeigh adds the given remote IP (as an in6_addr [16]byte) to the kernel
// neighbour table so that GTP encapsulated packets can be forwarded.
func addRemoteIPToNeigh(ctx context.Context, remoteIP [16]byte) {
	var zero [16]byte
	if remoteIP == zero {
		return
	}

	ip := ebpf.In6AddrToIP(remoteIP)
	if !ip.IsValid() {
		return
	}

	if err := kernel.AddNeighbour(ctx, ip); err != nil {
		logger.UpfLog.Warn("could not add gnb IP to neighbour list", logger.IPAddress(ip.String()), zap.Error(err))
	}
}
