package models

type N1N2MessageTransferError struct {
	Error   *ProblemDetails       `json:"error"`
	ErrInfo *N1N2MsgTxfrErrDetail `json:"errInfo,omitempty"`
}
