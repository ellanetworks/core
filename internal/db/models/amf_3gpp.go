package models

const (
	ImsVoPs_HOMOGENEOUS_SUPPORT        ImsVoPs = "HOMOGENEOUS_SUPPORT"
	ImsVoPs_HOMOGENEOUS_NON_SUPPORT    ImsVoPs = "HOMOGENEOUS_NON_SUPPORT"
	ImsVoPs_NON_HOMOGENEOUS_OR_UNKNOWN ImsVoPs = "NON_HOMOGENEOUS_OR_UNKNOWN"
)

type ImsVoPs string

type PlmnId struct {
	Mcc string `json:"mcc" yaml:"mcc" bson:"mcc" mapstructure:"Mcc"`
	Mnc string `json:"mnc" yaml:"mnc" bson:"mnc" mapstructure:"Mnc"`
}

type Guami struct {
	PlmnId *PlmnId `json:"plmnId" yaml:"plmnId" bson:"plmnId" mapstructure:"PlmnId"`
	AmfId  string  `json:"amfId" yaml:"amfId" bson:"amfId" mapstructure:"AmfId"`
}

type Amf3GPP struct {
	InitialRegistrationInd bool    `json:"initialRegistrationInd,omitempty"`
	Guami                  Guami   `json:"guami,omitempty"`
	RatType                string  `json:"ratType,omitempty"`
	UeId                   string  `json:"ueId,omitempty"`
	AmfInstanceId          string  `json:"amfInstanceId,omitempty"`
	ImsVoPs                ImsVoPs `json:"imsVoPs,omitempty"`
	DeregCallbackUri       string  `json:"deregCallbackUri,omitempty"`
}
