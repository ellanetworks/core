package models

type PccRule struct {
	// An array of IP flow packet filter information.
	FlowInfos []FlowInformation
	// Univocally identifies the PCC rule within a PDU session.
	PccRuleID  string
	Precedence int32
	// A reference to the QoSData policy type decision type. It is the qosId described in subclause 5.6.2.8. (NOTE)
	RefQosData []string
	// A reference to the TrafficControlData policy decision type. It is the tcId described in subclause 5.6.2.10. (NOTE)
	RefTcData []string
}
