package models

import (
	"time"
)

type SmContextCreatedData struct {
	HSmfUri              string
	PduSessionId         int32
	SNssai               *Snssai
	UpCnxState           UpCnxState
	N2SmInfo             *RefToBinaryData
	N2SmInfoType         N2SmInfoType
	HoState              HoState
	SmfServiceInstanceId string
	RecoveryTime         *time.Time
	SupportedFeatures    string
}
