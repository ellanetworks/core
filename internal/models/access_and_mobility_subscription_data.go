package models

type AccessAndMobilitySubscriptionData struct {
	// SupportedFeatures string   `json:"supportedFeatures,omitempty" bson:"supportedFeatures"`
	// Gpsis             []string `json:"gpsis,omitempty" bson:"gpsis"`
	// InternalGroupIds  []string `json:"internalGroupIds,omitempty" bson:"internalGroupIds"`
	SubscribedUeAmbr       *AmbrRm
	Nssai                  *Nssai
	RatRestrictions        []RatType
	ForbiddenAreas         []Area
	ServiceAreaRestriction *ServiceAreaRestriction
	// CoreNetworkTypeRestrictions []CoreNetworkType       `json:"coreNetworkTypeRestrictions,omitempty" bson:"coreNetworkTypeRestrictions"`
	RfspIndex int32
	// SubsRegTimer  int32 `json:"subsRegTimer,omitempty" bson:"subsRegTimer"`
	// UeUsageType   int32 `json:"ueUsageType,omitempty" bson:"ueUsageType"`
	// MpsPriority   bool  `json:"mpsPriority,omitempty" bson:"mpsPriority"`
	// McsPriority   bool  `json:"mcsPriority,omitempty" bson:"mcsPriority"`
	// ActiveTime    int32 `json:"activeTime,omitempty" bson:"activeTime"`
	// DlPacketCount int32 `json:"dlPacketCount,omitempty" bson:"dlPacketCount"`
	// SorInfo                     *SorInfo                `json:"sorInfo,omitempty" bson:"sorInfo"`
	// MicoAllowed     bool     `json:"micoAllowed,omitempty" bson:"micoAllowed"`
	// SharedAmDataIds []string `json:"sharedAmDataIds,omitempty" bson:"sharedAmDataIds"`
	// OdbPacketServices           OdbPacketServices       `json:"odbPacketServices,omitempty" bson:"odbPacketServices"`
}
