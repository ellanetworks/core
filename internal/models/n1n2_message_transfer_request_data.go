package models

type N1N2MessageTransferReqData struct {
	N1MessageClass  N1MessageClass
	N2InfoContainer *N2InfoContainer
	SkipInd         bool
	PduSessionID    int32
	Ppi             int32
}
