package models

type N1N2MessageTransferReqData struct {
	N1MessageContainer     *N1MessageContainer
	N2InfoContainer        *N2InfoContainer
	SkipInd                bool
	LastMsgIndication      bool
	PduSessionId           int32
	LcsCorrelationId       string
	Ppi                    int32
	Arp                    *Arp
	Var5qi                 int32
	N1n2FailureTxfNotifURI string
	SmfReallocationInd     bool
	SupportedFeatures      string
}
