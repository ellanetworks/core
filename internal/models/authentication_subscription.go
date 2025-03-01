package models

type AuthenticationSubscription struct {
	AuthenticationMethod          AuthMethod
	PermanentKey                  *PermanentKey
	SequenceNumber                string
	AuthenticationManagementField string
	Milenage                      *Milenage
	Opc                           *Opc
}
