package models

type UeContext struct {
	Supi                   string
	SupiUnauthInd          bool
	GpsiList               []string
	Pei                    string
	UdmGroupID             string
	AusfGroupID            string
	RoutingIndicator       string
	SubRfsp                int32
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
