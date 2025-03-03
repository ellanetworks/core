package models

type SmPolicyDecision struct {
	// A map of Sessionrules with the content being the SessionRule as described in subclause 5.6.2.7.
	SessRules map[string]*SessionRule
	// A map of PCC rules with the content being the PCCRule as described in subclause 5.6.2.6.
	PccRules map[string]*PccRule
	// Map of QoS data policy decisions.
	QosDecs map[string]*QosData
	// Map of Traffic Control data policy decisions.
	TraffContDecs map[string]*TrafficControlData
	// Indicates the offline charging is applicable to the PDU session or PCC rule.
	Offline bool
	// Indicates the online charging is applicable to the PDU session or PCC rule.
	Online bool
	// Defines the policy control request triggers subscribed by the PCF.
	PolicyCtrlReqTriggers []PolicyControlRequestTrigger
	Ipv4Index             int32
	Ipv6Index             int32
}
