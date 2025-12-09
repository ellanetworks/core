package models

type PolicyAssociationUpdateRequest struct {
	Triggers []RequestTrigger
	UserLoc  *UserLocation
}
