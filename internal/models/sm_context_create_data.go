package models

type SmContextCreateData struct {
	Supi                string
	UnauthenticatedSupi bool
	Pei                 string
	Gpsi                string
	PduSessionId        int32
	Dnn                 string
	SNssai              *Snssai
	HplmnSnssai         *Snssai
	ServingNfId         string
	Guami               *Guami
	ServingNetwork      *PlmnId
	N1SmMsg             *RefToBinaryData
	AnType              AccessType
	RatType             RatType
	PresenceInLadn      PresenceState
	UeLocation          *UserLocation
	UeTimeZone          string
	AddUeLocation       *UserLocation
	SmContextStatusUri  string
	OldPduSessionId     int32
}
