package models

type PolicyAssociationUpdateRequest struct {
	Triggers    []RequestTrigger
	ServAreaRes *ServiceAreaRestriction
	Rfsp        int32
	UserLoc     *UserLocation
}
