package queries

import (
	"encoding/json"
	"fmt"

	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/db/models"
	"go.mongodb.org/mongo-driver/bson"
)

func CreateSmPolicyData(snssai *models.Snssai, dnn string, imsi string) error {
	var smPolicyData models.SmPolicyData
	var smPolicySnssaiData models.SmPolicySnssaiData
	dnnData := map[string]models.SmPolicyDnnData{
		dnn: {
			Dnn: dnn,
		},
	}
	smPolicySnssaiData.Snssai = snssai
	smPolicySnssaiData.SmPolicyDnnData = dnnData
	smPolicyData.SmPolicySnssaiData = make(map[string]models.SmPolicySnssaiData)
	smPolicyData.SmPolicySnssaiData[SnssaiModelsToHex(*snssai)] = smPolicySnssaiData
	smPolicyDatBsonA := toBsonM(smPolicyData)
	smPolicyDatBsonA["ueId"] = "imsi-" + imsi
	filter := bson.M{"ueId": "imsi-" + imsi}
	_, err := db.CommonDBClient.RestfulAPIPost(db.SmPolicyDataColl, filter, smPolicyDatBsonA)
	if err != nil {
		return err
	}
	return nil
}

func SnssaiModelsToHex(snssai models.Snssai) string {
	sst := fmt.Sprintf("%02x", snssai.Sst)
	return sst + snssai.Sd
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
