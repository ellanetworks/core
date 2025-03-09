package models

type SessionRule struct {
	AuthSessAmbr *Ambr
	AuthDefQos   *AuthorizedDefaultQos
	// Univocally identifies the session rule within a PDU session.
	SessRuleID string
}
