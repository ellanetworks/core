package models

type PolicyAssociationRequest struct {
	Supi        string
	AccessType  AccessType
	UserLoc     *UserLocation
	ServingPlmn *PlmnID
	// Rfsp        int32
}
