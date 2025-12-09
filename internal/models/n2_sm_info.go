package models

type N2SmInformation struct {
	PduSessionID int32
	NgapIeType   NgapIeType
	SNssai       *Snssai
}
