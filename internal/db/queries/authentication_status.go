package queries

import (
	"encoding/json"

	"github.com/yeastengine/ella/internal/amf/logger"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/db/models"
	"go.mongodb.org/mongo-driver/bson"
)

func GetAuthenticationStatus(ueId string) (*models.AuthStatus, error) {
	filter := bson.M{"ueId": ueId}

	dbAuthStatus, errGetOne := db.CommonDBClient.RestfulAPIGetOne(db.SUBSCDATA_AUT_AUTHSTATUS, filter)
	if errGetOne != nil {
		logger.DataRepoLog.Warnln(errGetOne)
	}
	var authStatus *models.AuthStatus
	json.Unmarshal(mapToByte(dbAuthStatus), &authStatus)
	return authStatus, nil
}
