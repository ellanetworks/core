package configapi

import (
	"encoding/json"

	"github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/webui/backend/logger"
	"github.com/yeastengine/ella/internal/webui/configmodels"
	"go.mongodb.org/mongo-driver/bson"
)

func toBsonM(data interface{}) (ret bson.M) {
	tmp, err := json.Marshal(data)
	if err != nil {
		logger.DbLog.Errorln("Could not marshall data")
		return nil
	}
	err = json.Unmarshal(tmp, &ret)
	if err != nil {
		logger.DbLog.Errorln("Could not unmarshall data")
		return nil
	}
	return ret
}

func UpdateSMF() {
	networkSlices := make([]configmodels.Slice, 0)
	networkSliceNames := ListNetworkSlices()
	for _, networkSliceName := range networkSliceNames {
		networkSlice := GetNetworkSliceByName2(networkSliceName)
		networkSlices = append(networkSlices, networkSlice)
	}
	deviceGroups := make([]configmodels.DeviceGroups, 0)
	deviceGroupNames := ListDeviceGroups()
	for _, deviceGroupName := range deviceGroupNames {
		deviceGroup := GetDeviceGroupByName2(deviceGroupName)
		deviceGroups = append(deviceGroups, deviceGroup)
	}
	context.UpdateSMFContext(networkSlices, deviceGroups)
}
