// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"
	"net/netip"

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
	if smContext == nil || !smContext.Supi.IsValid() {
		return fmt.Errorf("failed to find SMContext for seid %d", report.SEID)
	}

	// A transfer rewrites the access, the policy and the tunnel together, so the
	// paging decision and the N2 content come from one critical section.
	smContext.Mutex.Lock()

	isEPS := smContext.IsEPS()
	hasUplink := smContext.Tunnel != nil && smContext.Tunnel.DataPath != nil && smContext.Tunnel.DataPath.UpLinkTunnel != nil
	policy := smContext.PolicyData
	pduSessionID := smContext.PDUSessionID
	snssai := smContext.Snssai
	ngapPDUType := nasToNgapPDUSessionType(smContext.PDUSessionType)

	var ulTEID uint32

	var ulN3IPv4, ulN3IPv6 netip.Addr

	if hasUplink {
		ul := smContext.Tunnel.DataPath.UpLinkTunnel
		ulTEID, ulN3IPv4, ulN3IPv6 = ul.TEID, ul.N3IPv4, ul.N3IPv6
	}

	smContext.Mutex.Unlock()

	// A 4G EPS session is paged via the MME (TS 23.401).
	if isEPS {
		if s.mme == nil {
			return fmt.Errorf("no MME registered to page EPS UE %s", smContext.Supi.IMSI())
		}

		return s.mme.Page(ctx, smContext.Supi.IMSI())
	}

	if !hasUplink || policy == nil {
		return fmt.Errorf("session %q has no uplink tunnel or policy to page for", smContext.Ref)
	}

	n2Pdu, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(&policy.Ambr, &policy.QosData, ulTEID, ulN3IPv4, ulN3IPv6, ngapPDUType)
	if err != nil {
		return fmt.Errorf("failed to build PDUSessionResourceSetupRequestTransfer: %v", err)
	}

	if err := s.amf.N2TransferOrPage(ctx, smContext.Supi, pduSessionID, snssai, n2Pdu); err != nil {
		return fmt.Errorf("failed to send N1N2MessageTransfer to AMF: %v", err)
	}

	return nil
}

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

func (s *SMF) IncrementDailyUsage(ctx context.Context, imsi string, uplinkBytes, downlinkBytes uint64) error {
	return s.store.IncrementDailyUsage(ctx, imsi, uplinkBytes, downlinkBytes)
}
