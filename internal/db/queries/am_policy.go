package queries

import (
	"github.com/yeastengine/ella/internal/db"
	"go.mongodb.org/mongo-driver/bson"
)

func DeleteAmPolicy(imsi string) error {
	filter := bson.M{"ueId": "imsi-" + imsi}
	err := db.CommonDBClient.RestfulAPIDeleteOne(db.AmPolicyDataColl, filter)
	if err != nil {
		return err
	}
	return nil
}
