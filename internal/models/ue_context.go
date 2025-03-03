package models

type UeContext struct {
	Supi                   string
	SupiUnauthInd          bool
	GpsiList               []string
	Pei                    string
	UdmGroupId             string
	AusfGroupId            string
	RoutingIndicator       string
	GroupList              []string
	DrxParameter           string
	SubRfsp                int32
	UsedRfsp               int32
	SubUeAmbr              *Ambr
	SeafData               *SeafData
	AmPolicyReqTriggerList []AmPolicyReqTrigger
	RestrictedRatList      []RatType
	ForbiddenAreaList      []Area
	ServiceAreaRestriction *ServiceAreaRestriction
	MmContextList          []MmContext
	SessionContextList     []PduSessionContext
	TraceData              *TraceData
}
