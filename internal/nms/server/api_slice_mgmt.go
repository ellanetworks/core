package server

import (
	"math"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/omec-project/util/httpwrapper"
	"github.com/sirupsen/logrus"
	"github.com/yeastengine/ella/internal/nms/db"
	"github.com/yeastengine/ella/internal/nms/logger"
	"github.com/yeastengine/ella/internal/nms/models"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	KPS = 1000
	MPS = 1000000
	GPS = 1000000000
)

var configLog *logrus.Entry

func init() {
	configLog = logger.ConfigLog
}

func DeviceGroupDeleteHandler(c *gin.Context) bool {
	var groupName string
	var exists bool
	if groupName, exists = c.Params.Get("group-name"); exists {
		configLog.Infof("Received Delete Group %v from Roc/simapp", groupName)
	}
	var msg models.ConfigMessage
	msg.MsgType = models.Device_group
	msg.MsgMethod = models.Delete_op
	msg.DevGroupName = groupName
	var config5gMsg Update5GSubscriberMsg
	config5gMsg.PrevDevGroup = getDeviceGroupByName(msg.DevGroupName)
	filter := bson.M{"group-name": msg.DevGroupName}
	errDelOne := db.CommonDBClient.RestfulAPIDeleteOne(db.DevGroupDataColl, filter)
	if errDelOne != nil {
		logger.DbLog.Warnln(errDelOne)
	}
	config5gMsg.Msg = &msg
	updateConfig(&config5gMsg)
	updateSMF()
	configLog.Infof("Deleted Device Group: %v", groupName)
	return true
}

func convertToBps(val int64, unit string) (bitrate int64) {
	if strings.EqualFold(unit, "bps") {
		bitrate = val
	} else if strings.EqualFold(unit, "kbps") {
		bitrate = val * KPS
	} else if strings.EqualFold(unit, "mbps") {
		bitrate = val * MPS
	} else if strings.EqualFold(unit, "gbps") {
		bitrate = val * GPS
	}
	// default consider it as bps
	return bitrate
}

func DeviceGroupPostHandler(c *gin.Context, msgOp int) bool {
	var groupName string
	var exists bool
	if groupName, exists = c.Params.Get("group-name"); exists {
		configLog.Infof("Received group %v", groupName)
	}

	var err error
	var request models.DeviceGroups
	s := strings.Split(c.GetHeader("Content-Type"), ";")
	switch s[0] {
	case "application/json":
		err = c.ShouldBindJSON(&request)
	}
	if err != nil {
		configLog.Infof(" err %v", err)
		return false
	}
	req := httpwrapper.NewRequest(c.Request, request)

	procReq := req.Body.(models.DeviceGroups)
	ipdomain := &procReq.IpDomainExpanded

	if ipdomain.UeDnnQos != nil {
		ipdomain.UeDnnQos.DnnMbrDownlink = convertToBps(ipdomain.UeDnnQos.DnnMbrDownlink, ipdomain.UeDnnQos.BitrateUnit)
		if ipdomain.UeDnnQos.DnnMbrDownlink < 0 {
			ipdomain.UeDnnQos.DnnMbrDownlink = math.MaxInt64
		}
		ipdomain.UeDnnQos.DnnMbrUplink = convertToBps(ipdomain.UeDnnQos.DnnMbrUplink, ipdomain.UeDnnQos.BitrateUnit)
		if ipdomain.UeDnnQos.DnnMbrUplink < 0 {
			ipdomain.UeDnnQos.DnnMbrUplink = math.MaxInt64
		}
	}

	var msg models.ConfigMessage
	procReq.DeviceGroupName = groupName
	msg.MsgType = models.Device_group
	msg.MsgMethod = msgOp
	msg.DevGroup = &procReq
	msg.DevGroupName = groupName
	var config5gMsg Update5GSubscriberMsg
	config5gMsg.Msg = &msg
	config5gMsg.PrevDevGroup = getDeviceGroupByName(msg.DevGroupName)
	updateConfig(&config5gMsg)
	filter := bson.M{"group-name": msg.DevGroupName}
	devGroupDataBsonA := toBsonM(msg.DevGroup)
	_, errPost := db.CommonDBClient.RestfulAPIPost(db.DevGroupDataColl, filter, devGroupDataBsonA)
	if errPost != nil {
		logger.DbLog.Warnln(errPost)
	}
	updateSMF()
	configLog.Infof("Created Device Group: %v", groupName)
	return true
}

