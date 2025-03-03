package models

type N1MessageContainer struct {
	N1MessageClass   N1MessageClass
	N1MessageContent *RefToBinaryData
	NfId             string
}
