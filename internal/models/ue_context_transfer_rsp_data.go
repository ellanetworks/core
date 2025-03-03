package models

type UeContextTransferRspData struct {
	UeContext         *UeContext     `json:"ueContext"`
	UeRadioCapability *N2InfoContent `json:"ueRadioCapability,omitempty"`
	SupportedFeatures string         `json:"supportedFeatures,omitempty"`
}
