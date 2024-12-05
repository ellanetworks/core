package models

const (
	ImsVoPs_HOMOGENEOUS_SUPPORT        ImsVoPs = "HOMOGENEOUS_SUPPORT"
	ImsVoPs_HOMOGENEOUS_NON_SUPPORT    ImsVoPs = "HOMOGENEOUS_NON_SUPPORT"
	ImsVoPs_NON_HOMOGENEOUS_OR_UNKNOWN ImsVoPs = "NON_HOMOGENEOUS_OR_UNKNOWN"
)

// List of ServiceName
const (
	ServiceName_NNRF_NFM                  ServiceName = "nnrf-nfm"
	ServiceName_NNRF_DISC                 ServiceName = "nnrf-disc"
	ServiceName_NUDM_SDM                  ServiceName = "nudm-sdm"
	ServiceName_NUDM_UECM                 ServiceName = "nudm-uecm"
	ServiceName_NUDM_UEAU                 ServiceName = "nudm-ueau"
	ServiceName_NUDM_EE                   ServiceName = "nudm-ee"
	ServiceName_NUDM_PP                   ServiceName = "nudm-pp"
	ServiceName_NAMF_COMM                 ServiceName = "namf-comm"
	ServiceName_NAMF_EVTS                 ServiceName = "namf-evts"
	ServiceName_NAMF_MT                   ServiceName = "namf-mt"
	ServiceName_NAMF_LOC                  ServiceName = "namf-loc"
	ServiceName_NSMF_PDUSESSION           ServiceName = "nsmf-pdusession"
	ServiceName_NSMF_EVENT_EXPOSURE       ServiceName = "nsmf-event-exposure"
	ServiceName_NAUSF_AUTH                ServiceName = "nausf-auth"
	ServiceName_NAUSF_SORPROTECTION       ServiceName = "nausf-sorprotection"
	ServiceName_NAUSF_UPUPROTECTION       ServiceName = "nausf-upuprotection"
	ServiceName_NNEF_PFDMANAGEMENT        ServiceName = "nnef-pfdmanagement"
	ServiceName_NPCF_AM_POLICY_CONTROL    ServiceName = "npcf-am-policy-control"
	ServiceName_NPCF_SMPOLICYCONTROL      ServiceName = "npcf-smpolicycontrol"
	ServiceName_NPCF_POLICYAUTHORIZATION  ServiceName = "npcf-policyauthorization"
	ServiceName_NPCF_BDTPOLICYCONTROL     ServiceName = "npcf-bdtpolicycontrol"
	ServiceName_NPCF_EVENTEXPOSURE        ServiceName = "npcf-eventexposure"
	ServiceName_NPCF_UE_POLICY_CONTROL    ServiceName = "npcf-ue-policy-control"
	ServiceName_NSMSF_SMS                 ServiceName = "nsmsf-sms"
	ServiceName_NNSSF_NSSELECTION         ServiceName = "nnssf-nsselection"
	ServiceName_NNSSF_NSSAIAVAILABILITY   ServiceName = "nnssf-nssaiavailability"
	ServiceName_NUDR_DR                   ServiceName = "nudr-dr"
	ServiceName_NLMF_LOC                  ServiceName = "nlmf-loc"
	ServiceName_N5G_EIR_EIC               ServiceName = "n5g-eir-eic"
	ServiceName_NBSF_MANAGEMENT           ServiceName = "nbsf-management"
	ServiceName_NCHF_SPENDINGLIMITCONTROL ServiceName = "nchf-spendinglimitcontrol"
	ServiceName_NCHF_CONVERGEDCHARGING    ServiceName = "nchf-convergedcharging"
	ServiceName_NNWDAF_EVENTSSUBSCRIPTION ServiceName = "nnwdaf-eventssubscription"
	ServiceName_NNWDAF_ANALYTICSINFO      ServiceName = "nnwdaf-analyticsinfo"
)

type ServiceName string

type ImsVoPs string

type PlmnId struct {
	Mcc string `json:"mcc" yaml:"mcc" bson:"mcc" mapstructure:"Mcc"`
	Mnc string `json:"mnc" yaml:"mnc" bson:"mnc" mapstructure:"Mnc"`
}

type Guami struct {
	PlmnId *PlmnId `json:"plmnId" yaml:"plmnId" bson:"plmnId" mapstructure:"PlmnId"`
	AmfId  string  `json:"amfId" yaml:"amfId" bson:"amfId" mapstructure:"AmfId"`
}

type BackupAmfInfo struct {
	BackupAmf string  `json:"backupAmf" yaml:"backupAmf" bson:"backupAmf" mapstructure:"BackupAmf"`
	GuamiList []Guami `json:"guamiList,omitempty" yaml:"guamiList" bson:"guamiList" mapstructure:"GuamiList"`
}

type Amf3GppAccessRegistration struct {
	AmfInstanceId     string  `json:"amfInstanceId" bson:"amfInstanceId"`
	SupportedFeatures string  `json:"supportedFeatures,omitempty" bson:"supportedFeatures"`
	PurgeFlag         bool    `json:"purgeFlag,omitempty" bson:"purgeFlag"`
	Pei               string  `json:"pei,omitempty" bson:"pei"`
	ImsVoPs           ImsVoPs `json:"imsVoPs,omitempty" bson:"imsVoPs"`
	// string providing an URI formatted according to IETF RFC 3986.
	DeregCallbackUri    string      `json:"deregCallbackUri" bson:"deregCallbackUri"`
	AmfServiceNameDereg ServiceName `json:"amfServiceNameDereg,omitempty" bson:"amfServiceNameDereg"`
	// string providing an URI formatted according to IETF RFC 3986.
	PcscfRestorationCallbackUri string          `json:"pcscfRestorationCallbackUri,omitempty" bson:"pcscfRestorationCallbackUri"`
	AmfServiceNamePcscfRest     ServiceName     `json:"amfServiceNamePcscfRest,omitempty" bson:"amfServiceNamePcscfRest"`
	InitialRegistrationInd      bool            `json:"initialRegistrationInd,omitempty" bson:"initialRegistrationInd"`
	Guami                       *Guami          `json:"guami" bson:"guami"`
	BackupAmfInfo               []BackupAmfInfo `json:"backupAmfInfo,omitempty" bson:"backupAmfInfo"`
	DrFlag                      bool            `json:"drFlag,omitempty" bson:"drFlag"`
	RatType                     RatType         `json:"ratType" bson:"ratType"`
}
