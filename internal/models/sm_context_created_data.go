package models

import (
	"time"
)

type SmContextCreatedData struct {
	HSmfUri      string           `json:"hSmfUri,omitempty"`
	PduSessionId int32            `json:"pduSessionId,omitempty"`
	SNssai       *Snssai          `json:"sNssai,omitempty"`
	UpCnxState   UpCnxState       `json:"upCnxState,omitempty"`
	N2SmInfo     *RefToBinaryData `json:"n2SmInfo,omitempty"`
	N2SmInfoType N2SmInfoType     `json:"n2SmInfoType,omitempty"`
	// AllocatedEbiList     []EbiArpMapping  `json:"allocatedEbiList,omitempty"`
	HoState              HoState    `json:"hoState,omitempty"`
	SmfServiceInstanceId string     `json:"smfServiceInstanceId,omitempty"`
	RecoveryTime         *time.Time `json:"recoveryTime,omitempty"`
	SupportedFeatures    string     `json:"supportedFeatures,omitempty"`
}
