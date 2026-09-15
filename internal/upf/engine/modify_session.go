// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"context"
	"fmt"
	"maps"
	"net/netip"
	"slices"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tracing/attrs"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// ModifySession modifies an existing UPF session from typed Go structs.
func (conn *SessionEngine) ModifySession(ctx context.Context, req *models.ModifyRequest) (*models.ModifyResponse, error) {
	ctx, span := tracer.Start(ctx, "upf/modify_session",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attrs.SessionOperation("modify"),
			attrs.SEID(req.SEID),
		),
	)
	defer span.End()

	session := conn.GetSession(req.SEID)
	if session == nil {
		err := fmt.Errorf("%w: SEID %d", models.ErrSessionNotFound, req.SEID)
		span.RecordError(err)
		span.SetStatus(codes.Error, "session not found")

		return nil, err
	}

	// Held across resolve → apply, and before opMu (filterMu is the outermost
	// engine lock), so the slot resolved here cannot be freed and reissued under it.
	conn.filterMu.RLock()

	session.opMu.Lock()

	drain, endMarkers, resp, err := conn.modifySessionLocked(ctx, span, req, session)

	session.opMu.Unlock()
	conn.filterMu.RUnlock()

	if err != nil {
		return nil, err
	}

	conn.sendEndMarkers(endMarkers)

	if drain {
		if b := conn.downlinkBuffer(); b != nil {
			b.Drain(req.SEID)
		}
	}

	return resp, nil
}

// modifySessionLocked applies the modification and reports whether any PDR's
// FAR transitioned into forwarding. The caller holds filterMu (read) and
// session.opMu.
func (conn *SessionEngine) modifySessionLocked(ctx context.Context, span trace.Span, req *models.ModifyRequest, session *Session) (bool, []endMarkerTarget, *models.ModifyResponse, error) {
	if session.deleted {
		err := fmt.Errorf("session %d is being deleted", req.SEID)
		span.RecordError(err)

		return false, nil, nil, err
	}

	bpfObjects := conn.BpfObjects

	pdrContext := NewPDRCreationContext(session, conn.FteIDResourceManager)

	snapPDRs, snapFARs, snapQERs := session.snapshot()

	var txn sessionTxn

	fail := func(err error) (bool, []endMarkerTarget, *models.ModifyResponse, error) {
		txn.rollback(ctx)
		session.restore(snapPDRs, snapFARs, snapQERs)
		span.RecordError(err)

		return false, nil, nil, err
	}

	touched := make(map[uint32]struct{}, len(req.UpdatePDRs))

	drain := false

	var endMarkers []endMarkerTarget

	for _, far := range req.UpdateFARs {
		oldFarInfo := session.GetFar(far.FARID)

		sFarInfo := farInfoFromMerge(far, conn.n3AddressIPv4, conn.n3AddressIPv6, oldFarInfo)

		if req.SendEndMarkers {
			if target, ok := endMarkerTargetFor(oldFarInfo, sFarInfo); ok {
				endMarkers = append(endMarkers, target)
			}
		}

		go addRemoteIPToNeigh(ctx, sFarInfo.RemoteIP)

		session.PutFar(far.FARID, sFarInfo)

		restampReferencingPDRs(session, touched, func(p SPDRInfo) bool { return p.PdrInfo.FarID == far.FARID },
			func(p *SPDRInfo) { p.PdrInfo.Far = sFarInfo })

		logger.WithTrace(ctx, logger.UpfLog).Debug("Updated Forwarding Action Rule",
			logger.FARID(far.FARID), zap.Any("far_info", sFarInfo))
	}

	for _, qer := range req.UpdateQERs {
		qerInfo := qerInfoFromMerge(qer, session.GetQer(qer.QERID))

		session.PutQer(qer.QERID, qerInfo)

		restampReferencingPDRs(session, touched, func(p SPDRInfo) bool { return p.PdrInfo.QerID == qer.QERID },
			func(p *SPDRInfo) { p.PdrInfo.Qer = qerInfo })

		logger.WithTrace(ctx, logger.UpfLog).Debug("Updated QoS Enforcement Rule",
			logger.QERID(qer.QERID), zap.Any("qer_info", qerInfo))
	}

	farMap := make(map[uint32]ebpf.FarInfo)
	maps.Copy(farMap, session.ListFARs())

	destinations := farDestinations(req.UpdateFARs)

	qerMap := make(map[uint32]ebpf.QerInfo)
	maps.Copy(qerMap, session.ListQERs())

	for _, pdr := range req.UpdatePDRs {
		old, hadOld := session.LookupPDR(uint32(pdr.PDRID))

		spdrInfo := old
		if !hadOld {
			spdrInfo.PdrID = uint32(pdr.PDRID)
			spdrInfo.PdrInfo.SEID = req.SEID
			spdrInfo.PdrInfo.PdrID = uint32(pdr.PDRID)
			spdrInfo.PdrInfo.IMSI = session.IMSI()
		}

		allocated, err := pdrContext.ExtractPDR(pdr, &spdrInfo, farMap, destinations, qerMap)
		if err != nil {
			return fail(fmt.Errorf("couldn't extract PDR info: %w", err))
		}

		if allocated {
			txn.onRollback(func() error {
				pdrContext.FteIDResourceManager.ReleaseTEID(session.SEID, spdrInfo.TeID)
				return nil
			})
		}

		if policyID := modifyPolicyID(req, session); policyID != "" {
			spdrInfo.PdrInfo.FilterMapIndex = conn.resolveFilterIndexLocked(policyID, pdrDirection(spdrInfo))
		}

		session.PutPDR(uint32(pdr.PDRID), spdrInfo)

		touched[uint32(pdr.PDRID)] = struct{}{}
	}

	for _, pdrID := range slices.Sorted(maps.Keys(touched)) {
		spdrInfo := session.GetPDR(pdrID)
		old, hadOld := snapPDRs[pdrID]

		if err := applyPDR(spdrInfo, session, bpfObjects); err != nil {
			return fail(fmt.Errorf("couldn't apply PDR: %w", err))
		}

		txn.onRollback(func() error {
			if err := unapplyPDR(spdrInfo, bpfObjects); err != nil {
				return err
			}

			if hadOld {
				return applyPDR(old, session, bpfObjects)
			}

			return nil
		})

		if hadOld && pdrKeyChanged(old, spdrInfo) {
			if err := unapplyPDR(old, bpfObjects); err != nil {
				return fail(fmt.Errorf("couldn't remove the superseded PDR entry: %w", err))
			}
		}

		if spdrInfo.PdrInfo.Far.Action&farForward != 0 {
			bpfObjects.ClearNotified(req.SEID, uint16(pdrID))

			if !hadOld || old.PdrInfo.Far.Action&farForward == 0 {
				drain = true
			}
		}
	}

	for _, pdrID := range req.RemovePDRs {
		removed, ok := session.RemovePDR(uint32(pdrID))
		if !ok {
			continue
		}

		if err := pdrContext.deletePDR(removed, bpfObjects); err != nil {
			return fail(fmt.Errorf("couldn't remove PDR %d: %w", pdrID, err))
		}

		txn.onRollback(func() error {
			session.PutPDR(uint32(pdrID), removed)

			return applyPDR(removed, session, bpfObjects)
		})
	}

	for _, farID := range req.RemoveFARs {
		session.RemoveFar(farID)
	}

	if req.PolicyID != "" && req.PolicyID != session.PolicyID() {
		oldPolicyID := session.PolicyID()
		session.SetPolicyID(req.PolicyID)

		conn.mu.Lock()
		conn.deregisterPolicy(oldPolicyID, session.SEID)
		conn.registerPolicy(req.PolicyID, session.SEID)
		conn.mu.Unlock()
	}

	logger.WithTrace(ctx, logger.UpfLog).Debug("Session modification successful")

	return drain, endMarkers, &models.ModifyResponse{ForwardingTEID: forwardingTEID(session)}, nil
}

