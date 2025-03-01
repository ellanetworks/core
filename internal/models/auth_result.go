package models

type AuthResult string

const (
	AuthResult_SUCCESS AuthResult = "AUTHENTICATION_SUCCESS"
	AuthResult_FAILURE AuthResult = "AUTHENTICATION_FAILURE"
	AuthResult_ONGOING AuthResult = "AUTHENTICATION_ONGOING"
)
