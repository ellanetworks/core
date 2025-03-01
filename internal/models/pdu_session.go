package models

type PduSession struct {
	Dnn           string
	SmfInstanceId string
	PlmnId        *PlmnId
}
