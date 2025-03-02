package models

import (
	"time"
)

type ConditionData struct {
	// Uniquely identifies the condition data within a PDU session.
	CondId           string
	ActivationTime   *time.Time
	DeactivationTime *time.Time
}
