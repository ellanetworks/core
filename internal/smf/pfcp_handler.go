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
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/ellanetworks/core/internal/tracing/attrs"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func (s *SMF) HandleDownlinkDataReport(ctx context.Context, report *models.DownlinkDataReport) error {
	ctx, span := tracer.Start(ctx, "smf/handle_downlink_data_report")
	defer span.End()

	smContext := s.GetSessionBySEID(report.SEID)
	if smContext == nil || !smContext.Supi.IsIMSI() {
		return fmt.Errorf("failed to find SMContext for seid %d", report.SEID)
	}

	return s.notifyDownlinkWaiting(ctx, smContext, models.DownlinkDataArrived)
}

func (s *SMF) notifyDownlinkWaiting(ctx context.Context, smContext *SMContext, cause models.DownlinkDataNotificationCause) error {
	smContext.Mutex.Lock()

	onEPS := smContext.Access == Access4G
	policy, tunnel := smContext.PolicyData, smContext.Tunnel
	pduSessionType, supi, pduSessionID, snssai := smContext.PDUSessionType, smContext.Supi, smContext.PDUSessionID, smContext.Snssai
	ebi := smContext.EBI

	var seid uint64
	if smContext.PFCPContext != nil {
		seid = smContext.PFCPContext.SEID
	}

	smContext.Mutex.Unlock()

	// A 4G EPS session is paged via the MME (TS 23.401).
	if onEPS {
		if s.mme == nil {
			return fmt.Errorf("no MME registered to page EPS UE %s", supi.IMSI())
		}

		return s.mme.NotifyDownlinkData(ctx, supi.IMSI(), ebi, cause)
	}

	if policy == nil || tunnel == nil {
		return fmt.Errorf("session for seid %d has no user plane to page for", seid)
	}

	n2Pdu, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(&policy.Ambr, &policy.QosData, tunnel.N3TEID, tunnel.N3IPv4, tunnel.N3IPv6, nasToNgapPDUSessionType(pduSessionType))
	if err != nil {
		return fmt.Errorf("failed to build PDUSessionResourceSetupRequestTransfer: %v", err)
	}

	transferCause, err := s.amf.N2TransferOrPage(ctx, supi, pduSessionID, snssai, n2Pdu, policy.QosData.Arp)
	if err != nil {
		return fmt.Errorf("failed to send N1N2MessageTransfer to AMF: %v", err)
	}

	logger.SmfLog.Debug("N1N2 message transfer accepted",
		logger.SUPI(supi.String()),
		logger.PDUSessionID(pduSessionID),
		logger.Cause(transferCause.String()))

	return nil
}

func (s *SMF) SendFlowReports(ctx context.Context, reqs []*models.FlowReportRequest) error {
	ctx, span := tracer.Start(ctx, "smf/send_flow_reports",
		trace.WithAttributes(attribute.Int("pfcp.report_batch_size", len(reqs))),
	)
	defer span.End()

	filtered := make([]*models.FlowReportRequest, 0, len(reqs))

	for _, req := range reqs {
		if req == nil || req.IMSI == "" {
			continue
		}

		filtered = append(filtered, req)
	}

	if len(filtered) == 0 {
		return nil
	}

	if err := s.store.InsertFlowReports(ctx, filtered); err != nil {
		logger.SmfLog.Error("Failed to insert flow report batch",
			zap.Int("batch_size", len(filtered)),
			zap.Error(err),
		)

		return err
	}

	logger.SmfLog.Debug("Flow report batch persisted",
		zap.Int("count", len(filtered)),
	)

	return nil
}

