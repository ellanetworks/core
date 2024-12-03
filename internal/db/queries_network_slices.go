package db

import (
	"encoding/json"

	"go.mongodb.org/mongo-driver/bson"
)

func ListNetworkSliceNames() ([]string, error) {
	var networkSlices []string = make([]string, 0)
	rawNetworkSlices, err := CommonDBClient.RestfulAPIGetMany(SliceDataColl, bson.M{})
	if err != nil {
		return nil, err
	}
	for _, rawNetworkSlice := range rawNetworkSlices {
		if rawNetworkSlice["slice-name"] == nil {
			DbLog.Warnln("Could not find slice-name in network slice")
			continue
		}
		networkSlices = append(networkSlices, rawNetworkSlice["slice-name"].(string))
	}
	return networkSlices, nil
}

func ListNetworkSlices() []*Slice {
	rawSlices, errGetMany := CommonDBClient.RestfulAPIGetMany(SliceDataColl, nil)
	if errGetMany != nil {
		return nil
	}
	var slices []*Slice
	for _, rawSlice := range rawSlices {
		var sliceData Slice
		err := json.Unmarshal(mapToByte(rawSlice), &sliceData)
		if err != nil {
			DbLog.Warnf("Could not unmarshal slice data: %v", rawSlice)
			continue
		}
		slices = append(slices, &sliceData)
	}
	return slices
}

func GetNetworkSliceByName(name string) (*Slice, error) {
	var networkSlice *Slice
	filter := bson.M{"slice-name": name}
	rawNetworkSlice, err := CommonDBClient.RestfulAPIGetOne(SliceDataColl, filter)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(mapToByte(rawNetworkSlice), &networkSlice)
	return networkSlice, nil
}

func DeleteNetworkSlice(name string) error {
	filter := bson.M{"slice-name": name}
	err := CommonDBClient.RestfulAPIDeleteOne(SliceDataColl, filter)
	if err != nil {
		return err
	}
	return nil
}

func CreateNetworkSlice(slice *Slice) error {
	filter := bson.M{"slice-name": slice.SliceName}
	sliceDataBsonA := toBsonM(&slice)
	_, err := CommonDBClient.RestfulAPIPost(SliceDataColl, filter, sliceDataBsonA)
	if err != nil {
		return err
	}
	return nil
}
