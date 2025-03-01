package models

type PccRule struct {
	// An array of IP flow packet filter information.
	FlowInfos []FlowInformation
	// A reference to the application detection filter configured at the UPF.
	AppId string
	// Represents the content version of some content.
	ContVer int32
	// Univocally identifies the PCC rule within a PDU session.
	PccRuleId  string
	Precedence int32
	// AfSigProtocol AfSigProtocol `json:"afSigProtocol,omitempty" yaml:"afSigProtocol" bson:"afSigProtocol" mapstructure:"AfSigProtocol"`
	// Indication of application relocation possibility.
	AppReloc bool
	// A reference to the QoSData policy type decision type. It is the qosId described in subclause 5.6.2.8. (NOTE)
	RefQosData []string
	// A reference to the TrafficControlData policy decision type. It is the tcId described in subclause 5.6.2.10. (NOTE)
	RefTcData []string
	// A reference to the ChargingData policy decision type. It is the chgId described in subclause 5.6.2.11. (NOTE)
	RefChgData []string
	// A reference to UsageMonitoringData policy decision type. It is the umId described in subclause 5.6.2.12. (NOTE)
	RefUmData []string
	// A reference to the condition data. It is the condId described in subclause 5.6.2.9.
	RefCondData string
}
