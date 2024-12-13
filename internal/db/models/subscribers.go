package models

const (
	AuthMethod__5_G_AKA      AuthMethod = "5G_AKA"
	AuthMethod_EAP_AKA_PRIME AuthMethod = "EAP_AKA_PRIME"
)

type Constants struct {
	C1 string `json:"c1" bson:"c1"`
	C2 string `json:"c2" bson:"c2"`
	C3 string `json:"c3" bson:"c3"`
	C4 string `json:"c4" bson:"c4"`
	C5 string `json:"c5" bson:"c5"`
}

type Rotations struct {
	R1 string `json:"r1" bson:"r1"`
	R2 string `json:"r2" bson:"r2"`
	R3 string `json:"r3" bson:"r3"`
	R4 string `json:"r4" bson:"r4"`
	R5 string `json:"r5" bson:"r5"`
}

type AuthMethod string

type PermanentKey struct {
	PermanentKeyValue   string `json:"permanentKeyValue" bson:"permanentKeyValue"`
	EncryptionKey       int32  `json:"encryptionKey" bson:"encryptionKey"`
	EncryptionAlgorithm int32  `json:"encryptionAlgorithm" bson:"encryptionAlgorithm"`
}

type Op struct {
	OpValue             string `json:"opValue" bson:"opValue"`
	EncryptionKey       int32  `json:"encryptionKey" bson:"encryptionKey"`
	EncryptionAlgorithm int32  `json:"encryptionAlgorithm" bson:"encryptionAlgorithm"`
}

type Opc struct {
	OpcValue            string `json:"opcValue" bson:"opcValue"`
	EncryptionKey       int32  `json:"encryptionKey" bson:"encryptionKey"`
	EncryptionAlgorithm int32  `json:"encryptionAlgorithm" bson:"encryptionAlgorithm"`
}

type Milenage struct {
	Op        *Op        `json:"op,omitempty" bson:"op"`
	Rotations *Rotations `json:"rotations,omitempty" bson:"rotations"`
	Constants *Constants `json:"constants,omitempty" bson:"constants"`
}

type AuthenticationSubscription struct {
	Milenage                      *Milenage     `json:"milenage,omitempty" bson:"milenage"`
	Opc                           *Opc          `json:"opc,omitempty" bson:"opc"`
	PermanentKey                  *PermanentKey `json:"permanentKey" bson:"permanentKey"`
	SequenceNumber                string        `json:"sequenceNumber" bson:"sequenceNumber"`
	AuthenticationManagementField string        `json:"authenticationManagementField,omitempty" bson:"authenticationManagementField"`
	AuthenticationMethod          AuthMethod    `json:"authenticationMethod" bson:"authenticationMethod"`
}

type AmPolicyData struct {
	SubscCats []string `json:"subscCats,omitempty" bson:"subscCats"`
}

type AmbrRm struct {
	Uplink   string `json:"uplink" yaml:"uplink" bson:"uplink" mapstructure:"Uplink"`
	Downlink string `json:"downlink" yaml:"downlink" bson:"downlink" mapstructure:"Downlink"`
}

type Nssai struct {
	SupportedFeatures   string   `json:"supportedFeatures,omitempty" yaml:"supportedFeatures" bson:"supportedFeatures" mapstructure:"SupportedFeatures"`
	DefaultSingleNssais []Snssai `json:"defaultSingleNssais" yaml:"defaultSingleNssais" bson:"defaultSingleNssais" mapstructure:"DefaultSingleNssais"`
	SingleNssais        []Snssai `json:"singleNssais,omitempty" yaml:"singleNssais" bson:"singleNssais" mapstructure:"SingleNssais"`
}

type AccessAndMobilitySubscriptionData struct {
	Nssai            *Nssai  `json:"nssai,omitempty" bson:"nssai"`
	ServingPlmnId    string  `json:"servingPlmnId"`
	SubscribedUeAmbr *AmbrRm `json:"subscribedUeAmbr,omitempty" bson:"subscribedUeAmbr"`
}

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
	SessionAmbr     *Ambr                 `json:"sessionAmbr,omitempty" yaml:"sessionAmbr" bson:"sessionAmbr" mapstructure:"SessionAmbr"`
	SscModes        *SscModes             `json:"sscModes" yaml:"sscModes" bson:"sscModes" mapstructure:"SscModes"`
	Var5gQosProfile *SubscribedDefaultQos `json:"5gQosProfile,omitempty" yaml:"5gQosProfile" bson:"5gQosProfile" mapstructure:"Var5gQosProfile"`
	PduSessionTypes *PduSessionTypes      `json:"pduSessionTypes" yaml:"pduSessionTypes" bson:"pduSessionTypes" mapstructure:"PduSessionTypes"`
}

type SessionManagementSubscriptionData struct {
	DnnConfigurations map[string]DnnConfiguration `json:"dnnConfigurations,omitempty" yaml:"dnnConfigurations" bson:"dnnConfigurations" mapstructure:"DnnConfigurations"`
	ServingPlmnId     string                      `json:"servingPlmnId"`
	SingleNssai       *Snssai                     `json:"singleNssai" yaml:"singleNssai" bson:"singleNssai" mapstructure:"SingleNssai"`
}

type SmPolicyDnnData struct {
	Dnn string `json:"dnn" bson:"dnn"`
}

type SmPolicySnssaiData struct {
	Snssai          *Snssai                    `json:"snssai" bson:"snssai"`
	SmPolicyDnnData map[string]SmPolicyDnnData `json:"smPolicyDnnData,omitempty" bson:"smPolicyDnnData"`
}

type SmPolicyData struct {
	SmPolicySnssaiData map[string]SmPolicySnssaiData `json:"smPolicySnssaiData" bson:"smPolicySnssaiData"`
}

type DnnInfo struct {
	Dnn string `json:"dnn" yaml:"dnn" bson:"dnn" mapstructure:"Dnn"`
}

type SnssaiInfo struct {
	DnnInfos []DnnInfo `json:"dnnInfos" yaml:"dnnInfos" bson:"dnnInfos" mapstructure:"DnnInfos"`
}

type SmfSelectionSubscriptionData struct {
	ServingPlmnId         string                `json:"servingPlmnId"`
	SubscribedSnssaiInfos map[string]SnssaiInfo `json:"subscribedSnssaiInfos,omitempty" yaml:"subscribedSnssaiInfos" bson:"subscribedSnssaiInfos" mapstructure:"SubscribedSnssaiInfos"`
}

type Snssai struct {
	Sst int32  `json:"sst" yaml:"sst" bson:"sst" mapstructure:"Sst"`
	Sd  string `json:"sd,omitempty" yaml:"sd" bson:"sd" mapstructure:"Sd"`
}

type Subscriber struct {
	PlmnID                            string                               `json:"plmnID"`
	UeId                              string                               `json:"ueId"`
	AuthenticationSubscription        AuthenticationSubscription           `json:"AuthenticationSubscription"`
	AccessAndMobilitySubscriptionData AccessAndMobilitySubscriptionData    `json:"AccessAndMobilitySubscriptionData"`
	SessionManagementSubscriptionData []*SessionManagementSubscriptionData `json:"SessionManagementSubscriptionData"`
	SmfSelectionSubscriptionData      SmfSelectionSubscriptionData         `json:"SmfSelectionSubscriptionData"`
	AmPolicyData                      AmPolicyData                         `json:"AmPolicyData"`
	SmPolicyData                      SmPolicyData                         `json:"SmPolicyData"`
}
