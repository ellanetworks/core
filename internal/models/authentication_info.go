package models

type AuthenticationInfo struct {
	Suci                  string
	ServingNetworkName    string
	ResynchronizationInfo *ResynchronizationInfo
}
