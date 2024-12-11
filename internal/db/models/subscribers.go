package models

type FlowRule struct {
	Filter      string `json:"filter,omitempty" yaml:"filter" bson:"filter" mapstructure:"filter"`
	Snssai      string `json:"snssai,omitempty" yaml:"snssai" bson:"snssai" mapstructure:"snssai"`
	Dnn         string `json:"dnn,omitempty" yaml:"v" bson:"dnn" mapstructure:"dnn"`
	Var5QI      int    `json:"5qi,omitempty" yaml:"5qi" bson:"5qi" mapstructure:"5qi"`
	MBRUL       string `json:"mbrUL,omitempty" yaml:"mbrUL" bson:"mbrUL" mapstructure:"mbrUL"`
	MBRDL       string `json:"mbrDL,omitempty" yaml:"mbrDL" bson:"mbrDL" mapstructure:"mbrDL"`
	GBRUL       string `json:"gbrUL,omitempty" yaml:"gbrUL" bson:"gbrUL" mapstructure:"gbrUL"`
	GBRDL       string `json:"gbrDL,omitempty" yaml:"gbrDL" bson:"gbrDL" mapstructure:"gbrDL"`
	BitRateUnit string `json:"bitrate-unit,omitempty" yaml:"bitrate-unit" bson:"bitrate-unit" mapstructure:"bitrate-unit"`
}

type Subscriber struct {
	PlmnID                            string                              `json:"plmnID"`
	UeId                              string                              `json:"ueId"`
	AuthenticationSubscription        AuthenticationSubscription          `json:"AuthenticationSubscription"`
	AccessAndMobilitySubscriptionData AccessAndMobilitySubscriptionData   `json:"AccessAndMobilitySubscriptionData"`
	SessionManagementSubscriptionData []SessionManagementSubscriptionData `json:"SessionManagementSubscriptionData"`
	SmfSelectionSubscriptionData      SmfSelectionSubscriptionData        `json:"SmfSelectionSubscriptionData"`
	AmPolicyData                      AmPolicyData                        `json:"AmPolicyData"`
	SmPolicyData                      SmPolicyData                        `json:"SmPolicyData"`
	FlowRules                         []FlowRule                          `json:"FlowRules"`
}
