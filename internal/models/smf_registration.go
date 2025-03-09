package models

type SmfRegistration struct {
	SmfInstanceID string
	PduSessionID  int32
	Dnn           string
	PlmnID        *PlmnID
	PgwFqdn       string
}
