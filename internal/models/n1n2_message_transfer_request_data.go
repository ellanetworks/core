package models

type N1N2MessageTransferReqData struct {
	PduSessionID uint8
	NgapIeType   N2SmInfoType
	SNssai       *Snssai
}
