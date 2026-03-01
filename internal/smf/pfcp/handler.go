// SPDX-FileCopyrightText: 2025-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package pfcp

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/amf/producer"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/dbwriter"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/pfcp_dispatcher"
	smfContext "github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
	"go.uber.org/zap"
)

type SmfPfcpHandler struct {
	// OnFlowReport is an optional callback invoked after each flow report is
	// persisted to the database. It is used to enqueue flows for Fleet sync.
	// It may be nil when Fleet sync is not configured.
	OnFlowReport func(req *pfcp_dispatcher.FlowReportRequest)
}

func (s SmfPfcpHandler) HandlePfcpSessionReportRequest(ctx context.Context, msg *message.SessionReportRequest) (*message.SessionReportResponse, error) {
	return HandlePfcpSessionReportRequest(ctx, msg)
}

func HandlePfcpSessionReportRequest(ctx context.Context, msg *message.SessionReportRequest) (*message.SessionReportResponse, error) {
	seid := msg.SEID()

	smf := smfContext.SMFSelf()

	smContext := smf.GetSMContextBySEID(seid)

	if smContext == nil || smContext.Supi == "" {
		return message.NewSessionReportResponse(
			1,
			0,
			seid,
			getSeqNumber(),
			0,
			ie.NewCause(ie.CauseRequestRejected),
		), fmt.Errorf("failed to find SMContext for seid %d", seid)
	}

	if msg.ReportType == nil {
		return message.NewSessionReportResponse(
			1,
			0,
			seid,
			getSeqNumber(),
			0,
			ie.NewCause(ie.CauseRequestRejected),
		), fmt.Errorf("report type IE is missing in PFCP Session Report Request")
	}

	// Downlink Data Report
	if msg.ReportType.HasDLDR() {
		n2Pdu, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(smContext.PolicyData, smContext.Tunnel.DataPath.UpLinkTunnel.TEID, smContext.Tunnel.DataPath.UpLinkTunnel.N3IP)
		if err != nil {
			return nil, fmt.Errorf("failed to build PDUSessionResourceSetupRequestTransfer: %v", err)
		}

		n1n2Request := models.N1N2MessageTransferRequest{
			PduSessionID:            smContext.PDUSessionID,
			SNssai:                  smContext.Snssai,
			BinaryDataN2Information: n2Pdu,
		}

		err = producer.N2MessageTransferOrPage(ctx, smContext.Supi, n1n2Request)
		if err != nil {
			return message.NewSessionReportResponse(
				1,
				0,
				seid,
				getSeqNumber(),
				0,
				ie.NewCause(ie.CauseRequestRejected),
			), fmt.Errorf("failed to send N1N2MessageTransfer to AMF: %v", err)
		}
	}

	// Usage Report
	if msg.ReportType.HasUSAR() {
		for _, urrReport := range msg.UsageReport {
			// Read Volume Measurement
			urrId, err := urrReport.URRID()
			if err != nil {
				return message.NewSessionReportResponse(
					1,
					0,
					seid,
					getSeqNumber(),
					0,
					ie.NewCause(ie.CauseRequestRejected),
				), fmt.Errorf("failed to get URR ID from Usage Report IE: %v", err)
			}

			volumeMeasurement, err := urrReport.VolumeMeasurement()
			if err != nil {
				return message.NewSessionReportResponse(
					1,
					0,
					seid,
					getSeqNumber(),
					0,
					ie.NewCause(ie.CauseRequestRejected),
				), fmt.Errorf("failed to get Volume Measurement from Usage Report IE: %v", err)
			}

			dailyUsage := db.DailyUsage{
				IMSI:          smContext.Supi,
				BytesUplink:   int64(volumeMeasurement.UplinkVolume),
				BytesDownlink: int64(volumeMeasurement.DownlinkVolume),
			}
			dailyUsage.SetDay(time.Now().UTC())

			err = smf.DBInstance.IncrementDailyUsage(ctx, dailyUsage)
			if err != nil {
				return message.NewSessionReportResponse(
					1,
					0,
					seid,
					getSeqNumber(),
					0,
					ie.NewCause(ie.CauseRequestRejected),
				), fmt.Errorf("failed to update uplink data volume in db for imsi %s: %v", smContext.Supi, err)
			}

			logger.WithTrace(ctx, logger.SmfLog).Debug(
				"Processed usage report",
				zap.String("supi", smContext.Supi),
				zap.Uint32("urrID", urrId),
				zap.Uint64("uplink_volume", volumeMeasurement.UplinkVolume),
				zap.Uint64("downlink_volume", volumeMeasurement.DownlinkVolume),
			)
		}
	}

	return message.NewSessionReportResponse(
		1,
		0,
		seid,
		getSeqNumber(),
		0,
		ie.NewCause(ie.CauseRequestAccepted),
	), nil
}

// SendFlowReport receives flow statistics from the UPF and persists them to the database.
// It uses the IMSI provided in the request to store the flow report.
func (s SmfPfcpHandler) SendFlowReport(ctx context.Context, req *pfcp_dispatcher.FlowReportRequest) error {
	if req == nil {
		return fmt.Errorf("flow report request is nil")
	}

	if req.IMSI == "" {
		return fmt.Errorf("flow report request missing required IMSI field")
	}

	smf := smfContext.SMFSelf()

	// Construct the database flow report directly
	dbFlowReport := &dbwriter.FlowReport{
		SubscriberID:    req.IMSI,
		SourceIP:        req.SourceIP,
		DestinationIP:   req.DestinationIP,
		SourcePort:      req.SourcePort,
		DestinationPort: req.DestinationPort,
		Protocol:        req.Protocol,
		Packets:         req.Packets,
		Bytes:           req.Bytes,
		StartTime:       req.StartTime,
		EndTime:         req.EndTime,
	}

	// Persist flow report to database
	err := smf.DBInstance.InsertFlowReport(ctx, dbFlowReport)
	if err != nil {
		logger.SmfLog.Error(
			"Failed to insert flow report",
			zap.String("subscriber_id", req.IMSI),
			zap.String("source_ip", req.SourceIP),
			zap.String("destination_ip", req.DestinationIP),
			zap.Error(err),
		)

		return err
	}

	logger.SmfLog.Debug(
		"Flow report persisted",
		zap.String("subscriber_id", req.IMSI),
		zap.String("source_ip", req.SourceIP),
		zap.String("destination_ip", req.DestinationIP),
		zap.Uint64("packets", req.Packets),
		zap.Uint64("bytes", req.Bytes),
	)

	if s.OnFlowReport != nil {
		s.OnFlowReport(req)
	}

	return nil
}
