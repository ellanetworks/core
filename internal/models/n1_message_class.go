package models

type N1MessageClass string

const (
	N1MessageClassSM   N1MessageClass = "SM"
	N1MessageClassLPP  N1MessageClass = "LPP"
	N1MessageClassSMS  N1MessageClass = "SMS"
	N1MessageClassUPDP N1MessageClass = "UPDP"
)
