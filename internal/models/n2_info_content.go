package models

type N2InfoContent struct {
	NgapMessageType int32
	NgapIeType      NgapIeType
	NgapData        *RefToBinaryData
}
