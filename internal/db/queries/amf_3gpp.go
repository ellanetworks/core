package queries

import (
	"encoding/json"

	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/db/models"
	"go.mongodb.org/mongo-driver/bson"
)

func GetAmf3GPP(ueId string) (*models.Amf3GppAccessRegistration, error) {
	filter := bson.M{"ueId": ueId}
	dbAmfData, err := db.CommonDBClient.RestfulAPIGetOne(db.SUBSCDATA_CTXDATA_AMF_3GPPACCESS, filter)
	if err != nil {
		return nil, err
	}
	var amfData *models.Amf3GppAccessRegistration
	json.Unmarshal(mapToByte(dbAmfData), &amfData)
	return amfData, nil
}

func EditAmf3GPP(ueId string, amfData *models.Amf3GppAccessRegistration) error {
	putData := toBsonM(amfData)
	putData["ueId"] = ueId
	filter := bson.M{"ueId": ueId}
	_, err := db.CommonDBClient.RestfulAPIPutOne(db.SUBSCDATA_CTXDATA_AMF_3GPPACCESS, filter, putData)
	if err != nil {
		return err
	}
	return nil
}

func PatchAmf3GPP(ueId string, patchItem []models.PatchItem) error {
	patchJSON, err := json.Marshal(patchItem)
	if err != nil {
		return err
	}
	filter := bson.M{"ueId": ueId}
	err = db.CommonDBClient.RestfulAPIJSONPatch(db.SUBSCDATA_CTXDATA_AMF_3GPPACCESS, filter, patchJSON)
	return err
}
