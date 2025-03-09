package models

type PolicyAssociationRequest struct {
	Supi        string
	Gpsi        string
	AccessType  AccessType
	Pei         string
	UserLoc     *UserLocation
	TimeZone    string
	ServingPlmn *PlmnID
	ServAreaRes *ServiceAreaRestriction
	Rfsp        int32
	Guami       *Guami
}
