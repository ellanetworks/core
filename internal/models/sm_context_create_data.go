package models

type SmContextCreateData struct {
	Supi         string
	PduSessionID int32
	Dnn          string
	SNssai       *Snssai
	UeLocation   *UserLocation
}
