package models

type AuthenticationInfo struct {
	SupiOrSuci            string
	ServingNetworkName    string
	ResynchronizationInfo *ResynchronizationInfo
}
