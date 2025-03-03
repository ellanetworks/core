package models

type SmContextUpdateError struct {
	Error        *ProblemDetails
	N1SmMsg      *RefToBinaryData
	N2SmInfo     *RefToBinaryData
	N2SmInfoType N2SmInfoType
	UpCnxState   UpCnxState
}
