package models

type N2InfoContent struct {
	NgapMessageType int32            `json:"ngapMessageType,omitempty"`
	NgapIeType      NgapIeType       `json:"ngapIeType"`
	NgapData        *RefToBinaryData `json:"ngapData"`
}
