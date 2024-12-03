package db

import (
	"go.mongodb.org/mongo-driver/bson"
)

func DeleteSmfSelection(imsi string, mcc string, mnc string) error {
	filter := bson.M{"ueId": "imsi-" + imsi, "servingPlmnId": mcc + mnc}
	err := CommonDBClient.RestfulAPIDeleteOne(SmfSelDataColl, filter)
	if err != nil {
		return err
	}
	return nil
}

func CreateSmfSelectionProviosionedData(snssai *Snssai, mcc, mnc, dnn, imsi string) error {
	smfSelData := SmfSelectionSubscriptionData{
		SubscribedSnssaiInfos: map[string]SnssaiInfo{
			SnssaiModelsToHex(*snssai): {
				DnnInfos: []DnnInfo{
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
	_, err := CommonDBClient.RestfulAPIPost(SmfSelDataColl, filter, smfSelecDataBsonA)
	if err != nil {
		return err
	}
	return nil
}
