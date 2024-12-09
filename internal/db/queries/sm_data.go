package queries

import (
	"encoding/json"

	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/db/models"
	"go.mongodb.org/mongo-driver/bson"
)

func DeleteSmData(imsi string) error {
	filter := bson.M{"ueId": "imsi-" + imsi}
	err := db.CommonDBClient.RestfulAPIDeleteOne(db.SmDataColl, filter)
	if err != nil {
		return err
	}
	return nil
}

func ListSmData(ueId string) ([]*models.SessionManagementSubscriptionData, error) {
	filter := bson.M{"ueId": ueId}
	smData, err := db.CommonDBClient.RestfulAPIGetMany(db.SmDataColl, filter)
	if err != nil {
		return nil, err
	}
	var smDataData []*models.SessionManagementSubscriptionData
	json.Unmarshal(sliceToByte(smData), &smDataData)
	return smDataData, nil
}

func CreateSmData(smData *models.SessionManagementSubscriptionData) error {
	smDataBsonA := toBsonM(smData)
	filter := bson.M{"ueId": smData.UeId}
	_, err := db.CommonDBClient.RestfulAPIPost(db.SmDataColl, filter, smDataBsonA)
	if err != nil {
		return err
	}
	return nil
}
