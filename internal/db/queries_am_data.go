package db

import "go.mongodb.org/mongo-driver/bson"

func DeleteAmData(imsi string, mcc string, mnc string) error {
	filter := bson.M{"ueId": "imsi-" + imsi, "servingPlmnId": mcc + mnc}
	err := CommonDBClient.RestfulAPIDeleteOne(AmDataColl, filter)
	if err != nil {
		return err
	}
	return nil
}
