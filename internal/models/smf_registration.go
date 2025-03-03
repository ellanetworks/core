package models

type SmfRegistration struct {
	SmfInstanceId string
	PduSessionId  int32
	Dnn           string
	PlmnId        *PlmnId
	PgwFqdn       string
}
