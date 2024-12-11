package models

import "time"

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

const (
	VectorAlgorithm_MILENAGE VectorAlgorithm = "MILENAGE"
	VectorAlgorithm_TUAK     VectorAlgorithm = "TUAK"
)

const (
	AuthMethod__5_G_AKA      AuthMethod = "5G_AKA"
	AuthMethod_EAP_AKA_PRIME AuthMethod = "EAP_AKA_PRIME"
)

const (
	TraceDepth_MINIMUM                     TraceDepth = "MINIMUM"
	TraceDepth_MEDIUM                      TraceDepth = "MEDIUM"
	TraceDepth_MAXIMUM                     TraceDepth = "MAXIMUM"
	TraceDepth_MINIMUM_WO_VENDOR_EXTENSION TraceDepth = "MINIMUM_WO_VENDOR_EXTENSION"
	TraceDepth_MEDIUM_WO_VENDOR_EXTENSION  TraceDepth = "MEDIUM_WO_VENDOR_EXTENSION"
	TraceDepth_MAXIMUM_WO_VENDOR_EXTENSION TraceDepth = "MAXIMUM_WO_VENDOR_EXTENSION"
)

type TraceDepth string

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

type Top struct {
	TopValue            string `json:"topValue" bson:"topValue"`
	EncryptionKey       int32  `json:"encryptionKey" bson:"encryptionKey"`
	EncryptionAlgorithm int32  `json:"encryptionAlgorithm" bson:"encryptionAlgorithm"`
}

type VectorAlgorithm string

type Topc struct {
	TopcValue           string `json:"topcValue" bson:"topcValue"`
	EncryptionKey       int32  `json:"encryptionKey" bson:"encryptionKey"`
	EncryptionAlgorithm int32  `json:"encryptionAlgorithm" bson:"encryptionAlgorithm"`
}

type Op struct {
	OpValue             string `json:"opValue" bson:"opValue"`
	EncryptionKey       int32  `json:"encryptionKey" bson:"encryptionKey"`
	EncryptionAlgorithm int32  `json:"encryptionAlgorithm" bson:"encryptionAlgorithm"`
}

type TraceData struct {
	TraceRef                 string     `json:"traceRef" yaml:"traceRef" bson:"traceRef" mapstructure:"TraceRef"`
	TraceDepth               TraceDepth `json:"traceDepth" yaml:"traceDepth" bson:"traceDepth" mapstructure:"TraceDepth"`
	NeTypeList               string     `json:"neTypeList" yaml:"neTypeList" bson:"neTypeList" mapstructure:"NeTypeList"`
	EventList                string     `json:"eventList" yaml:"eventList" bson:"eventList" mapstructure:"EventList"`
	CollectionEntityIpv4Addr string     `json:"collectionEntityIpv4Addr,omitempty" yaml:"collectionEntityIpv4Addr" bson:"collectionEntityIpv4Addr" mapstructure:"CollectionEntityIpv4Addr"`
	CollectionEntityIpv6Addr string     `json:"collectionEntityIpv6Addr,omitempty" yaml:"collectionEntityIpv6Addr" bson:"collectionEntityIpv6Addr" mapstructure:"CollectionEntityIpv6Addr"`
	InterfaceList            string     `json:"interfaceList,omitempty" yaml:"interfaceList" bson:"interfaceList" mapstructure:"InterfaceList"`
}

