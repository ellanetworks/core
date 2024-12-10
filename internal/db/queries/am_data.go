package queries

import (
	"encoding/json"

	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/db/models"
	"go.mongodb.org/mongo-driver/bson"
)

func DeleteAmData(imsi string) error {
	filter := bson.M{"ueId": "imsi-" + imsi}
	err := db.CommonDBClient.RestfulAPIDeleteOne(db.AmDataColl, filter)
	if err != nil {
		return err
	}
	return nil
}

func CreateAmData(amData *models.AccessAndMobilitySubscriptionData) error {
	amDataBsonA := toBsonM(amData)
	filter := bson.M{"ueId": amData.UeId}
	_, err := db.CommonDBClient.RestfulAPIPost(db.AmDataColl, filter, amDataBsonA)
	if err != nil {
		return err
	}
	return nil
}

func GetAmData(ueId string) (*models.AccessAndMobilitySubscriptionData, error) {
	filterUeIdOnly := bson.M{"ueId": ueId}
	amData, err := db.CommonDBClient.RestfulAPIGetOne(db.AmDataColl, filterUeIdOnly)
	if err != nil {
		return nil, err
	}
	amDataObj := &models.AccessAndMobilitySubscriptionData{}
	json.Unmarshal(mapToByte(amData), &amDataObj)
	return amDataObj, nil
}

func ListAmData() ([]*models.AccessAndMobilitySubscriptionData, error) {
	amDataListObj := make([]*models.AccessAndMobilitySubscriptionData, 0)
	amDataList, err := db.CommonDBClient.RestfulAPIGetMany(db.AmDataColl, bson.M{})
	if err != nil {
		return nil, err
	}
	for _, amData := range amDataList {
		amDataObj := &models.AccessAndMobilitySubscriptionData{}
		json.Unmarshal(mapToByte(amData), &amDataObj)
		amDataListObj = append(amDataListObj, amDataObj)
	}
	return amDataListObj, nil
}
