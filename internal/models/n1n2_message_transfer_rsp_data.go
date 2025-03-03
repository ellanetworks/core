package models

type N1N2MessageTransferRspData struct {
	Cause             N1N2MessageTransferCause
	SupportedFeatures string
}
