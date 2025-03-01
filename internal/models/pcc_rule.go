package models

type PccRule struct {
	// An array of IP flow packet filter information.
	FlowInfos []FlowInformation `json:"flowInfos,omitempty" yaml:"flowInfos" bson:"flowInfos" mapstructure:"FlowInfos"`
	// A reference to the application detection filter configured at the UPF.
	AppId string `json:"appId,omitempty" yaml:"appId" bson:"appId" mapstructure:"AppId"`
	// Represents the content version of some content.
	ContVer int32 `json:"contVer,omitempty" yaml:"contVer" bson:"contVer" mapstructure:"ContVer"`
	// Univocally identifies the PCC rule within a PDU session.
	PccRuleId  string `json:"pccRuleId" yaml:"pccRuleId" bson:"pccRuleId" mapstructure:"PccRuleId"`
	Precedence int32  `json:"precedence,omitempty" yaml:"precedence" bson:"precedence" mapstructure:"Precedence"`
	// AfSigProtocol AfSigProtocol `json:"afSigProtocol,omitempty" yaml:"afSigProtocol" bson:"afSigProtocol" mapstructure:"AfSigProtocol"`
	// Indication of application relocation possibility.
	AppReloc bool `json:"appReloc,omitempty" yaml:"appReloc" bson:"appReloc" mapstructure:"AppReloc"`
	// A reference to the QoSData policy type decision type. It is the qosId described in subclause 5.6.2.8. (NOTE)
	RefQosData []string `json:"refQosData,omitempty" yaml:"refQosData" bson:"refQosData" mapstructure:"RefQosData"`
	// A reference to the TrafficControlData policy decision type. It is the tcId described in subclause 5.6.2.10. (NOTE)
	RefTcData []string `json:"refTcData,omitempty" yaml:"refTcData" bson:"refTcData" mapstructure:"RefTcData"`
	// A reference to the ChargingData policy decision type. It is the chgId described in subclause 5.6.2.11. (NOTE)
	RefChgData []string `json:"refChgData,omitempty" yaml:"refChgData" bson:"refChgData" mapstructure:"RefChgData"`
	// A reference to UsageMonitoringData policy decision type. It is the umId described in subclause 5.6.2.12. (NOTE)
	RefUmData []string `json:"refUmData,omitempty" yaml:"refUmData" bson:"refUmData" mapstructure:"RefUmData"`
	// A reference to the condition data. It is the condId described in subclause 5.6.2.9.
	RefCondData string `json:"refCondData,omitempty" yaml:"refCondData" bson:"refCondData" mapstructure:"RefCondData"`
}
