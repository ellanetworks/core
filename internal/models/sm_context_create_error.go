package models

import (
	"time"
)

type SmContextCreateError struct {
	Error        *ProblemDetails
	N1SmMsg      *RefToBinaryData
	RecoveryTime *time.Time
}
