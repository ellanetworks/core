package queries

import (
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/logger"
	"go.mongodb.org/mongo-driver/bson"
)

func DeleteSmPolicy(imsi string) error {
	filter := bson.M{"ueId": "imsi-" + imsi}
	err := db.CommonDBClient.RestfulAPIDeleteOne(db.SmPolicyDataColl, filter)
	if err != nil {
		logger.DBLog.Warnln(err)
		return err
	}
	return nil
}
