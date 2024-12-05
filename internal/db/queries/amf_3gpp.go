package queries

import (
	"encoding/json"

	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/db/models"
	"go.mongodb.org/mongo-driver/bson"
)

func GetAmf3GPP(ueId string) (*models.Amf3GPP, error) {
	filter := bson.M{"ueId": ueId}
	dbAmfData, err := db.CommonDBClient.RestfulAPIGetOne(db.SUBSCDATA_CTXDATA_AMF_3GPPACCESS, filter)
	if err != nil {
		return nil, err
	}
	var amfData *models.Amf3GPP
	json.Unmarshal(mapToByte(dbAmfData), &amfData)
	return amfData, nil
}
