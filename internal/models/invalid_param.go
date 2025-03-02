package models

type InvalidParam struct {
	// Attribute's name encoded as a JSON Pointer, or header's name.
	Param string `json:"param" yaml:"param" bson:"param" mapstructure:"Param"`
	// A human-readable reason, e.g. \"must be a positive integer\".
	Reason string `json:"reason,omitempty" yaml:"reason" bson:"reason" mapstructure:"Reason"`
}
