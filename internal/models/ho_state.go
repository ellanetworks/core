package models

type HoState string

const (
	HoStatePreparing HoState = "PREPARING"
	HoStatePrepared  HoState = "PREPARED"
	HoStateCompleted HoState = "COMPLETED"
	HoStateCancelled HoState = "CANCELLED"
)
