package models

type UeContextTransferRspData struct {
	UeContext         *UeContext
	UeRadioCapability *N2InfoContent
	SupportedFeatures string
}
