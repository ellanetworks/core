package models

// Contains the SM policy data for a given subscriber.
type SmPolicyData struct {
	SmPolicySnssaiData map[string]SmPolicySnssaiData `json:"smPolicySnssaiData" bson:"smPolicySnssaiData"`
	// UmDataLimits       map[string]UsageMonDataLimit  `json:"umDataLimits,omitempty" bson:"umDataLimits"`
	// UmData             map[string]UsageMonData       `json:"umData,omitempty" bson:"umData"`
}
