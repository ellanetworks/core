package models

type PresenceState string

const (
	PresenceStateInArea    PresenceState = "IN_AREA"
	PresenceStateOutOfArea PresenceState = "OUT_OF_AREA"
)
