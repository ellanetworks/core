package models

type UeN1N2InfoSubscriptionCreatedData struct {
	N1n2NotifySubscriptionId string `json:"n1n2NotifySubscriptionId"`
	SupportedFeatures        string `json:"supportedFeatures,omitempty"`
}
