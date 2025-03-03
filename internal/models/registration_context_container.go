package models

type RegistrationContextContainer struct {
	UeContext           *UeContext
	AnType              AccessType
	AnN2ApId            int32
	RanNodeId           *GlobalRanNodeId
	InitialAmfName      string
	UserLocation        *UserLocation
	RrcEstCause         string
	UeContextRequest    bool
	AnN2IPv4Addr        string
	AllowedNssai        *AllowedNssai
	RejectedNssaiInPlmn []Snssai
	RejectedNssaiInTa   []Snssai
}
