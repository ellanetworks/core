package models

type PolicyUpdate struct {
	ResourceUri string
	// Request Triggers that the PCF subscribes. Only values \"LOC_CH\" and \"PRA_CH\" are permitted.
	Triggers    []RequestTrigger
	ServAreaRes *ServiceAreaRestriction
	Rfsp        int32
}
