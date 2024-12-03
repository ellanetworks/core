package db

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
