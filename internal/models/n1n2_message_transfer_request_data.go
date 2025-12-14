package models

type N1N2MessageTransferReqData struct {
	PduSessionID int32
	NgapIeType   N2SmInfoType
	SNssai       *Snssai
}
