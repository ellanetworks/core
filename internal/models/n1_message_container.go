package models

type N1MessageContainer struct {
	N1MessageClass   N1MessageClass   `json:"n1MessageClass"`
	N1MessageContent *RefToBinaryData `json:"n1MessageContent"`
	NfId             string           `json:"nfId,omitempty"`
}
