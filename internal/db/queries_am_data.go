package db

import (
	"strconv"

	"go.mongodb.org/mongo-driver/bson"
)

func DeleteAmData(imsi string, mcc string, mnc string) error {
	filter := bson.M{"ueId": "imsi-" + imsi, "servingPlmnId": mcc + mnc}
	err := CommonDBClient.RestfulAPIDeleteOne(AmDataColl, filter)
	if err != nil {
		return err
	}
	return nil
}

func CreateAmProvisionedData(snssai *Snssai, qos *DeviceGroupsIpDomainExpandedUeDnnQos, mcc, mnc, imsi string) error {
	amData := AccessAndMobilitySubscriptionData{
		Gpsis: []string{
			"msisdn-0900000000",
		},
		Nssai: &Nssai{
			DefaultSingleNssais: []Snssai{*snssai},
			SingleNssais:        []Snssai{*snssai},
		},
		SubscribedUeAmbr: &AmbrRm{
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
	_, err := CommonDBClient.RestfulAPIPost(AmDataColl, filter, amDataBsonA)
	if err != nil {
		return err
	}
	return nil
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