type SmsManagementSubscriptionData struct {
	SupportedFeatures   string   `json:"supportedFeatures,omitempty" yaml:"supportedFeatures" bson:"supportedFeatures" mapstructure:"SupportedFeatures"`
	MtSmsSubscribed     bool     `json:"mtSmsSubscribed,omitempty" yaml:"mtSmsSubscribed" bson:"mtSmsSubscribed" mapstructure:"MtSmsSubscribed"`
	MtSmsBarringAll     bool     `json:"mtSmsBarringAll,omitempty" yaml:"mtSmsBarringAll" bson:"mtSmsBarringAll" mapstructure:"MtSmsBarringAll"`
	MtSmsBarringRoaming bool     `json:"mtSmsBarringRoaming,omitempty" yaml:"mtSmsBarringRoaming" bson:"mtSmsBarringRoaming" mapstructure:"MtSmsBarringRoaming"`
	MoSmsSubscribed     bool     `json:"moSmsSubscribed,omitempty" yaml:"moSmsSubscribed" bson:"moSmsSubscribed" mapstructure:"MoSmsSubscribed"`
	MoSmsBarringAll     bool     `json:"moSmsBarringAll,omitempty" yaml:"moSmsBarringAll" bson:"moSmsBarringAll" mapstructure:"MoSmsBarringAll"`
	MoSmsBarringRoaming bool     `json:"moSmsBarringRoaming,omitempty" yaml:"moSmsBarringRoaming" bson:"moSmsBarringRoaming" mapstructure:"MoSmsBarringRoaming"`
	SharedSmsMngDataIds []string `json:"sharedSmsMngDataIds,omitempty" yaml:"sharedSmsMngDataIds" bson:"sharedSmsMngDataIds" mapstructure:"SharedSmsMngDataIds"`
}

type SmsSubscriptionData struct {
	SmsSubscribed       bool     `json:"smsSubscribed,omitempty" yaml:"smsSubscribed" bson:"smsSubscribed" mapstructure:"SmsSubscribed"`
	SharedSmsSubsDataId []string `json:"sharedSmsSubsDataId,omitempty" yaml:"sharedSmsSubsDataId" bson:"sharedSmsSubsDataId" mapstructure:"SharedSmsSubsDataId"`
}

type SharedData struct {
	SharedDataId            string                             `json:"sharedDataId" yaml:"sharedDataId" bson:"sharedDataId" mapstructure:"SharedDataId"`
	SharedAmData            *AccessAndMobilitySubscriptionData `json:"sharedAmData,omitempty" yaml:"sharedAmData" bson:"sharedAmData" mapstructure:"SharedAmData"`
	SharedSmsSubsData       *SmsSubscriptionData               `json:"sharedSmsSubsData,omitempty" yaml:"sharedSmsSubsData" bson:"sharedSmsSubsData" mapstructure:"SharedSmsSubsData"`
	SharedSmsMngSubsData    *SmsManagementSubscriptionData     `json:"sharedSmsMngSubsData,omitempty" yaml:"sharedSmsMngSubsData" bson:"sharedSmsMngSubsData" mapstructure:"SharedSmsMngSubsData"`
	SharedDnnConfigurations map[string]DnnConfiguration        `json:"sharedDnnConfigurations,omitempty" yaml:"sharedDnnConfigurations" bson:"sharedDnnConfigurations" mapstructure:"SharedDnnConfigurations"`
	SharedTraceData         *TraceData                         `json:"sharedTraceData,omitempty" yaml:"sharedTraceData" bson:"sharedTraceData" mapstructure:"SharedTraceData"`
	SharedSnssaiInfos       map[string]SnssaiInfo              `json:"sharedSnssaiInfos,omitempty" yaml:"sharedSnssaiInfos" bson:"sharedSnssaiInfos" mapstructure:"SharedSnssaiInfos"`
}

type Opc struct {
	OpcValue            string `json:"opcValue" bson:"opcValue"`
	EncryptionKey       int32  `json:"encryptionKey" bson:"encryptionKey"`
	EncryptionAlgorithm int32  `json:"encryptionAlgorithm" bson:"encryptionAlgorithm"`
}

type Tuak struct {
	Top              *Top  `json:"top,omitempty" bson:"top"`
	KeccakIterations int32 `json:"keccakIterations,omitempty" bson:"keccakIterations"`
}

type Milenage struct {
	Op        *Op        `json:"op,omitempty" bson:"op"`
	Rotations *Rotations `json:"rotations,omitempty" bson:"rotations"`
	Constants *Constants `json:"constants,omitempty" bson:"constants"`
}

