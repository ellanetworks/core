package models

import (
	"time"
)

type SdmSubscription struct {
	NfInstanceId        string
	ImplicitUnsubscribe bool
	Expires             *time.Time
	CallbackReference   string
	// AmfServiceName        ServiceName `json:"amfServiceName,omitempty" yaml:"amfServiceName" bson:"amfServiceName" mapstructure:"AmfServiceName"`
	MonitoredResourceUris []string
	SingleNssai           *Snssai
	Dnn                   string
	SubscriptionId        string
	PlmnId                *PlmnId
}
