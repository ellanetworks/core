package queries

import (
	"encoding/json"

	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/db/models"
	"go.mongodb.org/mongo-driver/bson"
)

func CreateAmPolicyData(imsi string) error {
	var amPolicy models.AmPolicyData
	amPolicy.SubscCats = append(amPolicy.SubscCats, "free5gc")
	amPolicyDatBsonA := toBsonM(amPolicy)
	amPolicyDatBsonA["ueId"] = "imsi-" + imsi
	filter := bson.M{"ueId": "imsi-" + imsi}
	_, err := db.CommonDBClient.RestfulAPIPost(db.AmPolicyDataColl, filter, amPolicyDatBsonA)
	if err != nil {
		return err
	}
	return nil
}

func GetAmPolicyData(ueId string) (*models.AmPolicyData, error) {
	filterUeIdOnly := bson.M{"ueId": ueId}
	amPolicyDataInterface, err := db.CommonDBClient.RestfulAPIGetOne(db.AmPolicyDataColl, filterUeIdOnly)
	if err != nil {
		return nil, err
	}
	var amPolicyData *models.AmPolicyData
	json.Unmarshal(mapToByte(amPolicyDataInterface), &amPolicyData)
	return amPolicyData, nil
}
