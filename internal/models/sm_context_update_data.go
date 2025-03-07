package models

type SmContextUpdateData struct {
	Pei                string
	Gpsi               string
	ServingNfId        string
	Guami              *Guami
	ServingNetwork     *PlmnID
	AnType             AccessType
	RatType            RatType
	PresenceInLadn     PresenceState
	UeLocation         *UserLocation
	AddUeLocation      *UserLocation
	UpCnxState         UpCnxState
	HoState            HoState
	ToBeSwitched       bool
	FailedToBeSwitched bool
	N1SmMsg            *RefToBinaryData
	N2SmInfo           *RefToBinaryData
	N2SmInfoType       N2SmInfoType
	TargetId           *NgRanTargetID
	TargetServingNfId  string
	SmContextStatusUri string
	Release            bool
	Cause              Cause
	NgApCause          *NgApCause
	Var5gMmCauseValue  int32
	AnTypeCanBeChanged bool
}
