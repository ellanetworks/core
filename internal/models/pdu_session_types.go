package models

type PduSessionTypes struct {
	DefaultSessionType  PduSessionType   `json:"defaultSessionType" yaml:"defaultSessionType" bson:"defaultSessionType" mapstructure:"DefaultSessionType"`
	AllowedSessionTypes []PduSessionType `json:"allowedSessionTypes,omitempty" yaml:"allowedSessionTypes" bson:"allowedSessionTypes" mapstructure:"AllowedSessionTypes"`
}
