package models

import (
	"time"
)

type SmContextCreateError struct {
	Error        *ProblemDetails  `json:"error"`
	N1SmMsg      *RefToBinaryData `json:"n1SmMsg,omitempty"`
	RecoveryTime *time.Time       `json:"recoveryTime,omitempty"`
}
