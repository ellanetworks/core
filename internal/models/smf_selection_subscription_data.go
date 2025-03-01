package models

type SmfSelectionSubscriptionData struct {
	SupportedFeatures     string
	SubscribedSnssaiInfos map[string]SnssaiInfo
	SharedSnssaiInfosId   string
}
