package models

type N2SmInformation struct {
	PduSessionId  int32          `json:"pduSessionId"`
	N2InfoContent *N2InfoContent `json:"n2InfoContent,omitempty"`
	SNssai        *Snssai        `json:"sNssai,omitempty"`
	SubjectToHo   bool           `json:"subjectToHo,omitempty"`
}
