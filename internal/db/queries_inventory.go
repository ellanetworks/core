package db

import (
	"encoding/json"

	"go.mongodb.org/mongo-driver/bson"
)

func ListInventoryGnbs() ([]*Gnb, error) {
	var gnbs []*Gnb
	rawGnbs, err := CommonDBClient.RestfulAPIGetMany(GnbDataColl, bson.M{})
	if err != nil {
		return nil, err
	}
	for _, rawGnb := range rawGnbs {
		var gnbData Gnb
		err := json.Unmarshal(mapToByte(rawGnb), &gnbData)
		if err != nil {
			DbLog.Errorf("Could not unmarshall gNB %v", rawGnb)
			continue
		}
		gnbs = append(gnbs, &gnbData)
	}
	return gnbs, nil
}

func CreateGnb(gnb *Gnb) error {
	filter := bson.M{"name": gnb.Name}
	gnbDataBson := toBsonM(&gnb)
	_, err := CommonDBClient.RestfulAPIPost(GnbDataColl, filter, gnbDataBson)
	if err != nil {
		return err
	}
	return nil
}

func DeleteGnb(name string) error {
	filter := bson.M{"name": name}
	err := CommonDBClient.RestfulAPIDeleteOne(GnbDataColl, filter)
	if err != nil {
		return err
	}
	return nil
}
