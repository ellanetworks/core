package models

type AuthStatus struct {
	UeId               string `json:"ueId" bson:"ueId"`
	NfInstanceId       string `json:"nfInstanceId" bson:"nfInstanceId"`
	Success            bool   `json:"success" bson:"success"`
	TimeStamp          string `json:"timeStamp" bson:"timeStamp"`
	AuthType           string `json:"authType" bson:"authType"`
	ServingNetworkName string `json:"servingNetworkName" bson:"servingNetworkName"`
}
