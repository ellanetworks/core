package models

type PduSessionContext struct {
	PduSessionId int32
	SmContextRef string
	SNssai       *Snssai
	Dnn          string
	AccessType   AccessType
	HsmfId       string
	VsmfId       string
	NsInstance   string
}
