package models

type SmContextUpdatedData struct {
	UpCnxState   UpCnxState
	HoState      HoState
	N1SmMsg      *RefToBinaryData
	N2SmInfo     *RefToBinaryData
	N2SmInfoType N2SmInfoType
}
