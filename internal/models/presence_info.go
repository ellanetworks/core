package models

type PresenceInfo struct {
	PraId            string
	PresenceState    PresenceState
	TrackingAreaList []Tai
}
