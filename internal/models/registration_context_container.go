package models

type RegistrationContextContainer struct {
	UeContext           *UeContext
	LocalTimeZone       string
	AnType              AccessType
	AnN2ApId            int32
	RanNodeId           *GlobalRanNodeId
	InitialAmfName      string
	UserLocation        *UserLocation
	RrcEstCause         string
	UeContextRequest    bool
	AnN2IPv4Addr        string
	AnN2IPv6Addr        string
	AllowedNssai        *AllowedNssai
	ConfiguredNssai     []ConfiguredSnssai
	RejectedNssaiInPlmn []Snssai
	RejectedNssaiInTa   []Snssai
}
