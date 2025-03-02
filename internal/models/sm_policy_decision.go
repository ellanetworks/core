package models

import (
	"time"
)

type SmPolicyDecision struct {
	// A map of Sessionrules with the content being the SessionRule as described in subclause 5.6.2.7.
	SessRules map[string]*SessionRule
	// A map of PCC rules with the content being the PCCRule as described in subclause 5.6.2.6.
	PccRules map[string]*PccRule
	// If it is included and set to true, it indicates the P-CSCF Restoration is requested.
	PcscfRestIndication bool
	// Map of QoS data policy decisions.
	QosDecs map[string]*QosData
	// Map of Traffic Control data policy decisions.
	TraffContDecs      map[string]*TrafficControlData
	ReflectiveQoSTimer int32
	// A map of condition data with the content being as described in subclause 5.6.2.9.
	Conds            map[string]*ConditionData
	RevalidationTime *time.Time
	// Indicates the offline charging is applicable to the PDU session or PCC rule.
	Offline bool
	// Indicates the online charging is applicable to the PDU session or PCC rule.
	Online bool
	// Defines the policy control request triggers subscribed by the PCF.
	PolicyCtrlReqTriggers []PolicyControlRequestTrigger
	Ipv4Index             int32
	Ipv6Index             int32
	QosFlowUsage          QosFlowUsage
	SuppFeat              string
}
