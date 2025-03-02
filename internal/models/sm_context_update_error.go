package models

import (
	"time"
)

type SmContextUpdateError struct {
	Error        *ProblemDetails
	N1SmMsg      *RefToBinaryData
	N2SmInfo     *RefToBinaryData
	N2SmInfoType N2SmInfoType
	UpCnxState   UpCnxState
	RecoveryTime *time.Time
}
