package models

type N1N2MessageTransferReqData struct {
	N1MessageContainer *N1MessageContainer
	N2InfoContainer    *N2InfoContainer
	SkipInd            bool
	PduSessionId       int32
	Ppi                int32
}
