package queries

import (
	"encoding/json"

	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/db/models"
	"github.com/yeastengine/ella/internal/logger"
	"go.mongodb.org/mongo-driver/bson"
)

func ListInventoryRadios() ([]*models.Radio, error) {
	var radios []*models.Radio
	rawGnbs, err := db.CommonDBClient.RestfulAPIGetMany(db.RadiosColl, bson.M{})
	if err != nil {
		return nil, err
	}
	for _, rawGnb := range rawGnbs {
		var radioData models.Radio
		err := json.Unmarshal(mapToByte(rawGnb), &radioData)
		if err != nil {
			logger.DBLog.Errorf("Could not unmarshall gNB %v", rawGnb)
			continue
		}
		radios = append(radios, &radioData)
	}
	return radios, nil
}

func CreateRadio(radio *models.Radio) error {
	filter := bson.M{"name": radio.Name}
	radioDataBson := toBsonM(&radio)
	_, err := db.CommonDBClient.RestfulAPIPost(db.RadiosColl, filter, radioDataBson)
	if err != nil {
		return err
	}
	return nil
}

func DeleteRadio(name string) error {
	filter := bson.M{"name": name}
	err := db.CommonDBClient.RestfulAPIDeleteOne(db.RadiosColl, filter)
	if err != nil {
		return err
	}
	return nil
}
