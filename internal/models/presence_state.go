package models

type PresenceState string

// List of PresenceState
const (
	PresenceState_IN_AREA     PresenceState = "IN_AREA"
	PresenceState_OUT_OF_AREA PresenceState = "OUT_OF_AREA"
	PresenceState_UNKNOWN     PresenceState = "UNKNOWN"
	PresenceState_INACTIVE    PresenceState = "INACTIVE"
)
