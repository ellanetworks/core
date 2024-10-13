package svcmsgtypes

type SmfMsgType string

// List of Msgs
const (

	// N11 Service
	MsgTypeNone           SmfMsgType = "none"
	CreateSmContext       SmfMsgType = "CreateSmContext"
	UpdateSmContext       SmfMsgType = "UpdateSmContext"
	ReleaseSmContext      SmfMsgType = "ReleaseSmContext"
	NotifySmContextStatus SmfMsgType = "NotifySmContextStatus"
	RetrieveSmContext     SmfMsgType = "RetrieveSmContext"
	NsmfPDUSessionCreate  SmfMsgType = "Create"  // Create a PDU session in the H-SMF
	NsmfPDUSessionUpdate  SmfMsgType = "Update"  // Update a PDU session in the H-SMF or V- SMF
	NsmfPDUSessionRelease SmfMsgType = "Release" // Release a PDU session in the H-SMF

	// NUDM_
	SmSubscriptionDataRetrieval SmfMsgType = "SmSubscriptionDataRetrieval"

	// NPCF_
	SmPolicyAssociationCreate       SmfMsgType = "SmPolicyAssociationCreate"
	SmPolicyAssociationDelete       SmfMsgType = "SmPolicyAssociationDelete"
	SmPolicyUpdateNotification      SmfMsgType = "SmPolicyUpdateNotification"
	SmPolicyTerminationNotification SmfMsgType = "SmPolicyTerminationNotification"

	// AMF_
	N1N2MessageTransfer                    SmfMsgType = "N1N2MessageTransfer"
	PfcpSessCreateFailure                  SmfMsgType = "PfcpSessCreateFailure"
	N1N2MessageTransferFailureNotification SmfMsgType = "N1N2MessageTransferFailureNotification"

	// PFCP
	PfcpSessCreate  SmfMsgType = "PfcpSessCreate"
	PfcpSessModify  SmfMsgType = "PfcpSessModify"
	PfcpSessRelease SmfMsgType = "PfcpSessRelease"
)
