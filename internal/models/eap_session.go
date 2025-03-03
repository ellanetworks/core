package models

type EapSession struct {
	EapPayload string
	KSeaf      string
	AuthResult AuthResult
	Supi       string
}
