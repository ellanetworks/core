package models

type N2InformationClass string

// List of N2InformationClass
const (
	N2InformationClass_SM       N2InformationClass = "SM"
	N2InformationClass_NRP_PA   N2InformationClass = "NRPPa"
	N2InformationClass_PWS      N2InformationClass = "PWS"
	N2InformationClass_PWS_BCAL N2InformationClass = "PWS-BCAL" // #nosec G101
	N2InformationClass_PWS_RF   N2InformationClass = "PWS-RF"
	N2InformationClass_RAN      N2InformationClass = "RAN"
)
