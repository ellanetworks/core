package queries

import (
	"encoding/json"

	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/db/models"
	"go.mongodb.org/mongo-driver/bson"
)

func ListDeviceGroupNames() ([]string, error) {
	var deviceGroups []string = make([]string, 0)
	rawDeviceGroups, errGetMany := db.CommonDBClient.RestfulAPIGetMany(db.DevGroupDataColl, bson.M{})
	if errGetMany != nil {
		return nil, errGetMany
	}
	for _, rawDeviceGroup := range rawDeviceGroups {
		groupName, err := rawDeviceGroup["group-name"].(string)
		if !err {
			db.DbLog.Warnf("Could not get group-name from %v", rawDeviceGroup)
			continue
		}
		deviceGroups = append(deviceGroups, groupName)
	}
	return deviceGroups, nil
}

func GetDeviceGroupByName(name string) *models.DeviceGroup {
	var deviceGroup *models.DeviceGroup
	filter := bson.M{"group-name": name}
	rawDeviceGroup, err := db.CommonDBClient.RestfulAPIGetOne(db.DevGroupDataColl, filter)
	if err != nil {
		db.DbLog.Warnln(err)
		return nil
	}
	json.Unmarshal(mapToByte(rawDeviceGroup), &deviceGroup)
	return deviceGroup
}

func DeleteDeviceGroup(name string) error {
	filter := bson.M{"group-name": name}
	err := db.CommonDBClient.RestfulAPIDeleteOne(db.DevGroupDataColl, filter)
	if err != nil {
		return err
	}
	return nil
}

func CreateDeviceGroup(deviceGroup *models.DeviceGroup) error {
	filter := bson.M{"group-name": deviceGroup.DeviceGroupName}
	deviceGroupData := toBsonM(&deviceGroup)
	_, err := db.CommonDBClient.RestfulAPIPost(db.DevGroupDataColl, filter, deviceGroupData)
	if err != nil {
		return err
	}
	db.DbLog.Infof("Created Device Group: %v", deviceGroup.DeviceGroupName)
	return nil
}
