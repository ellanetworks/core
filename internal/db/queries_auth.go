package db

import (
	"encoding/json"

	"go.mongodb.org/mongo-driver/bson"
)

func GetAuthenticationSubscription(ueId string) (*AuthenticationSubscription, error) {
	filterUeIdOnly := bson.M{"ueId": ueId}
	authSubsDataInterface, err := CommonDBClient.RestfulAPIGetOne(AuthSubsDataColl, filterUeIdOnly)
	if err != nil {
		return nil, err
	}
	var authSubsData *AuthenticationSubscription
	json.Unmarshal(mapToByte(authSubsDataInterface), &authSubsData)
	return authSubsData, nil
}
