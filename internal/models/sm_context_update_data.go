package models

type SmContextUpdateData struct {
	Pei            string  `json:"pei,omitempty"`
	Gpsi           string  `json:"gpsi,omitempty"`
	ServingNfId    string  `json:"servingNfId,omitempty"`
	Guami          *Guami  `json:"guami,omitempty"`
	ServingNetwork *PlmnId `json:"servingNetwork,omitempty"`
	// BackupAmfInfo      []BackupAmfInfo           `json:"backupAmfInfo,omitempty"`
	AnType             AccessType       `json:"anType,omitempty"`
	RatType            RatType          `json:"ratType,omitempty"`
	PresenceInLadn     PresenceState    `json:"presenceInLadn,omitempty"`
	UeLocation         *UserLocation    `json:"ueLocation,omitempty"`
	UeTimeZone         string           `json:"ueTimeZone,omitempty"`
	AddUeLocation      *UserLocation    `json:"addUeLocation,omitempty"`
	UpCnxState         UpCnxState       `json:"upCnxState,omitempty"`
	HoState            HoState          `json:"hoState,omitempty"`
	ToBeSwitched       bool             `json:"toBeSwitched,omitempty"`
	FailedToBeSwitched bool             `json:"failedToBeSwitched,omitempty"`
	N1SmMsg            *RefToBinaryData `json:"n1SmMsg,omitempty"`
	N2SmInfo           *RefToBinaryData `json:"n2SmInfo,omitempty"`
	N2SmInfoType       N2SmInfoType     `json:"n2SmInfoType,omitempty"`
	TargetId           *NgRanTargetId   `json:"targetId,omitempty"`
	TargetServingNfId  string           `json:"targetServingNfId,omitempty"`
	SmContextStatusUri string           `json:"smContextStatusUri,omitempty"`
	DataForwarding     bool             `json:"dataForwarding,omitempty"`
	EpsBearerSetup     []string         `json:"epsBearerSetup,omitempty"`
	RevokeEbiList      []int32          `json:"revokeEbiList,omitempty"`
	Release            bool             `json:"release,omitempty"`
	Cause              Cause            `json:"cause,omitempty"`
	NgApCause          *NgApCause       `json:"ngApCause,omitempty"`
	Var5gMmCauseValue  int32            `json:"5gMmCauseValue,omitempty"`
	SNssai             *Snssai          `json:"sNssai,omitempty"`
	// TraceData          *TraceData                `json:"traceData,omitempty"`
	// EpsInterworkingInd EpsInterworkingIndication `json:"epsInterworkingInd,omitempty"`
	AnTypeCanBeChanged bool `json:"anTypeCanBeChanged,omitempty"`
}
