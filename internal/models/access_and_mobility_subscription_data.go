package models

type AccessAndMobilitySubscriptionData struct {
	SubscribedUeAmbr       *AmbrRm
	Nssai                  *Nssai
	RatRestrictions        []RatType
	ForbiddenAreas         []Area
	ServiceAreaRestriction *ServiceAreaRestriction
	RfspIndex              int32
}
