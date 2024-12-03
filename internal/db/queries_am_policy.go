package db

import (
	"go.mongodb.org/mongo-driver/bson"
)

func DeleteAmPolicy(imsi string) error {
	filter := bson.M{"ueId": "imsi-" + imsi}
	err := CommonDBClient.RestfulAPIDeleteOne(AmPolicyDataColl, filter)
	if err != nil {
		DbLog.Warnln(err)
		return err
	}
	return nil
}
