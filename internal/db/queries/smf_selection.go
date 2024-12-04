package queries

import (
	"encoding/json"

	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/db/models"
	"go.mongodb.org/mongo-driver/bson"
)

func DeleteSmfSelection(imsi string, mcc string, mnc string) error {
	filter := bson.M{"ueId": "imsi-" + imsi, "servingPlmnId": mcc + mnc}
	err := db.CommonDBClient.RestfulAPIDeleteOne(db.SmfSelDataColl, filter)
	if err != nil {
		return err
	}
	return nil
}

func CreateSmfSelectionProviosionedData(snssai *models.Snssai, mcc, mnc, dnn, imsi string) error {
	smfSelData := models.SmfSelectionSubscriptionData{
		SubscribedSnssaiInfos: map[string]models.SnssaiInfo{
			SnssaiModelsToHex(*snssai): {
				DnnInfos: []models.DnnInfo{
					{
						Dnn: dnn,
					},
				},
			},
		},
	}
	smfSelecDataBsonA := toBsonM(smfSelData)
	smfSelecDataBsonA["ueId"] = "imsi-" + imsi
	smfSelecDataBsonA["servingPlmnId"] = mcc + mnc
	filter := bson.M{"ueId": "imsi-" + imsi, "servingPlmnId": mcc + mnc}
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
