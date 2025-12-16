package models

type SmContextCreateData struct {
	Supi         string
	PduSessionID uint8
	Dnn          string
	SNssai       *Snssai
}
