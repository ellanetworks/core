package queries

import (
	"encoding/json"
	"strconv"

	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/db/models"
	"go.mongodb.org/mongo-driver/bson"
)

func DeleteAmData(imsi string, mcc string, mnc string) error {
	filter := bson.M{"ueId": "imsi-" + imsi, "servingPlmnId": mcc + mnc}
	err := db.CommonDBClient.RestfulAPIDeleteOne(db.AmDataColl, filter)
	if err != nil {
		return err
	}
	return nil
}

func DeleteAmData2(ueId string) error {
	filter := bson.M{"ueId": "imsi-" + ueId}
	err := db.CommonDBClient.RestfulAPIDeleteOne(db.AmDataColl, filter)
	if err != nil {
		return err
	}
	return nil
}

// This method should be deleted in favor or `CreateAmData`
func CreateAmProvisionedData(snssai *models.Snssai, qos *models.DeviceGroupsIpDomainExpandedUeDnnQos, mcc, mnc, imsi string) error {
	amData := models.AccessAndMobilitySubscriptionData{
		Gpsis: []string{
			"msisdn-0900000000",
		},
		Nssai: &models.Nssai{
			DefaultSingleNssais: []models.Snssai{*snssai},
			SingleNssais:        []models.Snssai{*snssai},
		},
		SubscribedUeAmbr: &models.AmbrRm{
			Downlink: convertToString(uint64(qos.DnnMbrDownlink)),
			Uplink:   convertToString(uint64(qos.DnnMbrUplink)),
		},
	}
	amDataBsonA := toBsonM(amData)
	amDataBsonA["ueId"] = "imsi-" + imsi
	amDataBsonA["servingPlmnId"] = mcc + mnc
	filter := bson.M{
		"ueId": "imsi-" + imsi,
		"$or": []bson.M{
			{"servingPlmnId": mcc + mnc},
			{"servingPlmnId": bson.M{"$exists": false}},
		},
	}
	_, err := db.CommonDBClient.RestfulAPIPost(db.AmDataColl, filter, amDataBsonA)
	if err != nil {
		return err
	}
	return nil
}

func CreateAmData(ueId string) error {
	basicAmData := map[string]interface{}{
		"ueId": ueId,
	}
	filter := bson.M{"ueId": ueId}
	basicDataBson := toBsonM(basicAmData)
	_, err := db.CommonDBClient.RestfulAPIPost(db.AmDataColl, filter, basicDataBson)
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

func convertToString(val uint64) string {
	var mbVal, gbVal, kbVal uint64
	kbVal = val / 1000
	mbVal = val / 1000000
	gbVal = val / 1000000000
	var retStr string
	if gbVal != 0 {
		retStr = strconv.FormatUint(gbVal, 10) + " Gbps"
	} else if mbVal != 0 {
		retStr = strconv.FormatUint(mbVal, 10) + " Mbps"
	} else if kbVal != 0 {
		retStr = strconv.FormatUint(kbVal, 10) + " Kbps"
	} else {
		retStr = strconv.FormatUint(val, 10) + " bps"
	}

	return retStr
}
