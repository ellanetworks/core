package models

type PccRule struct {
	// An array of IP flow packet filter information.
	FlowInfos []FlowInformation
	// Univocally identifies the PCC rule within a PDU session.
	PccRuleID  string
	Precedence int32
	// A reference to the QoSData policy type decision type. It is the qosID described in subclause 5.6.2.8. (NOTE)
	RefQosData []string
}
