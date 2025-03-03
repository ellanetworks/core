package models

type SessionManagementSubscriptionData struct {
	SingleNssai       *Snssai
	DnnConfigurations map[string]DnnConfiguration // A map (list of key-value pairs where Dnn serves as key) of DnnConfigurations
}
