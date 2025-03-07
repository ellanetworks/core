package models

type SmContextCreateData struct {
	Supi                string
	UnauthenticatedSupi bool
	Pei                 string
	Gpsi                string
	PduSessionID        int32
	Dnn                 string
	SNssai              *Snssai
	ServingNfID         string
	Guami               *Guami
	ServingNetwork      *PlmnID
	N1SmMsg             *RefToBinaryData
	AnType              AccessType
	RatType             RatType
	PresenceInLadn      PresenceState
	UeLocation          *UserLocation
	UeTimeZone          string
	SmContextStatusUri  string
	OldPduSessionID     int32
}
