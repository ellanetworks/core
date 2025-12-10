package models

type AuthenticationSubscription struct {
	// AuthenticationMethod          AuthType
	PermanentKey                  *PermanentKey
	SequenceNumber                string
	AuthenticationManagementField string
	Milenage                      *Milenage
	Opc                           *Opc
}
