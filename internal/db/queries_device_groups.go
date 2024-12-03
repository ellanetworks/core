package db

import (
	"encoding/json"

	"go.mongodb.org/mongo-driver/bson"
)

func ListDeviceGroupNames() ([]string, error) {
	var deviceGroups []string = make([]string, 0)
	rawDeviceGroups, errGetMany := CommonDBClient.RestfulAPIGetMany(DevGroupDataColl, bson.M{})
	if errGetMany != nil {
		return nil, errGetMany
	}
	for _, rawDeviceGroup := range rawDeviceGroups {
		groupName, err := rawDeviceGroup["group-name"].(string)
		if !err {
			DbLog.Warnf("Could not get group-name from %v", rawDeviceGroup)
			continue
		}
		deviceGroups = append(deviceGroups, groupName)
	}
	return deviceGroups, nil
}

func GetDeviceGroupByName(name string) *DeviceGroup {
	var deviceGroup *DeviceGroup
	filter := bson.M{"group-name": name}
	rawDeviceGroup, err := CommonDBClient.RestfulAPIGetOne(DevGroupDataColl, filter)
	if err != nil {
		DbLog.Warnln(err)
	}
	json.Unmarshal(mapToByte(rawDeviceGroup), &deviceGroup)
	return deviceGroup
}

func DeleteDeviceGroup(name string) bool {
	filter := bson.M{"group-name": name}
	errDelOne := CommonDBClient.RestfulAPIDeleteOne(DevGroupDataColl, filter)
	if errDelOne != nil {
		DbLog.Warnln(errDelOne)
	}
	DbLog.Infof("Deleted Device Group: %v", name)
	return true
}

func CreateDeviceGroup(deviceGroup *DeviceGroup) error {
	filter := bson.M{"group-name": deviceGroup.DeviceGroupName}
	deviceGroupData := toBsonM(&deviceGroup)
	_, errPost := CommonDBClient.RestfulAPIPost(DevGroupDataColl, filter, deviceGroupData)
	if errPost != nil {
		DbLog.Warnln(errPost)
		return errPost
	}
	DbLog.Infof("Created Device Group: %v", deviceGroup.DeviceGroupName)
	return nil
}

func mapToByte(data map[string]interface{}) (ret []byte) {
	ret, _ = json.Marshal(data)
	return
}

func toBsonM(data interface{}) (ret bson.M) {
	tmp, err := json.Marshal(data)
	if err != nil {
		DbLog.Errorln("Could not marshall data")
		return nil
	}
	err = json.Unmarshal(tmp, &ret)
	if err != nil {
		DbLog.Errorln("Could not unmarshall data")
		return nil
	}
	return ret
}
