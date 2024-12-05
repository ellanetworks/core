package models

import (
	"time"
)

type AuthType string

const (
	AuthType__5_G_AKA      AuthType = "5G_AKA"
	AuthType_EAP_AKA_PRIME AuthType = "EAP_AKA_PRIME"
	AuthType_EAP_TLS       AuthType = "EAP_TLS"
)

type AuthEvent struct {
	NfInstanceId       string     `json:"nfInstanceId" yaml:"nfInstanceId" bson:"nfInstanceId" mapstructure:"NfInstanceId"`
	Success            bool       `json:"success" yaml:"success" bson:"success" mapstructure:"Success"`
	TimeStamp          *time.Time `json:"timeStamp" yaml:"timeStamp" bson:"timeStamp" mapstructure:"TimeStamp"`
	AuthType           AuthType   `json:"authType" yaml:"authType" bson:"authType" mapstructure:"AuthType"`
	ServingNetworkName string     `json:"servingNetworkName" yaml:"servingNetworkName" bson:"servingNetworkName" mapstructure:"ServingNetworkName"`
}
