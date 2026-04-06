// SPDX-FileCopyrightText: 2026-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package smf

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pfcp_dispatcher"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var reportSeq uint32

func getReportSeqNumber() uint32 {
	return atomic.AddUint32(&reportSeq, 1)
}

// HandlePfcpSessionReportRequest processes a PFCP Session Report from the UPF.
// It handles downlink data reports (triggering paging) and usage reports (persisting usage).
func (s *SMF) HandlePfcpSessionReportRequest(ctx context.Context, msg *message.SessionReportRequest) (*message.SessionReportResponse, error) {
	ctx, span := tracer.Start(ctx, "smf/handle_pfcp_session_report_request")
	defer span.End()

	seid := msg.SEID()

	smContext := s.GetSessionBySEID(seid)

	if smContext == nil || !smContext.Supi.IsValid() {
		return message.NewSessionReportResponse(
			1, 0, seid, getReportSeqNumber(), 0,
			ie.NewCause(ie.CauseRequestRejected),
		), fmt.Errorf("failed to find SMContext for seid %d", seid)
	}

	if msg.ReportType == nil {
		return message.NewSessionReportResponse(
			1, 0, seid, getReportSeqNumber(), 0,
			ie.NewCause(ie.CauseRequestRejected),
		), fmt.Errorf("report type IE is missing in PFCP Session Report Request")
	}

	// Downlink Data Report — page the UE via AMF
	if msg.ReportType.HasDLDR() {
		n2Pdu, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(&smContext.PolicyData.Ambr, &smContext.PolicyData.QosData, smContext.Tunnel.DataPath.UpLinkTunnel.TEID, smContext.Tunnel.DataPath.UpLinkTunnel.N3IP)
		if err != nil {
			return nil, fmt.Errorf("failed to build PDUSessionResourceSetupRequestTransfer: %v", err)
		}

		if err := s.amf.N2TransferOrPage(ctx, smContext.Supi, smContext.PDUSessionID, smContext.Snssai, n2Pdu); err != nil {
			return message.NewSessionReportResponse(
				1, 0, seid, getReportSeqNumber(), 0,
				ie.NewCause(ie.CauseRequestRejected),
			), fmt.Errorf("failed to send N1N2MessageTransfer to AMF: %v", err)
		}
	}

	// Usage Report — persist usage counters
	if msg.ReportType.HasUSAR() {
		for _, urrReport := range msg.UsageReport {
			urrID, err := urrReport.URRID()
			if err != nil {
				return message.NewSessionReportResponse(
					1, 0, seid, getReportSeqNumber(), 0,
					ie.NewCause(ie.CauseRequestRejected),
				), fmt.Errorf("failed to get URR ID from Usage Report IE: %v", err)
			}

			volumeMeasurement, err := urrReport.VolumeMeasurement()
			if err != nil {
				return message.NewSessionReportResponse(
					1, 0, seid, getReportSeqNumber(), 0,
					ie.NewCause(ie.CauseRequestRejected),
				), fmt.Errorf("failed to get Volume Measurement from Usage Report IE: %v", err)
			}

			if err := s.store.IncrementDailyUsage(ctx, smContext.Supi.IMSI(), volumeMeasurement.UplinkVolume, volumeMeasurement.DownlinkVolume); err != nil {
				return message.NewSessionReportResponse(
					1, 0, seid, getReportSeqNumber(), 0,
					ie.NewCause(ie.CauseRequestRejected),
				), fmt.Errorf("failed to update data volume for imsi %s: %v", smContext.Supi.String(), err)
			}

			logger.WithTrace(ctx, logger.SmfLog).Debug(
				"Processed usage report",
				logger.SUPI(smContext.Supi.String()),
				logger.URRID(urrID),
				logger.UplinkVolume(volumeMeasurement.UplinkVolume),
				logger.DownlinkVolume(volumeMeasurement.DownlinkVolume),
			)
		}
	}

	return message.NewSessionReportResponse(
		1, 0, seid, getReportSeqNumber(), 0,
		ie.NewCause(ie.CauseRequestAccepted),
	), nil
}

// SendFlowReports persists a batch of flow measurement records from the UPF
// in a single database transaction.
func (s *SMF) SendFlowReports(ctx context.Context, reqs []*pfcp_dispatcher.FlowReportRequest) error {
	ctx, span := tracer.Start(ctx, "smf/send_flow_reports",
		trace.WithAttributes(attribute.Int("batch_size", len(reqs))),
	)
	defer span.End()

	reports := make([]*FlowReport, 0, len(reqs))

	for _, req := range reqs {
		if req == nil || req.IMSI == "" {
			continue
		}

		reports = append(reports, &FlowReport{
			IMSI:            req.IMSI,
			SourceIP:        req.SourceIP,
			DestinationIP:   req.DestinationIP,
			SourcePort:      req.SourcePort,
			DestinationPort: req.DestinationPort,
			Protocol:        req.Protocol,
			Packets:         req.Packets,
			Bytes:           req.Bytes,
			StartTime:       req.StartTime,
			EndTime:         req.EndTime,
			Direction:       req.Direction,
			Action:          req.Action,
		})
	}

	if len(reports) == 0 {
		return nil
	}

	if err := s.store.InsertFlowReports(ctx, reports); err != nil {
		logger.SmfLog.Error("Failed to insert flow report batch",
			zap.Int("batch_size", len(reports)),
			zap.Error(err),
		)

		return err
	}

	logger.SmfLog.Debug("Flow report batch persisted",
		zap.Int("count", len(reports)),
	)

	return nil
}

// IncrementDailyUsage delegates daily usage accounting to the store.
func (s *SMF) IncrementDailyUsage(ctx context.Context, imsi string, uplinkBytes, downlinkBytes uint64) error {
	return s.store.IncrementDailyUsage(ctx, imsi, uplinkBytes, downlinkBytes)
}
