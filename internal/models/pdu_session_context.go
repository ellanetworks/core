package models

type PduSessionContext struct {
	PduSessionID int32
	SmContextRef string
	SNssai       *Snssai
	Dnn          string
	AccessType   AccessType
	HsmfID       string
	VsmfID       string
	NsInstance   string
}