type AuthenticationSubscription struct {
	AuthenticationMethod               AuthMethod      `json:"authenticationMethod" bson:"authenticationMethod"`
	PermanentKey                       *PermanentKey   `json:"permanentKey" bson:"permanentKey"`
	SequenceNumber                     string          `json:"sequenceNumber" bson:"sequenceNumber"`
	AuthenticationManagementField      string          `json:"authenticationManagementField,omitempty" bson:"authenticationManagementField"`
	VectorAlgorithm                    VectorAlgorithm `json:"vectorAlgorithm,omitempty" bson:"vectorAlgorithm"`
	Milenage                           *Milenage       `json:"milenage,omitempty" bson:"milenage"`
	Tuak                               *Tuak           `json:"tuak,omitempty" bson:"tuak"`
	Opc                                *Opc            `json:"opc,omitempty" bson:"opc"`
	Topc                               *Topc           `json:"topc,omitempty" bson:"topc"`
	SharedAuthenticationSubscriptionId *SharedData     `json:"sharedAuthenticationSubscriptionId,omitempty" bson:"sharedAuthenticationSubscriptionId"`
}

type RatType string

type Area struct {
	Tacs      []string `json:"tacs,omitempty" yaml:"tacs" bson:"tacs" mapstructure:"Tacs"`
	AreaCodes string   `json:"areaCodes,omitempty" yaml:"areaCodes" bson:"areaCodes" mapstructure:"AreaCodes"`
}