func NetworkSliceDeleteHandler(c *gin.Context) bool {
	var sliceName string
	var exists bool
	if sliceName, exists = c.Params.Get("slice-name"); exists {
		configLog.Infof("Received Deleted slice : %v from Roc/simapp", sliceName)
	}
	var msg models.ConfigMessage
	msg.MsgMethod = models.Delete_op
	msg.MsgType = models.Network_slice
	msg.SliceName = sliceName
	var config5gMsg Update5GSubscriberMsg

	config5gMsg.PrevSlice = getSliceByName(msg.SliceName)
	filter := bson.M{"slice-name": msg.SliceName}
	errDelOne := db.CommonDBClient.RestfulAPIDeleteOne(db.SliceDataColl, filter)
	if errDelOne != nil {
		logger.DbLog.Warnln(errDelOne)
	}
	config5gMsg.Msg = &msg
	updateConfig(&config5gMsg)
	updateSMF()
	configLog.Infof("Deleted Network Slice: %v", sliceName)
	return true
}

func NetworkSlicePostHandler(c *gin.Context, msgOp int) bool {
	var sliceName string
	var exists bool
	if sliceName, exists = c.Params.Get("slice-name"); exists {
		configLog.Infof("Received slice : %v", sliceName)
	}

	var err error
	var request models.Slice
	s := strings.Split(c.GetHeader("Content-Type"), ";")
	switch s[0] {
	case "application/json":
		err = c.ShouldBindJSON(&request)
	}
	if err != nil {
		return false
	}

	req := httpwrapper.NewRequest(c.Request, request)

	procReq := req.Body.(models.Slice)
	group := procReq.SiteDeviceGroup
	slices.Sort(group)

	for index := range procReq.ApplicationFilteringRules {
		ul := procReq.ApplicationFilteringRules[index].AppMbrUplink
		dl := procReq.ApplicationFilteringRules[index].AppMbrDownlink
		unit := procReq.ApplicationFilteringRules[index].BitrateUnit

		bitrate := convertToBps(int64(ul), unit)
		if bitrate < 0 || bitrate > math.MaxInt32 {
			procReq.ApplicationFilteringRules[index].AppMbrUplink = math.MaxInt32
		} else {
			procReq.ApplicationFilteringRules[index].AppMbrUplink = int32(bitrate)
		}

		bitrate = convertToBps(int64(dl), unit)
		if bitrate < 0 || bitrate > math.MaxInt32 {
			procReq.ApplicationFilteringRules[index].AppMbrDownlink = math.MaxInt32
		} else {
			procReq.ApplicationFilteringRules[index].AppMbrDownlink = int32(bitrate)
		}
	}

	var msg models.ConfigMessage
	msg.MsgMethod = msgOp
	procReq.SliceName = sliceName
	msg.MsgType = models.Network_slice
	msg.Slice = &procReq
	msg.SliceName = sliceName
	var config5gMsg Update5GSubscriberMsg
	config5gMsg.Msg = &msg
	config5gMsg.PrevSlice = getSliceByName(msg.SliceName)
	updateConfig(&config5gMsg)
	filter := bson.M{"slice-name": msg.SliceName}
	sliceDataBsonA := toBsonM(msg.Slice)
	_, errPost := db.CommonDBClient.RestfulAPIPost(db.SliceDataColl, filter, sliceDataBsonA)
	if errPost != nil {
		logger.DbLog.Warnln(errPost)
	}
	updateSMF()
	configLog.Infof("Created Network Slice: %v", sliceName)
	return true
}
