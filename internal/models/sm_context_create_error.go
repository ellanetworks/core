package models

type SmContextCreateError struct {
	Error   *ProblemDetails
	N1SmMsg *RefToBinaryData
}
