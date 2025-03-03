package models

type AuthenticationInfoRequest struct {
	ServingNetworkName    string
	ResynchronizationInfo *ResynchronizationInfo
}
