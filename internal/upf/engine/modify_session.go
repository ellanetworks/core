// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/netip"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// ModifySession modifies an existing UPF session from typed Go structs.
func (conn *SessionEngine) ModifySession(ctx context.Context, req *models.ModifyRequest) error {
	ctx, span := tracer.Start(ctx, "upf/modify_session",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("session.operation", "modify"),
			attribute.Int64("session.seid", int64(req.SEID)),
		),
	)
	defer span.End()

	session := conn.GetSession(req.SEID)
	if session == nil {
		err := fmt.Errorf("session not found for SEID %d", req.SEID)
		span.RecordError(err)
		span.SetStatus(codes.Error, "session not found")

		return err
	}

	session.opMu.Lock()
	defer session.opMu.Unlock()

	if session.deleted {
		err := fmt.Errorf("session %d is being deleted", req.SEID)
		span.RecordError(err)

		return err
	}

	bpfObjects := conn.BpfObjects

	pdrContext := NewPDRCreationContext(session, conn.FteIDResourceManager)

	// Removals run last so a rollback never has to re-create a torn-down URR/TEID;
	// a failed create/update unwinds its eBPF writes and restores the snapshot.
	snapPDRs, snapFARs, snapQERs := session.snapshot()

	var txn sessionTxn

	fail := func(err error) error {
		txn.rollback(ctx)
		session.restore(snapPDRs, snapFARs, snapQERs)
		span.RecordError(err)

		return err
	}

	// --- FAR / QER create and update ---

	for _, far := range req.CreateFARs {
		farInfo := farInfoFromModel(far, conn.n3AddressIPv4, conn.n3AddressIPv6)

		go addRemoteIPToNeigh(ctx, farInfo.RemoteIP)

		session.PutFar(far.FARID, farInfo)

		logger.WithTrace(ctx, logger.UpfLog).Info("Created Forwarding Action Rule",
			logger.FARID(far.FARID), zap.Any("farInfo", farInfo))
	}

	for _, far := range req.UpdateFARs {
		sFarInfo := session.GetFar(far.FARID)
		sFarInfo = farInfoFromMerge(far, conn.n3AddressIPv4, conn.n3AddressIPv6, sFarInfo)

		go addRemoteIPToNeigh(ctx, sFarInfo.RemoteIP)

		session.PutFar(far.FARID, sFarInfo)

		if err := conn.reapplyReferencingPDRs(session, &txn, func(p SPDRInfo) bool { return p.PdrInfo.FarID == far.FARID },
			func(p *SPDRInfo) { p.PdrInfo.Far = sFarInfo }); err != nil {
			return fail(fmt.Errorf("can't update PDR after FAR update: %w", err))
		}

		logger.WithTrace(ctx, logger.UpfLog).Info("Updated Forwarding Action Rule",
			logger.FARID(far.FARID), zap.Any("farInfo", sFarInfo))
	}

	for _, qer := range req.CreateQERs {
		qerInfo := qerInfoFromModel(qer)

		session.NewQer(qer.QERID, qerInfo)

		if err := conn.reapplyReferencingPDRs(session, &txn, func(p SPDRInfo) bool { return p.PdrInfo.QerID == qer.QERID },
			func(p *SPDRInfo) { p.PdrInfo.Qer = qerInfo }); err != nil {
			return fail(fmt.Errorf("can't apply PDR for new QER: %w", err))
		}

		logger.WithTrace(ctx, logger.UpfLog).Info("Created QoS Enforcement Rule",
			logger.QERID(qer.QERID), zap.Any("qerInfo", qerInfo))
	}

	for _, qer := range req.UpdateQERs {
		qerInfo := qerInfoFromMerge(qer, session.GetQer(qer.QERID))

		session.PutQer(qer.QERID, qerInfo)

		if err := conn.reapplyReferencingPDRs(session, &txn, func(p SPDRInfo) bool { return p.PdrInfo.QerID == qer.QERID },
			func(p *SPDRInfo) { p.PdrInfo.Qer = qerInfo }); err != nil {
			return fail(fmt.Errorf("can't update PDR after QER update: %w", err))
		}

		logger.WithTrace(ctx, logger.UpfLog).Info("Updated QoS Enforcement Rule",
			logger.QERID(qer.QERID), zap.Any("qerInfo", qerInfo))
	}

	// --- PDR create and update ---

	farMap := make(map[uint32]ebpf.FarInfo)
	maps.Copy(farMap, session.ListFARs())

	qerMap := make(map[uint32]ebpf.QerInfo)
	maps.Copy(qerMap, session.ListQERs())

	for _, pdr := range req.CreatePDRs {
		spdrInfo := SPDRInfo{
			PdrID: uint32(pdr.PDRID),
			PdrInfo: ebpf.PdrInfo{
				SEID:  req.SEID,
				PdrID: uint32(pdr.PDRID),
			},
		}

		if err := pdrContext.ExtractPDR(pdr, &spdrInfo, farMap, qerMap); err != nil {
			return fail(fmt.Errorf("couldn't extract PDR info: %w", err))
		}

		if spdrInfo.Allocated {
			txn.onRollback(func() error {
				pdrContext.FteIDResourceManager.ReleaseTEID(session.SEID, spdrInfo.TeID)
				return nil
			})
		}

		policyID := req.PolicyID
		if policyID == "" {
			policyID = session.PolicyID()
		}

		if policyID != "" {
			dir := models.DirectionUplink
			if spdrInfo.UEIP.IsValid() {
				dir = models.DirectionDownlink
			}

			spdrInfo.PdrInfo.FilterMapIndex = conn.resolveFilterIndex(policyID, dir)
		}

		session.PutPDR(spdrInfo.PdrID, spdrInfo)

		if err := applyPDR(spdrInfo, session, bpfObjects); err != nil {
			return fail(fmt.Errorf("couldn't apply PDR: %w", err))
		}

		txn.onRollback(func() error { return unapplyPDR(spdrInfo, bpfObjects) })

		bpfObjects.ClearNotified(req.SEID, pdr.PDRID, spdrInfo.PdrInfo.Qer.Qfi)
	}

	for _, pdr := range req.UpdatePDRs {
		old, hadOld := session.LookupPDR(uint32(pdr.PDRID))
		spdrInfo := old

		if err := pdrContext.ExtractPDR(pdr, &spdrInfo, farMap, qerMap); err != nil {
			return fail(fmt.Errorf("couldn't extract PDR info: %w", err))
		}

		if spdrInfo.Allocated {
			txn.onRollback(func() error {
				pdrContext.FteIDResourceManager.ReleaseTEID(session.SEID, spdrInfo.TeID)
				return nil
			})
		}

		session.PutPDR(uint32(pdr.PDRID), spdrInfo)

		if err := applyPDR(spdrInfo, session, bpfObjects); err != nil {
			return fail(fmt.Errorf("couldn't apply PDR: %w", err))
		}

		// Rollback removes the entry just written (which may be under a new key if
		// the update changed UEIP/TEID) and restores the prior one, if any.
		txn.onRollback(func() error {
			if err := unapplyPDR(spdrInfo, bpfObjects); err != nil {
				return err
			}

			if hadOld {
				return applyPDR(old, session, bpfObjects)
			}

			return nil
		})

		bpfObjects.ClearNotified(req.SEID, pdr.PDRID, spdrInfo.PdrInfo.Qer.Qfi)
	}

	// --- Removals (best-effort; the create/update phase above has committed) ---

	for _, farID := range req.RemoveFARIDs {
		session.RemoveFar(farID)
	}

	for _, qerID := range req.RemoveQERIDs {
		session.RemoveQer(qerID)
	}

	var removeErr error

	for _, pdrID := range req.RemovePDRIDs {
		if !session.HasPDR(uint32(pdrID)) {
			continue
		}

		// Delete from the data plane first; keep the in-memory PDR if that fails
		// so DeleteSession can still reach it.
		sPDRInfo := session.GetPDR(uint32(pdrID))
		if err := pdrContext.deletePDR(sPDRInfo, bpfObjects); err != nil {
			removeErr = errors.Join(removeErr, fmt.Errorf("couldn't delete PDR %d: %w", pdrID, err))
			continue
		}

		session.RemovePDR(uint32(pdrID))
	}

	if req.PolicyID != "" && req.PolicyID != session.PolicyID() {
		oldPolicyID := session.PolicyID()
		session.SetPolicyID(req.PolicyID)

		conn.mu.Lock()
		conn.deregisterPolicy(oldPolicyID, session.SEID)
		conn.registerPolicy(req.PolicyID, session.SEID)
		conn.mu.Unlock()
	}

	if removeErr != nil {
		span.RecordError(removeErr)
		return removeErr
	}

	logger.WithTrace(ctx, logger.UpfLog).Debug("Session modification successful")

	return nil
}

