// SPDX-FileCopyrightText: 2026-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"encoding/binary"
	"fmt"
	"maps"

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

	bpfObjects := conn.BpfObjects
	localIPv4 := conn.n3Address.To4()

	if localIPv4 == nil {
		return fmt.Errorf("N3 address is not IPv4")
	}

	localIP := binary.LittleEndian.Uint32(localIPv4)

	pdrContext := NewPDRCreationContext(session, conn.FteIDResourceManager)

	// --- FARs ---

	for _, far := range req.CreateFARs {
		farInfo := farInfoFromModel(far, localIP)

		go addRemoteIPToNeigh(ctx, farInfo.RemoteIP)

		session.NewFar(far.FARID, farInfo)

		logger.WithTrace(ctx, logger.UpfLog).Info("Created Forwarding Action Rule",
			logger.FARID(far.FARID), zap.Any("farInfo", farInfo))
	}

	for _, far := range req.UpdateFARs {
		sFarInfo := session.GetFar(far.FARID)
		sFarInfo = farInfoFromMerge(far, localIP, sFarInfo)

		go addRemoteIPToNeigh(ctx, sFarInfo.RemoteIP)

		session.UpdateFar(far.FARID, sFarInfo)

		// Re-apply all PDRs that reference this FAR with the updated embedded FAR.
		for _, spdrInfo := range session.ListPDRs() {
			if spdrInfo.PdrInfo.FarID == far.FARID {
				spdrInfo.PdrInfo.Far = sFarInfo
				session.PutPDR(spdrInfo.PdrID, spdrInfo)

				if err := applyPDR(spdrInfo, bpfObjects); err != nil {
					span.RecordError(err)
					return fmt.Errorf("can't update PDR after FAR update: %w", err)
				}
			}
		}

		logger.WithTrace(ctx, logger.UpfLog).Info("Updated Forwarding Action Rule",
			logger.FARID(far.FARID), zap.Any("farInfo", sFarInfo))
	}

	for _, farID := range req.RemoveFARIDs {
		session.RemoveFar(farID)

		logger.WithTrace(ctx, logger.UpfLog).Debug("Removed Forwarding Action Rule",
			logger.FARID(farID))
	}

	// --- QERs ---

	for _, qer := range req.CreateQERs {
		qerInfo := qerInfoFromModel(qer)

		session.NewQer(qer.QERID, qerInfo)

		// Re-apply all PDRs that reference this QER with the updated embedded QER.
		for _, spdrInfo := range session.ListPDRs() {
			if spdrInfo.PdrInfo.QerID == qer.QERID {
				spdrInfo.PdrInfo.Qer = qerInfo
				session.PutPDR(spdrInfo.PdrID, spdrInfo)

				if err := applyPDR(spdrInfo, bpfObjects); err != nil {
					span.RecordError(err)
					return fmt.Errorf("can't apply PDR for new QER: %w", err)
				}
			}
		}

		logger.WithTrace(ctx, logger.UpfLog).Info("Created QoS Enforcement Rule",
			logger.QERID(qer.QERID), zap.Any("qerInfo", qerInfo))
	}

	// --- PDRs ---

	// Build FAR and QER maps from session for PDR injection.
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
			span.RecordError(err)
			return fmt.Errorf("couldn't extract PDR info: %w", err)
		}

		if idx, ok := req.FilterIndexByPDRID[pdr.PDRID]; ok {
			spdrInfo.PdrInfo.FilterMapIndex = idx
		}

		session.PutPDR(spdrInfo.PdrID, spdrInfo)

		if err := applyPDR(spdrInfo, bpfObjects); err != nil {
			if spdrInfo.Allocated {
				pdrContext.FteIDResourceManager.ReleaseTEID(pdrContext.Session.SEID)
			}

			span.RecordError(err)

			return fmt.Errorf("couldn't apply PDR: %w", err)
		}

		bpfObjects.ClearNotified(req.SEID, pdr.PDRID, spdrInfo.PdrInfo.Qer.Qfi)
	}

	for _, pdr := range req.UpdatePDRs {
		spdrInfo := session.GetPDR(pdr.PDRID)

		if err := pdrContext.ExtractPDR(pdr, &spdrInfo, farMap, qerMap); err != nil {
			span.RecordError(err)
			return fmt.Errorf("couldn't extract PDR info: %w", err)
		}

		session.PutPDR(uint32(pdr.PDRID), spdrInfo)

		if err := applyPDR(spdrInfo, bpfObjects); err != nil {
			if spdrInfo.Allocated {
				pdrContext.FteIDResourceManager.ReleaseTEID(pdrContext.Session.SEID)
			}

			span.RecordError(err)

			return fmt.Errorf("couldn't apply PDR: %w", err)
		}

		bpfObjects.ClearNotified(req.SEID, pdr.PDRID, spdrInfo.PdrInfo.Qer.Qfi)
	}

	for _, pdrID := range req.RemovePDRIDs {
		if session.HasPDR(uint32(pdrID)) {
			sPDRInfo := session.RemovePDR(uint32(pdrID))

			if err := pdrContext.deletePDR(sPDRInfo, bpfObjects); err != nil {
				span.RecordError(err)
				return fmt.Errorf("couldn't delete PDR: %w", err)
			}
		}
	}

	logger.WithTrace(ctx, logger.UpfLog).Debug("Session modification successful")

	if req.PolicyID != 0 && req.PolicyID != session.PolicyID() {
		oldPolicyID := session.PolicyID()
		session.SetPolicyID(req.PolicyID)

		conn.mu.Lock()
		conn.deregisterPolicy(oldPolicyID, session.SEID)
		conn.registerPolicy(req.PolicyID, session.SEID)
		conn.mu.Unlock()
	}

	conn.AddSession(session.SEID, session)

	return nil
}

// farInfoFromMerge merges a models.FAR into an existing ebpf.FarInfo.
func farInfoFromMerge(far models.FAR, localIP uint32, existing ebpf.FarInfo) ebpf.FarInfo {
	existing.LocalIP = localIP
	existing.Action = encodeApplyAction(far.ApplyAction)

	if fp := far.ForwardingParameters; fp != nil {
		if ohc := fp.OuterHeaderCreation; ohc != nil {
			existing.OuterHeaderCreation = uint8(ohc.Description >> 8)
			existing.TeID = ohc.TEID

			if ohc.IPv4Address != nil {
				ip4 := ohc.IPv4Address.To4()
				if ip4 != nil {
					existing.RemoteIP = binary.LittleEndian.Uint32(ip4)
				}
			}
		}
	}

	return existing
}
