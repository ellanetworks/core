// SPDX-FileCopyrightText: 2026-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

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

// HandleDownlinkDataReport processes a downlink data notification from the UPF,
// triggering paging via the AMF so the UE re-establishes its radio bearer.
func (s *SMF) HandleDownlinkDataReport(ctx context.Context, report *models.DownlinkDataReport) error {
	ctx, span := tracer.Start(ctx, "smf/handle_downlink_data_report")
	defer span.End()

	smContext := s.GetSessionBySEID(report.SEID)
	if smContext == nil || !smContext.Supi.IsValid() {
		return fmt.Errorf("failed to find SMContext for seid %d", report.SEID)
	}

	n2Pdu, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(&smContext.PolicyData.Ambr, &smContext.PolicyData.QosData, smContext.Tunnel.DataPath.UpLinkTunnel.TEID, smContext.Tunnel.DataPath.UpLinkTunnel.N3IP)
	if err != nil {
		return fmt.Errorf("failed to build PDUSessionResourceSetupRequestTransfer: %v", err)
	}

	if err := s.amf.N2TransferOrPage(ctx, smContext.Supi, smContext.PDUSessionID, smContext.Snssai, n2Pdu); err != nil {
		return fmt.Errorf("failed to send N1N2MessageTransfer to AMF: %v", err)
	}

	return nil
}

// HandleUsageReport processes a usage report from the UPF,
// persisting the volume counters for the subscriber.
func (s *SMF) HandleUsageReport(ctx context.Context, report *models.UsageReport) error {
	ctx, span := tracer.Start(ctx, "smf/handle_usage_report")
	defer span.End()

	smContext := s.GetSessionBySEID(report.SEID)
	if smContext == nil || !smContext.Supi.IsValid() {
		return fmt.Errorf("failed to find SMContext for seid %d", report.SEID)
	}

	if err := s.store.IncrementDailyUsage(ctx, smContext.Supi.IMSI(), report.UplinkVolume, report.DownlinkVolume); err != nil {
		return fmt.Errorf("failed to update data volume for imsi %s: %v", smContext.Supi.String(), err)
	}

	logger.WithTrace(ctx, logger.SmfLog).Debug(
		"Processed usage report",
		logger.SUPI(smContext.Supi.String()),
		logger.UplinkVolume(report.UplinkVolume),
		logger.DownlinkVolume(report.DownlinkVolume),
	)

	return nil
}

// SendFlowReports persists a batch of flow measurement records from the UPF
// in a single database transaction.
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

// IncrementDailyUsage delegates daily usage accounting to the store.
func (s *SMF) IncrementDailyUsage(ctx context.Context, imsi string, uplinkBytes, downlinkBytes uint64) error {
	return s.store.IncrementDailyUsage(ctx, imsi, uplinkBytes, downlinkBytes)
}
