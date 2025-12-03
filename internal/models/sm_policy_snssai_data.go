package models

// Contains the SM policy data for a given subscriber and S-NSSAI.
type SmPolicySnssaiData struct {
	Snssai          *Snssai
	SmPolicyDnnData SmPolicyDnnData
}
