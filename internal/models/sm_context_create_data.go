package models

type SmContextCreateData struct {
	Supi                    string
	UnauthenticatedSupi     bool
	Pei                     string
	Gpsi                    string
	PduSessionId            int32
	Dnn                     string
	SNssai                  *Snssai
	HplmnSnssai             *Snssai
	ServingNfId             string
	Guami                   *Guami
	ServingNetwork          *PlmnId
	RequestType             RequestType
	N1SmMsg                 *RefToBinaryData
	AnType                  AccessType
	RatType                 RatType
	PresenceInLadn          PresenceState
	UeLocation              *UserLocation
	UeTimeZone              string
	AddUeLocation           *UserLocation
	SmContextStatusUri      string
	HSmfUri                 string
	AdditionalHsmfUri       []string
	OldPduSessionId         int32
	PduSessionsActivateList []int32
	UeEpsPdnConnection      string
	HoState                 HoState
	PcfId                   string
	NrfUri                  string
	SupportedFeatures       string
	UdmGroupId              string
	RoutingIndicator        string
	IndirectForwardingFlag  bool
}
