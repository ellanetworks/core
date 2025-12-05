package models

type AccessAndMobilitySubscriptionData struct {
	SubscribedUeAmbr *AmbrRm
	Snssai           *Snssai
}

type SubscriberData struct {
	AccessAndMobilitySubscriptionData *AccessAndMobilitySubscriptionData
	Dnn                               string
}
