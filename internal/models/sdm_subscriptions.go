package models

type SdmSubscription struct {
	NfInstanceID   string
	SubscriptionID string
	PlmnID         *PlmnID
}
