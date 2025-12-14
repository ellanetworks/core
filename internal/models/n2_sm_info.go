package models

type N2SmInformation struct {
	PduSessionID int32
	NgapIeType   N2SmInfoType
	SNssai       *Snssai
}
