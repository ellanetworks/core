package models

type NetworkId struct {
	Mnc string `json:"mnc,omitempty" yaml:"mnc" bson:"mnc" mapstructure:"Mnc"`
	Mcc string `json:"mcc,omitempty" yaml:"mcc" bson:"mcc" mapstructure:"Mcc"`
}