// reapplyReferencingPDRs re-applies every PDR that matches, after mutate updates
// its embedded FAR/QER, registering an undo that restores the prior entry.
func (conn *SessionEngine) reapplyReferencingPDRs(session *Session, txn *sessionTxn, matches func(SPDRInfo) bool, mutate func(*SPDRInfo)) error {
	for _, spdrInfo := range session.ListPDRs() {
		if !matches(spdrInfo) {
			continue
		}

		old := spdrInfo
		mutate(&spdrInfo)
		session.PutPDR(spdrInfo.PdrID, spdrInfo)

		if err := applyPDR(spdrInfo, session, conn.BpfObjects); err != nil {
			return err
		}

		txn.onRollback(func() error { return applyPDR(old, session, conn.BpfObjects) })
	}

	return nil
}

// farInfoFromMerge merges a models.FAR into an existing ebpf.FarInfo.
func farInfoFromMerge(far models.FAR, localIPv4 netip.Addr, localIPv6 netip.Addr, existing ebpf.FarInfo) ebpf.FarInfo {
	existing.Action = encodeApplyAction(far.ApplyAction)

	if fp := far.ForwardingParameters; fp != nil {
		if ohc := fp.OuterHeaderCreation; ohc != nil {
			existing.OuterHeaderCreation = uint8(ohc.Description >> 8)
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
