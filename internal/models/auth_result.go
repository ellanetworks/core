package models

type AuthResult string

const (
	AuthResultSuccess AuthResult = "AUTHENTICATION_SUCCESS"
	AuthResultFailure AuthResult = "AUTHENTICATION_FAILURE"
	AuthResultOngoing AuthResult = "AUTHENTICATION_ONGOING"
)
