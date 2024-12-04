package queries

import (
	"encoding/json"

	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/db/models"
	"go.mongodb.org/mongo-driver/bson"
)

func sliceToByte(data []map[string]interface{}) (ret []byte) {
	ret, _ = json.Marshal(data)
	return
}

func DeleteSmData(imsi string, mcc string, mnc string) error {
	filter := bson.M{"ueId": "imsi-" + imsi, "servingPlmnId": mcc + mnc}
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

func CreateSmProvisionedData(snssai *models.Snssai, qos *models.DeviceGroupsIpDomainExpandedUeDnnQos, mcc, mnc, dnn, imsi string) error {
	smData := models.SessionManagementSubscriptionData{
		SingleNssai: snssai,
		DnnConfigurations: map[string]models.DnnConfiguration{
			dnn: {
				PduSessionTypes: &models.PduSessionTypes{
					DefaultSessionType:  models.PduSessionType_IPV4,
					AllowedSessionTypes: []models.PduSessionType{models.PduSessionType_IPV4},
				},
				SscModes: &models.SscModes{
					DefaultSscMode: models.SscMode__1,
					AllowedSscModes: []models.SscMode{
						"SSC_MODE_2",
						"SSC_MODE_3",
					},
				},
				SessionAmbr: &models.Ambr{
					Downlink: convertToString(uint64(qos.DnnMbrDownlink)),
					Uplink:   convertToString(uint64(qos.DnnMbrUplink)),
				},
				Var5gQosProfile: &models.SubscribedDefaultQos{
					Var5qi: 9,
					Arp: &models.Arp{
						PriorityLevel: 8,
					},
					PriorityLevel: 8,
				},
			},
		},
	}
	smDataBsonA := toBsonM(smData)
	smDataBsonA["ueId"] = "imsi-" + imsi
	smDataBsonA["servingPlmnId"] = mcc + mnc
	filter := bson.M{"ueId": "imsi-" + imsi, "servingPlmnId": mcc + mnc}
	_, err := db.CommonDBClient.RestfulAPIPost(db.SmDataColl, filter, smDataBsonA)
	if err != nil {
		return err
	}
	return nil
}