func (s *SMF) HandleErrorIndicationReport(ctx context.Context, report *models.ErrorIndicationReport) error {
	ctx, span := tracer.Start(ctx, "smf/handle_error_indication_report")
	defer span.End()

	span.SetAttributes(
		attrs.SEID(report.SEID),
		attribute.Int64("pfcp.far_id", int64(report.FARID)),
	)

	smContext := s.GetSessionBySEID(report.SEID)
	if smContext == nil || !smContext.Supi.IsIMSI() {
		return fmt.Errorf("failed to find SMContext for seid %d", report.SEID)
	}

	switch report.FARID {
	case farIDDownlink:
		return s.releaseBrokenAccessTunnel(ctx, smContext, report)
	case farIDForwarding:
		return s.releaseBrokenForwardingTunnel(ctx, smContext, report)
	default:
		logger.From(ctx, logger.SmfLog).Info(
			"Ignoring a GTP-U Error Indication for a tunnel that carries no traffic of its own",
			logger.SUPI(smContext.Supi.String()), logger.SEID(report.SEID), logger.FARID(report.FARID),
			logger.TEID(report.RemoteFTEID.TEID))

		return nil
	}
}

func (s *SMF) releaseBrokenAccessTunnel(ctx context.Context, smContext *SMContext, report *models.ErrorIndicationReport) error {
	smContext.Mutex.Lock()

	if smContext.Tunnel == nil || smContext.PFCPContext == nil {
		smContext.Mutex.Unlock()

		return fmt.Errorf("session for seid %d has no user plane to release", report.SEID)
	}

	if !smContext.upConnectionActive() || !reportNamesAnchor(report, smContext.Tunnel.AN) {
		logger.From(ctx, logger.SmfLog).Debug(
			"Ignoring a GTP-U Error Indication for a tunnel the session no longer forwards into",
			logger.SUPI(smContext.Supi.String()), logger.SEID(report.SEID),
			logger.TEID(report.RemoteFTEID.TEID))
		smContext.Mutex.Unlock()

		return nil
	}

	access := smContext.Access
	supi := smContext.Supi

	smContext.Mutex.Unlock()

	logger.From(ctx, logger.SmfLog).Warn(
		"Access network reported a GTP-U Error Indication; buffering the downlink and re-establishing the tunnel",
		logger.SUPI(supi.String()),
		logger.SEID(report.SEID), logger.FARID(report.FARID),
		zap.String("gtpu_peer", report.RemoteFTEID.Addr.String()),
		logger.TEID(report.RemoteFTEID.TEID))

	affected := []*SMContext{smContext}
	if access == Access4G {
		affected = s.epsSessionsOf(supi)
	}

	var reportedErr error

	for _, sc := range affected {
		err := s.bufferDownlinkAfterErrorIndication(ctx, sc, access)
		if err == nil {
			continue
		}

		if sc == smContext {
			reportedErr = err

			continue
		}

		logger.From(ctx, logger.SmfLog).Warn(
			"could not stop the downlink of another PDN connection of the UE after an Error Indication",
			zap.Error(err), logger.SUPI(supi.String()), logger.SEID(report.SEID))
	}

	if reportedErr != nil {
		return reportedErr
	}

	if access != Access4G && s.releaseAccessResources(ctx, smContext) {
		return nil
	}

	return s.notifyDownlinkWaiting(ctx, smContext, models.DownlinkDataErrorIndication)
}

func (s *SMF) releaseBrokenForwardingTunnel(ctx context.Context, smContext *SMContext, report *models.ErrorIndicationReport) error {
	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if smContext.Tunnel == nil || smContext.Tunnel.Forwarding == nil {
		return nil
	}

	if !reportNamesAnchor(report, *smContext.Tunnel.Forwarding) {
		logger.From(ctx, logger.SmfLog).Debug(
			"Ignoring a GTP-U Error Indication for a forwarding tunnel the session no longer relays into",
			logger.SUPI(smContext.Supi.String()), logger.SEID(report.SEID),
			logger.TEID(report.RemoteFTEID.TEID))

		return nil
	}

	smContext.forwardingRelease.Stop()

	logger.From(ctx, logger.SmfLog).Info(
		"Handover target reported a GTP-U Error Indication; releasing the indirect data forwarding tunnel early",
		logger.SUPI(smContext.Supi.String()),
		logger.SEID(report.SEID),
		zap.String("gtpu_peer", report.RemoteFTEID.Addr.String()),
		logger.TEID(report.RemoteFTEID.TEID))

	return s.closeForwardingTunnel(ctx, smContext)
}

