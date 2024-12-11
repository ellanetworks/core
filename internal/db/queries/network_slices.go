package queries

import (
	"encoding/json"

	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/db/models"
	"github.com/yeastengine/ella/internal/logger"
	"go.mongodb.org/mongo-driver/bson"
)

func ListNetworkSliceNames() ([]string, error) {
	var networkSlices []string = make([]string, 0)
	rawNetworkSlices, err := db.CommonDBClient.RestfulAPIGetMany(db.NetworkSlicesColl, bson.M{})
	if err != nil {
		return nil, err
	}
	for _, rawNetworkSlice := range rawNetworkSlices {
		if rawNetworkSlice["name"] == nil {
			logger.DBLog.Warnln("Could not find name in network slice")
			continue
		}
		networkSlices = append(networkSlices, rawNetworkSlice["name"].(string))
	}
	return networkSlices, nil
}

func ListNetworkSlices() []*models.NetworkSlice {
	rawSlices, errGetMany := db.CommonDBClient.RestfulAPIGetMany(db.NetworkSlicesColl, nil)
	if errGetMany != nil {
		return nil
	}
	var slices []*models.NetworkSlice
	for _, rawSlice := range rawSlices {
		var sliceData models.NetworkSlice
		err := json.Unmarshal(mapToByte(rawSlice), &sliceData)
		if err != nil {
			logger.DBLog.Warnf("Could not unmarshal slice data: %v", rawSlice)
			continue
		}
		slices = append(slices, &sliceData)
	}
	return slices
}

func GetNetworkSliceByName(name string) (*models.NetworkSlice, error) {
	var networkSlice *models.NetworkSlice
	filter := bson.M{"name": name}
	rawNetworkSlice, err := db.CommonDBClient.RestfulAPIGetOne(db.NetworkSlicesColl, filter)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(mapToByte(rawNetworkSlice), &networkSlice)
	return networkSlice, nil
}

func DeleteNetworkSlice(name string) error {
	filter := bson.M{"name": name}
	err := db.CommonDBClient.RestfulAPIDeleteOne(db.NetworkSlicesColl, filter)
	if err != nil {
		return err
	}
	return nil
}

func CreateNetworkSlice(slice *models.NetworkSlice) error {
	filter := bson.M{"name": slice.Name}
	sliceDataBsonA := toBsonM(&slice)
	_, err := db.CommonDBClient.RestfulAPIPost(db.NetworkSlicesColl, filter, sliceDataBsonA)
	if err != nil {
		return err
	}
	return nil
}
