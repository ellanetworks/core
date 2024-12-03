package db

import "go.mongodb.org/mongo-driver/bson"

func DeleteSmPolicy(imsi string) error {
	filter := bson.M{"ueId": "imsi-" + imsi}
	err := CommonDBClient.RestfulAPIDeleteOne(SmPolicyDataColl, filter)
	if err != nil {
		DbLog.Warnln(err)
		return err
	}
	return nil
}
