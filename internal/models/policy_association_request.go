package models

type PolicyAssociationRequest struct {
	Supi        string
	Gpsi        string
	AccessType  AccessType
	Pei         string
	UserLoc     *UserLocation
	ServingPlmn *PlmnID
	Rfsp        int32
}
