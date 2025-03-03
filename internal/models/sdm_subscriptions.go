package models

type SdmSubscription struct {
	NfInstanceId   string
	SubscriptionId string
	PlmnId         *PlmnId
}
