package models

type GlobalRanNodeId struct {
	PlmnId  *PlmnId
	N3IwfId string
	GnbID   *GnbID
	NgeNbId string
}
