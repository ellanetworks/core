package models

type PduSession struct {
	Dnn           string
	SmfInstanceID string
	PlmnID        *PlmnID
}
