package models

type SmfRegistration struct {
	PduSessionID int32
	Dnn          string
	PlmnID       *PlmnID
	PgwFqdn      string
}
