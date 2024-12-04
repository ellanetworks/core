package queries

import (
	"encoding/json"

	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/db/models"
	"go.mongodb.org/mongo-driver/bson"
)

func ListInventoryGnbs() ([]*models.Gnb, error) {
	var gnbs []*models.Gnb
	rawGnbs, err := db.CommonDBClient.RestfulAPIGetMany(db.GnbDataColl, bson.M{})
	if err != nil {
		return nil, err
	}
	for _, rawGnb := range rawGnbs {
		var gnbData models.Gnb
		err := json.Unmarshal(mapToByte(rawGnb), &gnbData)
		if err != nil {
			db.DbLog.Errorf("Could not unmarshall gNB %v", rawGnb)
			continue
		}
		gnbs = append(gnbs, &gnbData)
	}
	return gnbs, nil
}

func CreateGnb(gnb *models.Gnb) error {
	filter := bson.M{"name": gnb.Name}
	gnbDataBson := toBsonM(&gnb)
	_, err := db.CommonDBClient.RestfulAPIPost(db.GnbDataColl, filter, gnbDataBson)
	if err != nil {
		return err
	}
	return nil
}

func DeleteGnb(name string) error {
	filter := bson.M{"name": name}
	err := db.CommonDBClient.RestfulAPIDeleteOne(db.GnbDataColl, filter)
	if err != nil {
		return err
	}
	return nil
}
