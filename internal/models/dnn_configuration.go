package models

type DnnConfiguration struct {
	PduSessionTypes *PduSessionTypes
	SscModes        *SscModes
	Var5gQosProfile *SubscribedDefaultQos
	SessionAmbr     *Ambr
}
