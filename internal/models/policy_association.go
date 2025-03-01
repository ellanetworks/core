package models

type PolicyAssociation struct {
	Request *PolicyAssociationRequest
	// Request Triggers that the PCF subscribes. Only values \"LOC_CH\" and \"PRA_CH\" are permitted.
	Triggers    []RequestTrigger
	ServAreaRes *ServiceAreaRestriction
	Rfsp        int32
	Pras        map[string]PresenceInfo
	SuppFeat    string
}
