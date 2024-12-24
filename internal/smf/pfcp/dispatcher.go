package pfcp

import (
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/pfcp/handler"
	"github.com/ellanetworks/core/internal/smf/pfcp/udp"
	"github.com/wmnsk/go-pfcp/message"
)

func Dispatch(msg *udp.Message) {
	msgType := msg.PfcpMessage.MessageType()
	switch msgType {
	case message.MsgTypeHeartbeatRequest:
		handler.HandlePfcpHeartbeatRequest(msg)
	case message.MsgTypeHeartbeatResponse:
		handler.HandlePfcpHeartbeatResponse(msg)
	case message.MsgTypeAssociationSetupResponse:
		handler.HandlePfcpAssociationSetupResponse(msg)
	case message.MsgTypeAssociationUpdateRequest:
		handler.HandlePfcpAssociationUpdateRequest(msg)
	case message.MsgTypeAssociationUpdateResponse:
		handler.HandlePfcpAssociationUpdateResponse(msg)
	case message.MsgTypeAssociationReleaseRequest:
		handler.HandlePfcpAssociationReleaseRequest(msg)
	case message.MsgTypeAssociationReleaseResponse:
		handler.HandlePfcpAssociationReleaseResponse(msg)
	case message.MsgTypeVersionNotSupportedResponse:
		handler.HandlePfcpVersionNotSupportedResponse(msg)
	case message.MsgTypeNodeReportRequest:
		handler.HandlePfcpNodeReportRequest(msg)
	case message.MsgTypeNodeReportResponse:
		handler.HandlePfcpNodeReportResponse(msg)
	case message.MsgTypeSessionSetDeletionRequest:
		handler.HandlePfcpSessionSetDeletionRequest(msg)
	case message.MsgTypeSessionSetDeletionResponse:
		handler.HandlePfcpSessionSetDeletionResponse(msg)
	case message.MsgTypeSessionEstablishmentResponse:
		handler.HandlePfcpSessionEstablishmentResponse(msg)
	case message.MsgTypeSessionModificationResponse:
		handler.HandlePfcpSessionModificationResponse(msg)
	case message.MsgTypeSessionDeletionResponse:
		handler.HandlePfcpSessionDeletionResponse(msg)
	case message.MsgTypeSessionReportRequest:
		handler.HandlePfcpSessionReportRequest(msg)
	case message.MsgTypeSessionReportResponse:
		handler.HandlePfcpSessionReportResponse(msg)
	default:
		logger.SmfLog.Errorf("Unknown PFCP message type: %d", msgType)
		return
	}
}
