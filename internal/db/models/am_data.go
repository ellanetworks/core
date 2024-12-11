package models

import (
	"time"
)

const (
	OdbPacketServices_ALL_PACKET_SERVICES    OdbPacketServices = "ALL_PACKET_SERVICES"
	OdbPacketServices_ROAMER_ACCESS_HPLMN_AP OdbPacketServices = "ROAMER_ACCESS_HPLMN_AP"
	OdbPacketServices_ROAMER_ACCESS_VPLMN_AP OdbPacketServices = "ROAMER_ACCESS_VPLMN_AP"
)

const (
	CoreNetworkType__5_GC CoreNetworkType = "5GC"
	CoreNetworkType_EPC   CoreNetworkType = "EPC"
)

const (
	RestrictionType_ALLOWED_AREAS     RestrictionType = "ALLOWED_AREAS"
	RestrictionType_NOT_ALLOWED_AREAS RestrictionType = "NOT_ALLOWED_AREAS"
)

const (
	RatType_NR      RatType = "NR"
	RatType_EUTRA   RatType = "EUTRA"
	RatType_WLAN    RatType = "WLAN"
	RatType_VIRTUAL RatType = "VIRTUAL"
)

type RatType string

type Area struct {
	Tacs      []string `json:"tacs,omitempty" yaml:"tacs" bson:"tacs" mapstructure:"Tacs"`
	AreaCodes string   `json:"areaCodes,omitempty" yaml:"areaCodes" bson:"areaCodes" mapstructure:"AreaCodes"`
}

type CoreNetworkType string

type RestrictionType string

type ServiceAreaRestriction struct {
	RestrictionType RestrictionType `json:"restrictionType,omitempty" yaml:"restrictionType" bson:"restrictionType" mapstructure:"RestrictionType"`
	Areas           []Area          `json:"areas,omitempty" yaml:"areas" bson:"areas" mapstructure:"Areas"`
	MaxNumOfTAs     int32           `json:"maxNumOfTAs,omitempty" yaml:"maxNumOfTAs" bson:"maxNumOfTAs" mapstructure:"MaxNumOfTAs"`
}

type SteeringContainer struct{}

type SorInfo struct {
	SteeringContainer *SteeringContainer `json:"steeringContainer,omitempty" yaml:"steeringContainer" bson:"steeringContainer" mapstructure:"SteeringContainer"`
	AckInd            bool               `json:"ackInd" yaml:"ackInd" bson:"ackInd" mapstructure:"AckInd"`
	SorMacIausf       string             `json:"sorMacIausf,omitempty" yaml:"sorMacIausf" bson:"sorMacIausf" mapstructure:"SorMacIausf"`
	Countersor        string             `json:"countersor,omitempty" yaml:"countersor" bson:"countersor" mapstructure:"Countersor"`
	ProvisioningTime  *time.Time         `json:"provisioningTime" yaml:"provisioningTime" bson:"provisioningTime" mapstructure:"ProvisioningTime"`
}

type OdbPacketServices string

type AmPolicyData struct {
	SubscCats []string `json:"subscCats,omitempty" bson:"subscCats"`
}

type AmbrRm struct {
	Uplink   string `json:"uplink" yaml:"uplink" bson:"uplink" mapstructure:"Uplink"`
	Downlink string `json:"downlink" yaml:"downlink" bson:"downlink" mapstructure:"Downlink"`
}

type Nssai struct {
	SupportedFeatures string   `json:"supportedFeatures,omitempty" yaml:"supportedFeatures" bson:"supportedFeatures" mapstructure:"SupportedFeatures"`
	SingleNssais      []Snssai `json:"singleNssais,omitempty" yaml:"singleNssais" bson:"singleNssais" mapstructure:"SingleNssais"`
}

type AccessAndMobilitySubscriptionData struct {
	UeId                        string                  `json:"ueId"`
	ServingPlmnId               string                  `json:"servingPlmnId"`
	SupportedFeatures           string                  `json:"supportedFeatures,omitempty" bson:"supportedFeatures"`
	InternalGroupIds            []string                `json:"internalGroupIds,omitempty" bson:"internalGroupIds"`
	SubscribedUeAmbr            *AmbrRm                 `json:"subscribedUeAmbr,omitempty" bson:"subscribedUeAmbr"`
	Nssai                       *Nssai                  `json:"nssai,omitempty" bson:"nssai"`
	RatRestrictions             []RatType               `json:"ratRestrictions,omitempty" bson:"ratRestrictions"`
	ForbiddenAreas              []Area                  `json:"forbiddenAreas,omitempty" bson:"forbiddenAreas"`
	ServiceAreaRestriction      *ServiceAreaRestriction `json:"serviceAreaRestriction,omitempty" bson:"serviceAreaRestriction"`
	CoreNetworkTypeRestrictions []CoreNetworkType       `json:"coreNetworkTypeRestrictions,omitempty" bson:"coreNetworkTypeRestrictions"`
	RfspIndex                   int32                   `json:"rfspIndex,omitempty" bson:"rfspIndex"`
	SubsRegTimer                int32                   `json:"subsRegTimer,omitempty" bson:"subsRegTimer"`
	UeUsageType                 int32                   `json:"ueUsageType,omitempty" bson:"ueUsageType"`
	MpsPriority                 bool                    `json:"mpsPriority,omitempty" bson:"mpsPriority"`
	McsPriority                 bool                    `json:"mcsPriority,omitempty" bson:"mcsPriority"`
	ActiveTime                  int32                   `json:"activeTime,omitempty" bson:"activeTime"`
	DlPacketCount               int32                   `json:"dlPacketCount,omitempty" bson:"dlPacketCount"`
	SorInfo                     *SorInfo                `json:"sorInfo,omitempty" bson:"sorInfo"`
	MicoAllowed                 bool                    `json:"micoAllowed,omitempty" bson:"micoAllowed"`
	SharedAmDataIds             []string                `json:"sharedAmDataIds,omitempty" bson:"sharedAmDataIds"`
	OdbPacketServices           OdbPacketServices       `json:"odbPacketServices,omitempty" bson:"odbPacketServices"`
}
