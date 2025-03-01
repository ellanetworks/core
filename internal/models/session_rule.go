package models

type SessionRule struct {
	AuthSessAmbr *Ambr
	AuthDefQos   *AuthorizedDefaultQos
	// Univocally identifies the session rule within a PDU session.
	SessRuleId string
	// A reference to UsageMonitoringData policy decision type. It is the umId described in subclause 5.6.2.12.
	RefUmData string
	// A reference to the condition data. It is the condId described in subclause 5.6.2.9.
	RefCondData string
}
