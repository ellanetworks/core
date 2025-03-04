package models

type SmContextUpdateError struct {
	N1SmMsg      *RefToBinaryData
	N2SmInfo     *RefToBinaryData
	N2SmInfoType N2SmInfoType
	UpCnxState   UpCnxState
}
