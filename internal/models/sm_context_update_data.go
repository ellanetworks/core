package models

type SmContextUpdateData struct {
	Pei                string
	Gpsi               string
	ServingNfId        string
	Guami              *Guami
	ServingNetwork     *PlmnId
	AnType             AccessType
	RatType            RatType
	PresenceInLadn     PresenceState
	UeLocation         *UserLocation
	UeTimeZone         string
	AddUeLocation      *UserLocation
	UpCnxState         UpCnxState
	HoState            HoState
	ToBeSwitched       bool
	FailedToBeSwitched bool
	N1SmMsg            *RefToBinaryData
	N2SmInfo           *RefToBinaryData
	N2SmInfoType       N2SmInfoType
	TargetId           *NgRanTargetId
	TargetServingNfId  string
	SmContextStatusUri string
	DataForwarding     bool
	EpsBearerSetup     []string
	RevokeEbiList      []int32
	Release            bool
	Cause              Cause
	NgApCause          *NgApCause
	Var5gMmCauseValue  int32
	SNssai             *Snssai
	AnTypeCanBeChanged bool
}
