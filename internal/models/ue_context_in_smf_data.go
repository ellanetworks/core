package models

type UeContextInSmfData struct {
	// A map (list of key-value pairs where PduSessionId serves as key) of PduSessions
	PduSessions map[string]PduSession
	PgwInfo     []PgwInfo
}
