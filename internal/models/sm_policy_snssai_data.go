package models

// Contains the SM policy data for a given subscriber and S-NSSAI.
type SmPolicySnssaiData struct {
	Snssai          *Snssai                    `json:"snssai" bson:"snssai"`
	SmPolicyDnnData map[string]SmPolicyDnnData `json:"smPolicyDnnData,omitempty" bson:"smPolicyDnnData"`
}
