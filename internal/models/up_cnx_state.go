package models

type UpCnxState string

const (
	UpCnxState_ACTIVATED   UpCnxState = "ACTIVATED"
	UpCnxState_DEACTIVATED UpCnxState = "DEACTIVATED"
	UpCnxState_ACTIVATING  UpCnxState = "ACTIVATING"
)
