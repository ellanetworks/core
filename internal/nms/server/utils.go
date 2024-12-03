package server

import (
	"encoding/json"

	"github.com/yeastengine/ella/internal/nms/logger"
	"go.mongodb.org/mongo-driver/bson"
)

// This should be deleted at the end of this task
func toBsonM(data interface{}) (ret bson.M) {
	tmp, err := json.Marshal(data)
	if err != nil {
		logger.NMSLog.Errorln("Could not marshall data")
		return nil
	}
	err = json.Unmarshal(tmp, &ret)
	if err != nil {
		logger.NMSLog.Errorln("Could not unmarshall data")
		return nil
	}
	return ret
}
