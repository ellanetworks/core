package queries

import (
	"encoding/json"

	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/db/models"
	"go.mongodb.org/mongo-driver/bson"
)

func DeleteSmfSelection(imsi string) error {
	filter := bson.M{"ueId": "imsi-" + imsi}
	err := db.CommonDBClient.RestfulAPIDeleteOne(db.SmfSelDataColl, filter)
	if err != nil {
		return err
	}
	return nil
}

func CreateSmfSelectionData(smfSelData *models.SmfSelectionSubscriptionData) error {
	smfSelecDataBsonA := toBsonM(smfSelData)
	filter := bson.M{"ueId": smfSelData.UeId}
	_, err := db.CommonDBClient.RestfulAPIPost(db.SmfSelDataColl, filter, smfSelecDataBsonA)
	if err != nil {
		return err
	}
	return nil
}

func GetSmfSelectionSubscriptionData(ueId string) (*models.SmfSelectionSubscriptionData, error) {
	filter := bson.M{"ueId": ueId}
	smfSelDataInterface, err := db.CommonDBClient.RestfulAPIGetOne(db.SmfSelDataColl, filter)
	if err != nil {
		return nil, err
	}
	var smfSelData *models.SmfSelectionSubscriptionData
	json.Unmarshal(mapToByte(smfSelDataInterface), &smfSelData)
	return smfSelData, nil
}
