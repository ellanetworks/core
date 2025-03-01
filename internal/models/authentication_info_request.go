package models

type AuthenticationInfoRequest struct {
	SupportedFeatures     string
	ServingNetworkName    string
	ResynchronizationInfo *ResynchronizationInfo
	AusfInstanceId        string
}
