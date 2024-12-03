package db

import "go.mongodb.org/mongo-driver/bson"

func CreateAmPolicyData(imsi string) error {
	var amPolicy AmPolicyData
	amPolicy.SubscCats = append(amPolicy.SubscCats, "free5gc")
	amPolicyDatBsonA := toBsonM(amPolicy)
	amPolicyDatBsonA["ueId"] = "imsi-" + imsi
	filter := bson.M{"ueId": "imsi-" + imsi}
	_, err := CommonDBClient.RestfulAPIPost(AmPolicyDataColl, filter, amPolicyDatBsonA)
	if err != nil {
		return err
	}
	return nil
}
