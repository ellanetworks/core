package models

type HoState string

const (
	HoState_PREPARING HoState = "PREPARING"
	HoState_PREPARED  HoState = "PREPARED"
	HoState_COMPLETED HoState = "COMPLETED"
	HoState_CANCELLED HoState = "CANCELLED"
)
