// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/ngap"
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

	smContext.Mutex.Lock()

	onEPS := smContext.Access == Access4G
	policy, tunnel := smContext.PolicyData, smContext.Tunnel
	pduSessionType, supi, pduSessionID, snssai := smContext.PDUSessionType, smContext.Supi, smContext.PDUSessionID, smContext.Snssai
	ebi := smContext.EBI

	smContext.Mutex.Unlock()

	// A 4G EPS session is paged via the MME (TS 23.401).
	if onEPS {
		if s.mme == nil {
			return fmt.Errorf("no MME registered to page EPS UE %s", supi.IMSI())
		}

		return s.mme.Page(ctx, supi.IMSI(), ebi, epsArp(policy))
	}

	if policy == nil || tunnel == nil {
		return fmt.Errorf("session for seid %d has no user plane to page for", report.SEID)
	}

	n2Pdu, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(&policy.Ambr, &policy.QosData, tunnel.N3TEID, tunnel.N3IPv4, tunnel.N3IPv6, nasToNgapPDUSessionType(pduSessionType))
	if err != nil {
		return fmt.Errorf("failed to build PDUSessionResourceSetupRequestTransfer: %v", err)
	}

	cause, err := s.amf.N2TransferOrPage(ctx, supi, pduSessionID, snssai, n2Pdu, policy.QosData.Arp, policy.QosData.Var5qi)
	if err != nil {
		return fmt.Errorf("failed to send N1N2MessageTransfer to AMF: %v", err)
	}

	logger.SmfLog.Debug("N1N2 message transfer accepted",
		zap.String("supi", supi.String()),
		zap.Uint8("pdu_session_id", pduSessionID),
		zap.String("cause", cause.String()))

	return nil
}

func epsArp(policy *Policy) *models.Arp {
	if policy == nil {
		return nil
	}

	return policy.QosData.Arp
}

func (s *SMF) SendFlowReports(ctx context.Context, reqs []*models.FlowReportRequest) error {
	ctx, span := tracer.Start(ctx, "smf/send_flow_reports",
		trace.WithAttributes(attribute.Int("batch_size", len(reqs))),
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

func (s *SMF) HandleUsageReports(ctx context.Context, reports []*models.UsageReport) error {
	ctx, span := tracer.Start(ctx, "smf/handle_usage_reports")
	defer span.End()

	usages := make([]models.SubscriberUsage, 0, len(reports))

	for _, report := range reports {
		smContext := s.GetSessionBySEID(report.SEID)
		if smContext == nil || !smContext.Supi.IsIMSI() {
			logger.WithTrace(ctx, logger.SmfLog).Error(
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

	logger.WithTrace(ctx, logger.SmfLog).Debug(
		"Processed usage reports",
		zap.Int("subscribers", len(usages)),
	)

	return nil
}
