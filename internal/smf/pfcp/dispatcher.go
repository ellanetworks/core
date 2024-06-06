package pfcp

import (
	"github.com/omec-project/pfcp"
	"github.com/omec-project/pfcp/pfcpUdp"
	"github.com/yeastengine/ella/internal/smf/logger"
	"github.com/yeastengine/ella/internal/smf/pfcp/handler"
)

func Dispatch(msg *pfcpUdp.Message) {
	// TODO: Add return status to all handlers
	switch msg.PfcpMessage.Header.MessageType {
	case pfcp.PFCP_HEARTBEAT_REQUEST:
		handler.HandlePfcpHeartbeatRequest(msg)
	case pfcp.PFCP_HEARTBEAT_RESPONSE:
		handler.HandlePfcpHeartbeatResponse(msg)
	case pfcp.PFCP_PFD_MANAGEMENT_REQUEST:
		handler.HandlePfcpPfdManagementRequest(msg)
	case pfcp.PFCP_PFD_MANAGEMENT_RESPONSE:
		handler.HandlePfcpPfdManagementResponse(msg)
	case pfcp.PFCP_ASSOCIATION_SETUP_REQUEST:
		handler.HandlePfcpAssociationSetupRequest(msg)
	case pfcp.PFCP_ASSOCIATION_SETUP_RESPONSE:
		handler.HandlePfcpAssociationSetupResponse(msg)
	case pfcp.PFCP_ASSOCIATION_UPDATE_REQUEST:
		handler.HandlePfcpAssociationUpdateRequest(msg)
	case pfcp.PFCP_ASSOCIATION_UPDATE_RESPONSE:
		handler.HandlePfcpAssociationUpdateResponse(msg)
	case pfcp.PFCP_ASSOCIATION_RELEASE_REQUEST:
		handler.HandlePfcpAssociationReleaseRequest(msg)
	case pfcp.PFCP_ASSOCIATION_RELEASE_RESPONSE:
		handler.HandlePfcpAssociationReleaseResponse(msg)
	case pfcp.PFCP_VERSION_NOT_SUPPORTED_RESPONSE:
		handler.HandlePfcpVersionNotSupportedResponse(msg)
	case pfcp.PFCP_NODE_REPORT_REQUEST:
		handler.HandlePfcpNodeReportRequest(msg)
	case pfcp.PFCP_NODE_REPORT_RESPONSE:
		handler.HandlePfcpNodeReportResponse(msg)
	case pfcp.PFCP_SESSION_SET_DELETION_REQUEST:
		handler.HandlePfcpSessionSetDeletionRequest(msg)
	case pfcp.PFCP_SESSION_SET_DELETION_RESPONSE:
		handler.HandlePfcpSessionSetDeletionResponse(msg)
	case pfcp.PFCP_SESSION_ESTABLISHMENT_RESPONSE:
		handler.HandlePfcpSessionEstablishmentResponse(msg)
	case pfcp.PFCP_SESSION_MODIFICATION_RESPONSE:
		handler.HandlePfcpSessionModificationResponse(msg)
	case pfcp.PFCP_SESSION_DELETION_RESPONSE:
		handler.HandlePfcpSessionDeletionResponse(msg)
	case pfcp.PFCP_SESSION_REPORT_REQUEST:
		handler.HandlePfcpSessionReportRequest(msg)
	case pfcp.PFCP_SESSION_REPORT_RESPONSE:
		handler.HandlePfcpSessionReportResponse(msg)
	default:
		logger.PfcpLog.Errorf("Unknown PFCP message type: %d", msg.PfcpMessage.Header.MessageType)
		return
	}
}
