package models

import (
	"time"
)

type SmContextUpdateError struct {
	Error        *ProblemDetails  `json:"error"`
	N1SmMsg      *RefToBinaryData `json:"n1SmMsg,omitempty"`
	N2SmInfo     *RefToBinaryData `json:"n2SmInfo,omitempty"`
	N2SmInfoType N2SmInfoType     `json:"n2SmInfoType,omitempty"`
	UpCnxState   UpCnxState       `json:"upCnxState,omitempty"`
	RecoveryTime *time.Time       `json:"recoveryTime,omitempty"`
}
