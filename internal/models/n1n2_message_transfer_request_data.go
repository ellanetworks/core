package models

type N1N2MessageTransferReqData struct {
	N1MessageContainer     *N1MessageContainer `json:"n1MessageContainer,omitempty"`
	N2InfoContainer        *N2InfoContainer    `json:"n2InfoContainer,omitempty"`
	SkipInd                bool                `json:"skipInd,omitempty"`
	LastMsgIndication      bool                `json:"lastMsgIndication,omitempty"`
	PduSessionId           int32               `json:"pduSessionId,omitempty"`
	LcsCorrelationId       string              `json:"lcsCorrelationId,omitempty"`
	Ppi                    int32               `json:"ppi,omitempty"`
	Arp                    *Arp                `json:"arp,omitempty"`
	Var5qi                 int32               `json:"5qi,omitempty"`
	N1n2FailureTxfNotifURI string              `json:"n1n2FailureTxfNotifURI,omitempty"`
	SmfReallocationInd     bool                `json:"smfReallocationInd,omitempty"`
	// AreaOfValidity         *AreaOfValidity     `json:"areaOfValidity,omitempty"`
	SupportedFeatures string `json:"supportedFeatures,omitempty"`
}
