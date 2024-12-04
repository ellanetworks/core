package queries

import (
	"encoding/json"

	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/db/models"
	"go.mongodb.org/mongo-driver/bson"
)

func ListNetworkSliceNames() ([]string, error) {
	var networkSlices []string = make([]string, 0)
	rawNetworkSlices, err := db.CommonDBClient.RestfulAPIGetMany(db.SliceDataColl, bson.M{})
	if err != nil {
		return nil, err
	}
	for _, rawNetworkSlice := range rawNetworkSlices {
		if rawNetworkSlice["slice-name"] == nil {
			db.DbLog.Warnln("Could not find slice-name in network slice")
			continue
		}
		networkSlices = append(networkSlices, rawNetworkSlice["slice-name"].(string))
	}
	return networkSlices, nil
}

func ListNetworkSlices() []*models.Slice {
	rawSlices, errGetMany := db.CommonDBClient.RestfulAPIGetMany(db.SliceDataColl, nil)
	if errGetMany != nil {
		return nil
	}
	var slices []*models.Slice
	for _, rawSlice := range rawSlices {
		var sliceData models.Slice
		err := json.Unmarshal(mapToByte(rawSlice), &sliceData)
		if err != nil {
			db.DbLog.Warnf("Could not unmarshal slice data: %v", rawSlice)
			continue
		}
		slices = append(slices, &sliceData)
	}
	return slices
}

func GetNetworkSliceByName(name string) (*models.Slice, error) {
	var networkSlice *models.Slice
	filter := bson.M{"slice-name": name}
	rawNetworkSlice, err := db.CommonDBClient.RestfulAPIGetOne(db.SliceDataColl, filter)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(mapToByte(rawNetworkSlice), &networkSlice)
	return networkSlice, nil
}

func DeleteNetworkSlice(name string) error {
	filter := bson.M{"slice-name": name}
	err := db.CommonDBClient.RestfulAPIDeleteOne(db.SliceDataColl, filter)
	if err != nil {
		return err
	}
	return nil
}

func CreateNetworkSlice(slice *models.Slice) error {
	filter := bson.M{"slice-name": slice.SliceName}
	sliceDataBsonA := toBsonM(&slice)
	_, err := db.CommonDBClient.RestfulAPIPost(db.SliceDataColl, filter, sliceDataBsonA)
	if err != nil {
		return err
	}
	return nil
}
