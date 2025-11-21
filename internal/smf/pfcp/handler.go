// SPDX-FileCopyrightText: 2025-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package pfcp

import (
	ctxt "context"
	"fmt"

	amf_producer "github.com/ellanetworks/core/internal/amf/producer"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

type SmfPfcpHandler struct{}

func (s SmfPfcpHandler) HandlePfcpSessionReportRequest(ctx ctxt.Context, msg *message.SessionReportRequest) (*message.SessionReportResponse, error) {
	return HandlePfcpSessionReportRequest(ctx, msg)
}

func HandlePfcpSessionReportRequest(ctx ctxt.Context, msg *message.SessionReportRequest) (*message.SessionReportResponse, error) {
	ies := make([]*ie.IE, 0)

	seid := msg.SEID()
	smContext := context.GetSMContextBySEID(seid)

	if smContext == nil {
		ies = append(ies, ie.NewCause(ie.CauseRequestRejected))
		return message.NewSessionReportResponse(
			1,
			0,
			seid,
			getSeqNumber(),
			0,
			ies...,
		), fmt.Errorf("failed to find SMContext for seid %d", seid)
	}

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

	// N1 Container Info
	n1MsgContainer := models.N1MessageContainer{
		N1MessageClass:   "SM",
		N1MessageContent: &models.RefToBinaryData{ContentID: "GSM_NAS"},
	}

	// N1N2 Json Data
	n1n2Request.JSONData = &models.N1N2MessageTransferReqData{
		PduSessionID:       smContext.PDUSessionID,
		N1MessageContainer: &n1MsgContainer,
		N2InfoContainer:    &n2InfoContainer,
	}

	rsp, err := amf_producer.CreateN1N2MessageTransfer(ctx, smContext.Supi, n1n2Request)
	if err != nil || rsp.Cause == models.N1N2MessageTransferCauseN1MsgNotTransferred {
		ies = append(ies, ie.NewCause(ie.CauseRequestRejected))
		return message.NewSessionReportResponse(
			1,
			0,
			seid,
			getSeqNumber(),
			0,
			ies...,
		), fmt.Errorf("failed to send N1N2MessageTransfer to AMF: %v", err)
	}

	ies = append(ies, ie.NewCause(ie.CauseRequestAccepted))
	return message.NewSessionReportResponse(
		1,
		0,
		seid,
		getSeqNumber(),
		0,
		ies...,
	), nil
}
