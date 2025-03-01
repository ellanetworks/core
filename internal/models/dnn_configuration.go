package models

type DnnConfiguration struct {
	PduSessionTypes *PduSessionTypes `json:"pduSessionTypes" yaml:"pduSessionTypes" bson:"pduSessionTypes" mapstructure:"PduSessionTypes"`
	SscModes        *SscModes        `json:"sscModes" yaml:"sscModes" bson:"sscModes" mapstructure:"SscModes"`
	// IwkEpsInd                      bool                  `json:"iwkEpsInd,omitempty" yaml:"iwkEpsInd" bson:"iwkEpsInd" mapstructure:"IwkEpsInd"`
	Var5gQosProfile *SubscribedDefaultQos `json:"5gQosProfile,omitempty" yaml:"5gQosProfile" bson:"5gQosProfile" mapstructure:"Var5gQosProfile"`
	SessionAmbr     *Ambr                 `json:"sessionAmbr,omitempty" yaml:"sessionAmbr" bson:"sessionAmbr" mapstructure:"SessionAmbr"`
	// Var3gppChargingCharacteristics string                `json:"3gppChargingCharacteristics,omitempty" yaml:"3gppChargingCharacteristics" bson:"3gppChargingCharacteristics" mapstructure:"Var3gppChargingCharacteristics"`
	// StaticIpAddress                []IpAddress           `json:"staticIpAddress,omitempty" yaml:"staticIpAddress" bson:"staticIpAddress" mapstructure:"StaticIpAddress"`
	// UpSecurity                     *UpSecurity           `json:"upSecurity,omitempty" yaml:"upSecurity" bson:"upSecurity" mapstructure:"UpSecurity"`
}
