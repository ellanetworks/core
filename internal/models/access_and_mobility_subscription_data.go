package models

type AccessAndMobilitySubscriptionData struct {
	SubscribedUeAmbr *AmbrRm
}

type SubscriberData struct {
	AccessAndMobilitySubscriptionData *AccessAndMobilitySubscriptionData
	Dnn                               string
}
