package queries

import (
	"encoding/json"

	"github.com/yeastengine/ella/internal/amf/logger"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/db/models"
	"github.com/yeastengine/ella/internal/udr/util"
	"go.mongodb.org/mongo-driver/bson"
)

func GetAuthenticationStatus(ueId string) (*models.AuthEvent, error) {
	filter := bson.M{"ueId": ueId}

	dbAuthStatus, errGetOne := db.CommonDBClient.RestfulAPIGetOne(db.SUBSCDATA_AUT_AUTHSTATUS, filter)
	if errGetOne != nil {
		logger.DataRepoLog.Warnln(errGetOne)
	}
	var authStatus *models.AuthEvent
	json.Unmarshal(mapToByte(dbAuthStatus), &authStatus)
	return authStatus, nil
}

func EditAuthenticationStatus(ueId string, authStatus *models.AuthEvent) error {
	filter := bson.M{"ueId": ueId}
	putData := util.ToBsonM(authStatus)
	putData["ueId"] = ueId
	_, err := db.CommonDBClient.RestfulAPIPutOne(db.SUBSCDATA_AUT_AUTHSTATUS, filter, putData)
	if err != nil {
		return err
	}
	return nil
}
