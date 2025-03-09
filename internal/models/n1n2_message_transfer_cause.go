package models

type N1N2MessageTransferCause string

const (
	N1N2MessageTransferCauseAttemptingToReachUE   N1N2MessageTransferCause = "ATTEMPTING_TO_REACH_UE"
	N1N2MessageTransferCauseN1N2TransferInitiated N1N2MessageTransferCause = "N1_N2_TRANSFER_INITIATED"
	N1N2MessageTransferCauseN1MsgNotTransferred   N1N2MessageTransferCause = "N1_MSG_NOT_TRANSFERRED"
)
