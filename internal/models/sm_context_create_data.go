package models

type SmContextCreateData struct {
	Supi                string  `json:"supi,omitempty"`
	UnauthenticatedSupi bool    `json:"unauthenticatedSupi,omitempty"`
	Pei                 string  `json:"pei,omitempty"`
	Gpsi                string  `json:"gpsi,omitempty"`
	PduSessionId        int32   `json:"pduSessionId,omitempty"`
	Dnn                 string  `json:"dnn,omitempty"`
	SNssai              *Snssai `json:"sNssai,omitempty"`
	HplmnSnssai         *Snssai `json:"hplmnSnssai,omitempty"`
	ServingNfId         string  `json:"servingNfId"`
	Guami               *Guami  `json:"guami,omitempty"`
	// ServiceName             ServiceName               `json:"serviceName,omitempty"`
	ServingNetwork          *PlmnId          `json:"servingNetwork"`
	RequestType             RequestType      `json:"requestType,omitempty"`
	N1SmMsg                 *RefToBinaryData `json:"n1SmMsg,omitempty"`
	AnType                  AccessType       `json:"anType"`
	RatType                 RatType          `json:"ratType,omitempty"`
	PresenceInLadn          PresenceState    `json:"presenceInLadn,omitempty"`
	UeLocation              *UserLocation    `json:"ueLocation,omitempty"`
	UeTimeZone              string           `json:"ueTimeZone,omitempty"`
	AddUeLocation           *UserLocation    `json:"addUeLocation,omitempty"`
	SmContextStatusUri      string           `json:"smContextStatusUri"`
	HSmfUri                 string           `json:"hSmfUri,omitempty"`
	AdditionalHsmfUri       []string         `json:"additionalHsmfUri,omitempty"`
	OldPduSessionId         int32            `json:"oldPduSessionId,omitempty"`
	PduSessionsActivateList []int32          `json:"pduSessionsActivateList,omitempty"`
	UeEpsPdnConnection      string           `json:"ueEpsPdnConnection,omitempty"`
	HoState                 HoState          `json:"hoState,omitempty"`
	PcfId                   string           `json:"pcfId,omitempty"`
	NrfUri                  string           `json:"nrfUri,omitempty"`
	SupportedFeatures       string           `json:"supportedFeatures,omitempty"`
	// SelMode                 DnnSelectionMode          `json:"selMode,omitempty"`
	// BackupAmfInfo           []BackupAmfInfo           `json:"backupAmfInfo,omitempty"`
	// TraceData               *TraceData                `json:"traceData,omitempty"`
	UdmGroupId       string `json:"udmGroupId,omitempty"`
	RoutingIndicator string `json:"routingIndicator,omitempty"`
	// EpsInterworkingInd      EpsInterworkingIndication `json:"epsInterworkingInd,omitempty"`
	IndirectForwardingFlag bool `json:"indirectForwardingFlag,omitempty"`
}
