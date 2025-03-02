package models

type HoState string

// List of HoState
const (
	HoState_NONE      HoState = "NONE"
	HoState_PREPARING HoState = "PREPARING"
	HoState_PREPARED  HoState = "PREPARED"
	HoState_COMPLETED HoState = "COMPLETED"
	HoState_CANCELLED HoState = "CANCELLED"
)