func forwardingTEID(session *Session) uint32 {
	for _, pdr := range session.ListPDRs() {
		if pdr.TeID == 0 || pdr.UEIP.IsValid() {
			continue
		}

		if pdr.PdrInfo.Far.OuterHeaderCreation != 0 {
			return pdr.TeID
		}
	}

	return 0
}

func modifyPolicyID(req *models.ModifyRequest, session *Session) string {
	if req.PolicyID != "" {
		return req.PolicyID
	}

	return session.PolicyID()
}

func restampReferencingPDRs(session *Session, touched map[uint32]struct{}, matches func(SPDRInfo) bool, mutate func(*SPDRInfo)) {
	for _, spdrInfo := range session.ListPDRs() {
		if !matches(spdrInfo) {
			continue
		}

		mutate(&spdrInfo)
		session.PutPDR(spdrInfo.PdrID, spdrInfo)

		touched[spdrInfo.PdrID] = struct{}{}
	}
}

// farInfoFromMerge merges a models.FAR into an existing ebpf.FarInfo.
func farInfoFromMerge(far models.FAR, localIPv4 netip.Addr, localIPv6 netip.Addr, existing ebpf.FarInfo) ebpf.FarInfo {
	existing.Action = encodeApplyAction(far.ApplyAction)

	if fp := far.ForwardingParameters; fp != nil {
		if ohc := fp.OuterHeaderCreation; ohc != nil {
			existing.OuterHeaderCreation = uint8(ohc.Description >> 8)
			if ohc.S1U {
				existing.OuterHeaderCreation |= ohcNoPSC
			}

			existing.TeID = ohc.TEID

			if ohc.Description == models.OuterHeaderCreationGtpUUdpIpv6 && ohc.IPv6Address != nil {
				existing.LocalIP = ebpf.IPToIn6Addr(localIPv6)

				v6 := ohc.IPv6Address.To16()
				if v6 != nil {
					var v6arr [16]byte
					copy(v6arr[:], v6)
					existing.RemoteIP = v6arr
				}
			} else if ohc.IPv4Address != nil {
				existing.LocalIP = ebpf.IPToIn6Addr(localIPv4)

				ip4 := ohc.IPv4Address.To4()
				if ip4 != nil {
					var ip4arr [4]byte
					copy(ip4arr[:], ip4)
					existing.RemoteIP = ebpf.IPToIn6Addr(netip.AddrFrom4(ip4arr))
				}
			} else {
				existing.LocalIP = ebpf.IPToIn6Addr(localIPv4)
			}
		}
	}

	return existing
}

// qerInfoFromMerge merges a models.QER into an existing ebpf.QerInfo.
func qerInfoFromMerge(qer models.QER, existing ebpf.QerInfo) ebpf.QerInfo {
	existing.Qfi = qer.QFI

	if qer.GateStatus != nil {
		existing.GateStatusDL = qer.GateStatus.DLGate
		existing.GateStatusUL = qer.GateStatus.ULGate
	}

	if qer.MBR != nil {
		existing.MaxBitrateDL = qer.MBR.DLMBR * 1000
		existing.MaxBitrateUL = qer.MBR.ULMBR * 1000
	}

	return existing
}
