package models

type N2SmInformation struct {
	PduSessionID  int32
	N2InfoContent *N2InfoContent
	SNssai        *Snssai
}
