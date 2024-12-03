package db

import (
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
)

func CreateSmPolicyData(snssai *Snssai, dnn string, imsi string) error {
	var smPolicyData SmPolicyData
	var smPolicySnssaiData SmPolicySnssaiData
	dnnData := map[string]SmPolicyDnnData{
		dnn: {
			Dnn: dnn,
		},
	}
	smPolicySnssaiData.Snssai = snssai
	smPolicySnssaiData.SmPolicyDnnData = dnnData
	smPolicyData.SmPolicySnssaiData = make(map[string]SmPolicySnssaiData)
	smPolicyData.SmPolicySnssaiData[SnssaiModelsToHex(*snssai)] = smPolicySnssaiData
	smPolicyDatBsonA := toBsonM(smPolicyData)
	smPolicyDatBsonA["ueId"] = "imsi-" + imsi
	filter := bson.M{"ueId": "imsi-" + imsi}
	_, err := CommonDBClient.RestfulAPIPost(SmPolicyDataColl, filter, smPolicyDatBsonA)
	if err != nil {
		return err
	}
	return nil
}

func SnssaiModelsToHex(snssai Snssai) string {
	sst := fmt.Sprintf("%02x", snssai.Sst)
	return sst + snssai.Sd
}
