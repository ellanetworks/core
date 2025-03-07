package models

type SdmSubscription struct {
	NfInstanceId   string
	SubscriptionId string
	PlmnID         *PlmnID
}
