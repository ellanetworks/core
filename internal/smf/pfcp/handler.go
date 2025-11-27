// SPDX-FileCopyrightText: 2025-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package pfcp

import (
	ctxt "context"
	"fmt"
	"time"

	amf_producer "github.com/ellanetworks/core/internal/amf/producer"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
	"go.uber.org/zap"
)

type SmfPfcpHandler struct{}

func (s SmfPfcpHandler) HandlePfcpSessionReportRequest(ctx ctxt.Context, msg *message.SessionReportRequest) (*message.SessionReportResponse, error) {
	return HandlePfcpSessionReportRequest(ctx, msg)
}

func HandlePfcpSessionReportRequest(ctx ctxt.Context, msg *message.SessionReportRequest) (*message.SessionReportResponse, error) {
	seid := msg.SEID()

	smContext := context.GetSMContextBySEID(seid)

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
		n1n2Request := models.N1N2MessageTransferRequest{}
		// N2 Container Info
		n2InfoContainer := models.N2InfoContainer{
			N2InformationClass: models.N2InformationClassSM,
			SmInfo: &models.N2SmInformation{
				PduSessionID: smContext.PDUSessionID,
				N2InfoContent: &models.N2InfoContent{
					NgapIeType: models.NgapIeTypePduResSetupReq,
					NgapData: &models.RefToBinaryData{
						ContentID: "N2SmInformation",
					},
				},
				SNssai: smContext.Snssai,
			},
		}

		// N1N2 Json Data
		n1n2Request.JSONData = &models.N1N2MessageTransferReqData{
			PduSessionID:    smContext.PDUSessionID,
			N2InfoContainer: &n2InfoContainer,
		}

		if n2Pdu, err := context.BuildPDUSessionResourceSetupRequestTransfer(smContext); err != nil {
			logger.SmfLog.Error("Build PDUSessionResourceSetupRequestTransfer failed", zap.Error(err))
		} else {
			n1n2Request.BinaryDataN2Information = n2Pdu
			n1n2Request.JSONData.N2InfoContainer = &n2InfoContainer
		}

		rsp, err := amf_producer.CreateN1N2MessageTransfer(ctx, smContext.Supi, n1n2Request)
		if err != nil || rsp.Cause == models.N1N2MessageTransferCauseN1MsgNotTransferred {
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
		smfSelf := context.SMFSelf()

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

			err = smfSelf.DBInstance.IncrementDailyUsage(ctx, dailyUsage)
			if err != nil {
				return message.NewSessionReportResponse(
					1,
					0,
					seid,
					getSeqNumber(),
					0,
					ie.NewCause(ie.CauseRequestRejected),
				), fmt.Errorf("failed to update uplink data volume in db: %v", err)
			}
			logger.SmfLog.Debug(
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
