package models

type N2SmInformation struct {
	PduSessionId  int32
	N2InfoContent *N2InfoContent
	SNssai        *Snssai
	SubjectToHo   bool
}
