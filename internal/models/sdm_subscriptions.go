package models

import (
	"time"
)

type SdmSubscription struct {
	NfInstanceId          string
	ImplicitUnsubscribe   bool
	Expires               *time.Time
	CallbackReference     string
	MonitoredResourceUris []string
	SingleNssai           *Snssai
	Dnn                   string
	SubscriptionId        string
	PlmnId                *PlmnId
}
