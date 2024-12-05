package queries

import (
	"encoding/json"

	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/db/models"
	"go.mongodb.org/mongo-driver/bson"
)

func GetAuthenticationSubscription(ueId string) (*models.AuthenticationSubscription, error) {
	filterUeIdOnly := bson.M{"ueId": ueId}
	authSubsDataInterface, err := db.CommonDBClient.RestfulAPIGetOne(db.AuthSubsDataColl, filterUeIdOnly)
	if err != nil {
		return nil, err
	}
	var authSubsData *models.AuthenticationSubscription
	json.Unmarshal(mapToByte(authSubsDataInterface), &authSubsData)
	return authSubsData, nil
}

func CreateAuthenticationSubscription(ueId string, authSubsData *models.AuthenticationSubscription) error {
	filter := bson.M{"ueId": ueId}
	authDataBsonA := toBsonM(authSubsData)
	authDataBsonA["ueId"] = ueId
	_, err := db.CommonDBClient.RestfulAPIPost(db.AuthSubsDataColl, filter, authDataBsonA)
	if err != nil {
		return err
	}
	return nil
}

func DeleteAuthenticationSubscription(ueId string) error {
	filter := bson.M{"ueId": "imsi-" + ueId}
	err := db.CommonDBClient.RestfulAPIDeleteOne(db.AuthSubsDataColl, filter)
	if err != nil {
		return err
	}
	return nil
}
