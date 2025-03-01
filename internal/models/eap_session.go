package models

type EapSession struct {
	// contains an EAP packet
	EapPayload string
	KSeaf      string
	Links      map[string]LinksValueSchema
	AuthResult AuthResult
	Supi       string
}
