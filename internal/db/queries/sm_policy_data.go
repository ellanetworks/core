package queries

import (
	"encoding/json"

	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/db/models"
	"go.mongodb.org/mongo-driver/bson"
)

func CreateSmPolicyData(smPolicyData *models.SmPolicyData) error {
	smPolicyDatBsonA := toBsonM(smPolicyData)
	filter := bson.M{"ueId": smPolicyData.UeId}
	_, err := db.CommonDBClient.RestfulAPIPost(db.SmPolicyDataColl, filter, smPolicyDatBsonA)
	if err != nil {
		return err
	}
	return nil
}

func GetSmPolicyData(ueId string) (*models.SmPolicyData, error) {
	filter := bson.M{"ueId": ueId}
	smPolicyDataInterface, err := db.CommonDBClient.RestfulAPIGetOne(db.SmPolicyDataColl, filter)
	if err != nil {
		return nil, err
	}
	var smPolicyData *models.SmPolicyData
	json.Unmarshal(mapToByte(smPolicyDataInterface), &smPolicyData)
	return smPolicyData, nil
}
