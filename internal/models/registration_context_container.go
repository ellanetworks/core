package models

type RegistrationContextContainer struct {
	UeContext           *UeContext         `json:"ueContext"`
	LocalTimeZone       string             `json:"localTimeZone,omitempty"`
	AnType              AccessType         `json:"anType"`
	AnN2ApId            int32              `json:"anN2ApId"`
	RanNodeId           *GlobalRanNodeId   `json:"ranNodeId"`
	InitialAmfName      string             `json:"initialAmfName"`
	UserLocation        *UserLocation      `json:"userLocation"`
	RrcEstCause         string             `json:"rrcEstCause,omitempty"`
	UeContextRequest    bool               `json:"ueContextRequest,omitempty"`
	AnN2IPv4Addr        string             `json:"anN2IPv4Addr,omitempty"`
	AnN2IPv6Addr        string             `json:"anN2IPv6Addr,omitempty"`
	AllowedNssai        *AllowedNssai      `json:"allowedNssai,omitempty"`
	ConfiguredNssai     []ConfiguredSnssai `json:"configuredNssai,omitempty"`
	RejectedNssaiInPlmn []Snssai           `json:"rejectedNssaiInPlmn,omitempty"`
	RejectedNssaiInTa   []Snssai           `json:"rejectedNssaiInTa,omitempty"`
}