func (s *SMF) bufferDownlinkAfterErrorIndication(ctx context.Context, sc *SMContext, access AccessType) error {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if sc.Access != access || sc.Tunnel == nil || sc.PFCPContext == nil || !sc.upConnectionActive() {
		return nil
	}

	seid := sc.PFCPContext.SEID

	next := sc.Tunnel.dataPlane
	next.Downlink = DownlinkBuffering
	next.AN = AnchorBinding{}

	if err := s.applyDataPlane(ctx, sc, next, ""); err != nil {
		return fmt.Errorf("buffer the downlink of session %d after an Error Indication: %w", seid, err)
	}

	return nil
}

func (s *SMF) releaseAccessResources(ctx context.Context, smContext *SMContext) bool {
	smContext.Mutex.Lock()
	supi, pduSessionID := smContext.Supi, smContext.PDUSessionID
	smContext.Mutex.Unlock()

	n2Transfer, err := ngap.BuildPDUSessionResourceReleaseCommandTransfer()
	if err != nil {
		logger.From(ctx, logger.SmfLog).Warn("could not build the PDU Session Resource Release Command transfer",
			zap.Error(err), logger.SUPI(supi.String()), logger.PDUSessionID(pduSessionID))

		return false
	}

	smContext.Mutex.Lock()
	smContext.recordN2Release(n2ReleaseUPConnection)
	smContext.Mutex.Unlock()

	if err := s.amf.ReleaseAccessResources(ctx, supi, pduSessionID, n2Transfer); err != nil {
		smContext.Mutex.Lock()
		smContext.recordN2Release(n2ReleaseSession)
		smContext.Mutex.Unlock()

		if !errors.Is(err, ErrUENotReachable) {
			logger.From(ctx, logger.SmfLog).Warn("could not release the access resources of a broken tunnel",
				zap.Error(err), logger.SUPI(supi.String()), logger.PDUSessionID(pduSessionID))
		}

		return false
	}

	return true
}

func (s *SMF) epsSessionsOf(supi etsi.SUPI) []*SMContext {
	s.mu.RLock()

	var sessions []*SMContext

	for _, sc := range s.pool {
		if sc.Supi == supi {
			sessions = append(sessions, sc)
		}
	}

	s.mu.RUnlock()

	var out []*SMContext

	for _, sc := range sessions {
		if sc.onEPS() {
			out = append(out, sc)
		}
	}

	return out
}

func reportNamesAnchor(report *models.ErrorIndicationReport, an AnchorBinding) bool {
	if report.RemoteFTEID.TEID != an.TEID || !report.RemoteFTEID.Addr.IsValid() {
		return false
	}

	peer := report.RemoteFTEID.Addr

	if peer.Is4() {
		return an.IPv4 != nil && an.IPv4.Equal(net.IP(peer.AsSlice()))
	}

	return an.IPv6 != nil && an.IPv6.Equal(net.IP(peer.AsSlice()))
}

func (s *SMF) HandleUsageReports(ctx context.Context, reports []*models.UsageReport) error {
	ctx, span := tracer.Start(ctx, "smf/handle_usage_reports")
	defer span.End()

	usages := make([]models.SubscriberUsage, 0, len(reports))

	for _, report := range reports {
		smContext := s.GetSessionBySEID(report.SEID)
		if smContext == nil || !smContext.Supi.IsIMSI() {
			logger.From(ctx, logger.SmfLog).Error(
				"usage bytes lost: the SEID no longer resolves to a subscriber",
				logger.SEID(report.SEID),
				logger.UplinkVolume(report.UplinkVolume),
				logger.DownlinkVolume(report.DownlinkVolume),
			)

			continue
		}

		usages = append(usages, models.SubscriberUsage{
			IMSI:           smContext.Supi.IMSI(),
			UplinkVolume:   report.UplinkVolume,
			DownlinkVolume: report.DownlinkVolume,
		})
	}

	if len(usages) == 0 {
		return nil
	}

	if err := s.store.IncrementDailyUsageBatch(ctx, usages); err != nil {
		return fmt.Errorf("failed to update data volume for %d subscribers: %w", len(usages), err)
	}

	logger.From(ctx, logger.SmfLog).Debug(
		"Processed usage reports",
		zap.Int("subscribers", len(usages)),
	)

	return nil
}
