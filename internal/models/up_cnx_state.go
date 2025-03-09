package models

type UpCnxState string

const (
	UpCnxStateActivated   UpCnxState = "ACTIVATED"
	UpCnxStateDeactivated UpCnxState = "DEACTIVATED"
	UpCnxStateActivating  UpCnxState = "ACTIVATING"
)
