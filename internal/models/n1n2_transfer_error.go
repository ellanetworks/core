package models

type N1N2MessageTransferError struct {
	Error   *ProblemDetails
	ErrInfo *N1N2MsgTxfrErrDetail
}
