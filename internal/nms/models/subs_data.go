package models

import (
	"github.com/omec-project/openapi/models"
)

type SubsData struct {
	PlmnID                            string                                     `json:"plmnID"`
	UeId                              string                                     `json:"ueId"`
	Sst                               int32                                      `json:"sst" yaml:"sst" bson:"sst" mapstructure:"Sst"`
	Sd                                string                                     `json:"sd,omitempty" yaml:"sd" bson:"sd" mapstructure:"Sd"`
	Dnn                               string                                     `json:"dnn" yaml:"dnn" bson:"dnn" mapstructure:"Dnn"`
	AuthenticationSubscription        models.AuthenticationSubscription          `json:"AuthenticationSubscription"`
	AccessAndMobilitySubscriptionData models.AccessAndMobilitySubscriptionData   `json:"AccessAndMobilitySubscriptionData"`
	SessionManagementSubscriptionData []models.SessionManagementSubscriptionData `json:"SessionManagementSubscriptionData"`
	AmPolicyData                      models.AmPolicyData                        `json:"AmPolicyData"`
	SmPolicyData                      models.SmPolicyData                        `json:"SmPolicyData"`
	FlowRules                         []FlowRule                                 `json:"FlowRules"`
}

type SubsOverrideData struct {
	PlmnID         string `json:"plmnID"`
	OPc            string `json:"opc"`
	Key            string `json:"key"`
	SequenceNumber string `json:"sequenceNumber"`
}
