package models

const (
	PduSessionType_IPV4         PduSessionType = "IPV4"
	PduSessionType_IPV6         PduSessionType = "IPV6"
	PduSessionType_IPV4_V6      PduSessionType = "IPV4V6"
	PduSessionType_UNSTRUCTURED PduSessionType = "UNSTRUCTURED"
	PduSessionType_ETHERNET     PduSessionType = "ETHERNET"
)

const (
	SscMode__1 SscMode = "SSC_MODE_1"
	SscMode__2 SscMode = "SSC_MODE_2"
	SscMode__3 SscMode = "SSC_MODE_3"
)

const (
	UpConfidentiality_REQUIRED   UpConfidentiality = "REQUIRED"
	UpConfidentiality_PREFERRED  UpConfidentiality = "PREFERRED"
	UpConfidentiality_NOT_NEEDED UpConfidentiality = "NOT_NEEDED"
)

const (
	UpIntegrity_REQUIRED   UpIntegrity = "REQUIRED"
	UpIntegrity_PREFERRED  UpIntegrity = "PREFERRED"
	UpIntegrity_NOT_NEEDED UpIntegrity = "NOT_NEEDED"
)

type PreemptionVulnerability string

const (
	PreemptionVulnerability_NOT_PREEMPTABLE PreemptionVulnerability = "NOT_PREEMPTABLE"
	PreemptionVulnerability_PREEMPTABLE     PreemptionVulnerability = "PREEMPTABLE"
)

const (
	PreemptionCapability_NOT_PREEMPT PreemptionCapability = "NOT_PREEMPT"
	PreemptionCapability_MAY_PREEMPT PreemptionCapability = "MAY_PREEMPT"
)

type PreemptionCapability string

type UpIntegrity string

type SscMode string

type PduSessionType string

type UpConfidentiality string

type Arp struct {
	PriorityLevel int32                   `json:"priorityLevel" yaml:"priorityLevel" bson:"priorityLevel" mapstructure:"PriorityLevel"`
	PreemptCap    PreemptionCapability    `json:"preemptCap" yaml:"preemptCap" bson:"preemptCap" mapstructure:"PreemptCap"`
	PreemptVuln   PreemptionVulnerability `json:"preemptVuln" yaml:"preemptVuln" bson:"preemptVuln" mapstructure:"PreemptVuln"`
}

type IpAddress struct {
	Ipv4Addr   string `json:"ipv4Addr,omitempty" yaml:"ipv4Addr" bson:"ipv4Addr" mapstructure:"Ipv4Addr"`
	Ipv6Addr   string `json:"ipv6Addr,omitempty" yaml:"ipv6Addr" bson:"ipv6Addr" mapstructure:"Ipv6Addr"`
	Ipv6Prefix string `json:"ipv6Prefix,omitempty" yaml:"ipv6Prefix" bson:"ipv6Prefix" mapstructure:"Ipv6Prefix"`
}

type UpSecurity struct {
	UpIntegr UpIntegrity       `json:"upIntegr" yaml:"upIntegr" bson:"upIntegr" mapstructure:"UpIntegr"`
	UpConfid UpConfidentiality `json:"upConfid" yaml:"upConfid" bson:"upConfid" mapstructure:"UpConfid"`
}

type Ambr struct {
	Uplink   string `json:"uplink" yaml:"uplink" bson:"uplink" mapstructure:"Uplink"`
	Downlink string `json:"downlink" yaml:"downlink" bson:"downlink" mapstructure:"Downlink"`
}

type SubscribedDefaultQos struct {
	Var5qi        int32 `json:"5qi" yaml:"5qi" bson:"5qi" mapstructure:"Var5qi"`
	Arp           *Arp  `json:"arp" yaml:"arp" bson:"arp" mapstructure:"Arp"`
	PriorityLevel int32 `json:"priorityLevel,omitempty" yaml:"priorityLevel" bson:"priorityLevel" mapstructure:"PriorityLevel"`
}

type SscModes struct {
	DefaultSscMode  SscMode   `json:"defaultSscMode" yaml:"defaultSscMode" bson:"defaultSscMode" mapstructure:"DefaultSscMode"`
	AllowedSscModes []SscMode `json:"allowedSscModes,omitempty" yaml:"allowedSscModes" bson:"allowedSscModes" mapstructure:"AllowedSscModes"`
}

type PduSessionTypes struct {
	DefaultSessionType  PduSessionType   `json:"defaultSessionType" yaml:"defaultSessionType" bson:"defaultSessionType" mapstructure:"DefaultSessionType"`
	AllowedSessionTypes []PduSessionType `json:"allowedSessionTypes,omitempty" yaml:"allowedSessionTypes" bson:"allowedSessionTypes" mapstructure:"AllowedSessionTypes"`
}

type DnnConfiguration struct {
	PduSessionTypes                *PduSessionTypes      `json:"pduSessionTypes" yaml:"pduSessionTypes" bson:"pduSessionTypes" mapstructure:"PduSessionTypes"`
	SscModes                       *SscModes             `json:"sscModes" yaml:"sscModes" bson:"sscModes" mapstructure:"SscModes"`
	IwkEpsInd                      bool                  `json:"iwkEpsInd,omitempty" yaml:"iwkEpsInd" bson:"iwkEpsInd" mapstructure:"IwkEpsInd"`
	Var5gQosProfile                *SubscribedDefaultQos `json:"5gQosProfile,omitempty" yaml:"5gQosProfile" bson:"5gQosProfile" mapstructure:"Var5gQosProfile"`
	SessionAmbr                    *Ambr                 `json:"sessionAmbr,omitempty" yaml:"sessionAmbr" bson:"sessionAmbr" mapstructure:"SessionAmbr"`
	Var3gppChargingCharacteristics string                `json:"3gppChargingCharacteristics,omitempty" yaml:"3gppChargingCharacteristics" bson:"3gppChargingCharacteristics" mapstructure:"Var3gppChargingCharacteristics"`
	StaticIpAddress                []IpAddress           `json:"staticIpAddress,omitempty" yaml:"staticIpAddress" bson:"staticIpAddress" mapstructure:"StaticIpAddress"`
	UpSecurity                     *UpSecurity           `json:"upSecurity,omitempty" yaml:"upSecurity" bson:"upSecurity" mapstructure:"UpSecurity"`
}

type SessionManagementSubscriptionData struct {
	SingleNssai                *Snssai                     `json:"singleNssai" yaml:"singleNssai" bson:"singleNssai" mapstructure:"SingleNssai"`
	DnnConfigurations          map[string]DnnConfiguration `json:"dnnConfigurations,omitempty" yaml:"dnnConfigurations" bson:"dnnConfigurations" mapstructure:"DnnConfigurations"`
	InternalGroupIds           []string                    `json:"internalGroupIds,omitempty" yaml:"internalGroupIds" bson:"internalGroupIds" mapstructure:"InternalGroupIds"`
	SharedDnnConfigurationsIds string                      `json:"sharedDnnConfigurationsIds,omitempty" yaml:"sharedDnnConfigurationsIds" bson:"sharedDnnConfigurationsIds" mapstructure:"SharedDnnConfigurationsIds"`
}