type AmPolicyData struct {
	UeId      string   `json:"ueId" yaml:"ueId" bson:"ueId" mapstructure:"UeId"`
	SubscCats []string `json:"subscCats,omitempty" bson:"subscCats"`
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

type FlowRule struct {
	Filter      string `json:"filter,omitempty" yaml:"filter" bson:"filter" mapstructure:"filter"`
	Snssai      string `json:"snssai,omitempty" yaml:"snssai" bson:"snssai" mapstructure:"snssai"`
	Dnn         string `json:"dnn,omitempty" yaml:"v" bson:"dnn" mapstructure:"dnn"`
	Var5QI      int    `json:"5qi,omitempty" yaml:"5qi" bson:"5qi" mapstructure:"5qi"`
	MBRUL       string `json:"mbrUL,omitempty" yaml:"mbrUL" bson:"mbrUL" mapstructure:"mbrUL"`
	MBRDL       string `json:"mbrDL,omitempty" yaml:"mbrDL" bson:"mbrDL" mapstructure:"mbrDL"`
	GBRUL       string `json:"gbrUL,omitempty" yaml:"gbrUL" bson:"gbrUL" mapstructure:"gbrUL"`
	GBRDL       string `json:"gbrDL,omitempty" yaml:"gbrDL" bson:"gbrDL" mapstructure:"gbrDL"`
	BitRateUnit string `json:"bitrate-unit,omitempty" yaml:"bitrate-unit" bson:"bitrate-unit" mapstructure:"bitrate-unit"`
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
	FlowRules                         []FlowRule                           `json:"FlowRules"`
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
	UeId                       string                      `json:"ueId"`
	ServingPlmnId              string                      `json:"servingPlmnId"`
	SingleNssai                *Snssai                     `json:"singleNssai" yaml:"singleNssai" bson:"singleNssai" mapstructure:"SingleNssai"`
	DnnConfigurations          map[string]DnnConfiguration `json:"dnnConfigurations,omitempty" yaml:"dnnConfigurations" bson:"dnnConfigurations" mapstructure:"DnnConfigurations"`
	InternalGroupIds           []string                    `json:"internalGroupIds,omitempty" yaml:"internalGroupIds" bson:"internalGroupIds" mapstructure:"InternalGroupIds"`
	SharedDnnConfigurationsIds string                      `json:"sharedDnnConfigurationsIds,omitempty" yaml:"sharedDnnConfigurationsIds" bson:"sharedDnnConfigurationsIds" mapstructure:"SharedDnnConfigurationsIds"`
}

const (
	UsageMonLevel_SESSION_LEVEL UsageMonLevel = "SESSION_LEVEL"
	UsageMonLevel_SERVICE_LEVEL UsageMonLevel = "SERVICE_LEVEL"
)

type Periodicity string

const (
	Periodicity_YEARLY  Periodicity = "YEARLY"
	Periodicity_MONTHLY Periodicity = "MONTHLY"
	Periodicity_WEEKLY  Periodicity = "WEEKLY"
	Periodicity_DAILY   Periodicity = "DAILY"
	Periodicity_HOURLY  Periodicity = "HOURLY"
)

type UsageMonLevel string

type UsageMonDataScope struct {
	Snssai *Snssai  `json:"snssai" bson:"snssai"`
	Dnn    []string `json:"dnn,omitempty" bson:"dnn"`
}

type TimePeriod struct {
	Period       Periodicity `json:"period" bson:"period"`
	MaxNumPeriod int32       `json:"maxNumPeriod,omitempty" bson:"maxNumPeriod"`
}

type UsageThreshold struct {
	Duration       int32 `json:"duration,omitempty" yaml:"duration" bson:"duration" mapstructure:"Duration"`
	TotalVolume    int64 `json:"totalVolume,omitempty" yaml:"totalVolume" bson:"totalVolume" mapstructure:"TotalVolume"`
	DownlinkVolume int64 `json:"downlinkVolume,omitempty" yaml:"downlinkVolume" bson:"downlinkVolume" mapstructure:"DownlinkVolume"`
	UplinkVolume   int64 `json:"uplinkVolume,omitempty" yaml:"uplinkVolume" bson:"uplinkVolume" mapstructure:"UplinkVolume"`
}

type UsageMonDataLimit struct {
	LimitId     string                       `json:"limitId" bson:"limitId"`
	Scopes      map[string]UsageMonDataScope `json:"scopes,omitempty" bson:"scopes"`
	UmLevel     UsageMonLevel                `json:"umLevel,omitempty" bson:"umLevel"`
	StartDate   *time.Time                   `json:"startDate,omitempty" bson:"startDate"`
	EndDate     *time.Time                   `json:"endDate,omitempty" bson:"endDate"`
	UsageLimit  *UsageThreshold              `json:"usageLimit,omitempty" bson:"usageLimit"`
	ResetPeriod *time.Time                   `json:"resetPeriod,omitempty" bson:"resetPeriod"`
}

type UsageMonData struct {
	LimitId      string                       `json:"limitId" bson:"limitId"`
	Scopes       map[string]UsageMonDataScope `json:"scopes,omitempty" bson:"scopes"`
	UmLevel      UsageMonLevel                `json:"umLevel,omitempty" bson:"umLevel"`
	AllowedUsage *UsageThreshold              `json:"allowedUsage,omitempty" bson:"allowedUsage"`
	ResetTime    *TimePeriod                  `json:"resetTime,omitempty" bson:"resetTime"`
}

type LimitIdToMonitoringKey struct {
	LimitId string   `json:"limitId" bson:"limitId"`
	Monkey  []string `json:"monkey,omitempty" bson:"monkey"`
}

type ChargingInformation struct {
	PrimaryChfAddress   string `json:"primaryChfAddress" yaml:"primaryChfAddress" bson:"primaryChfAddress" mapstructure:"PrimaryChfAddress"`
	SecondaryChfAddress string `json:"secondaryChfAddress" yaml:"secondaryChfAddress" bson:"secondaryChfAddress" mapstructure:"SecondaryChfAddress"`
}

type SmPolicyDnnData struct {
	Dnn                 string                            `json:"dnn" bson:"dnn"`
	AllowedServices     []string                          `json:"allowedServices,omitempty" bson:"allowedServices"`
	SubscCats           []string                          `json:"subscCats,omitempty" bson:"subscCats"`
	GbrUl               string                            `json:"gbrUl,omitempty" bson:"gbrUl"`
	GbrDl               string                            `json:"gbrDl,omitempty" bson:"gbrDl"`
	AdcSupport          bool                              `json:"adcSupport,omitempty" bson:"adcSupport"`
	SubscSpendingLimits bool                              `json:"subscSpendingLimits,omitempty" bson:"subscSpendingLimits"`
	Ipv4Index           int32                             `json:"ipv4Index,omitempty" bson:"ipv4Index"`
	Ipv6Index           int32                             `json:"ipv6Index,omitempty" bson:"ipv6Index"`
	Offline             bool                              `json:"offline,omitempty" bson:"offline"`
	Online              bool                              `json:"online,omitempty" bson:"online"`
	ChfInfo             *ChargingInformation              `json:"chfInfo,omitempty" bson:"chfInfo"`
	RefUmDataLimitIds   map[string]LimitIdToMonitoringKey `json:"refUmDataLimitIds,omitempty" bson:"refUmDataLimitIds"`
	MpsPriority         bool                              `json:"mpsPriority,omitempty" bson:"mpsPriority"`
	ImsSignallingPrio   bool                              `json:"imsSignallingPrio,omitempty" bson:"imsSignallingPrio"`
	MpsPriorityLevel    int32                             `json:"mpsPriorityLevel,omitempty" bson:"mpsPriorityLevel"`
}

type SmPolicySnssaiData struct {
	Snssai          *Snssai                    `json:"snssai" bson:"snssai"`
	SmPolicyDnnData map[string]SmPolicyDnnData `json:"smPolicyDnnData,omitempty" bson:"smPolicyDnnData"`
}

type SmPolicyData struct {
	UeId               string                        `json:"ueId"`
	SmPolicySnssaiData map[string]SmPolicySnssaiData `json:"smPolicySnssaiData" bson:"smPolicySnssaiData"`
	UmDataLimits       map[string]UsageMonDataLimit  `json:"umDataLimits,omitempty" bson:"umDataLimits"`
	UmData             map[string]UsageMonData       `json:"umData,omitempty" bson:"umData"`
}

type DnnInfo struct {
	Dnn                 string `json:"dnn" yaml:"dnn" bson:"dnn" mapstructure:"Dnn"`
	DefaultDnnIndicator bool   `json:"defaultDnnIndicator,omitempty" yaml:"defaultDnnIndicator" bson:"defaultDnnIndicator" mapstructure:"DefaultDnnIndicator"`
	LboRoamingAllowed   bool   `json:"lboRoamingAllowed,omitempty" yaml:"lboRoamingAllowed" bson:"lboRoamingAllowed" mapstructure:"LboRoamingAllowed"`
	IwkEpsInd           bool   `json:"iwkEpsInd,omitempty" yaml:"iwkEpsInd" bson:"iwkEpsInd" mapstructure:"IwkEpsInd"`
}

type SnssaiInfo struct {
	DnnInfos []DnnInfo `json:"dnnInfos" yaml:"dnnInfos" bson:"dnnInfos" mapstructure:"DnnInfos"`
}

type SmfSelectionSubscriptionData struct {
	UeId                  string                `json:"ueId"`
	ServingPlmnId         string                `json:"servingPlmnId"`
	SupportedFeatures     string                `json:"supportedFeatures,omitempty" yaml:"supportedFeatures" bson:"supportedFeatures" mapstructure:"SupportedFeatures"`
	SubscribedSnssaiInfos map[string]SnssaiInfo `json:"subscribedSnssaiInfos,omitempty" yaml:"subscribedSnssaiInfos" bson:"subscribedSnssaiInfos" mapstructure:"SubscribedSnssaiInfos"`
	SharedSnssaiInfosId   string                `json:"sharedSnssaiInfosId,omitempty" yaml:"sharedSnssaiInfosId" bson:"sharedSnssaiInfosId" mapstructure:"SharedSnssaiInfosId"`
}

type Snssai struct {
	Sst int32  `json:"sst" yaml:"sst" bson:"sst" mapstructure:"Sst"`
	Sd  string `json:"sd,omitempty" yaml:"sd" bson:"sd" mapstructure:"Sd"`
}
