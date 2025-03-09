package models

type SmPolicyDecision struct {
	// A map of Sessionrules with the content being the SessionRule as described in subclause 5.6.2.7.
	SessRules map[string]*SessionRule
	// A map of PCC rules with the content being the PCCRule as described in subclause 5.6.2.6.
	PccRules map[string]*PccRule
	// Map of QoS data policy decisions.
	QosDecs map[string]*QosData
}
