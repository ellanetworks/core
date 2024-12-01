package configapi

import (
	"math"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/omec-project/util/httpwrapper"
	"github.com/sirupsen/logrus"
	"github.com/yeastengine/ella/internal/webui/backend/logger"
	"github.com/yeastengine/ella/internal/webui/configmodels"
)

const (
	KPS = 1000
	MPS = 1000000
	GPS = 1000000000
)

var configChannel chan *configmodels.ConfigMessage

var configLog *logrus.Entry

func init() {
	configLog = logger.ConfigLog
}

func SetChannel(cfgChannel chan *configmodels.ConfigMessage) {
	configLog.Infof("Setting configChannel")
	configChannel = cfgChannel
}

func DeviceGroupDeleteHandler(c *gin.Context) bool {
	var groupName string
	var exists bool
	if groupName, exists = c.Params.Get("group-name"); exists {
		configLog.Infof("Received Delete Group %v from Roc/simapp", groupName)
	}
	var msg configmodels.ConfigMessage
	msg.MsgType = configmodels.Device_group
	msg.MsgMethod = configmodels.Delete_op
	msg.DevGroupName = groupName
	configChannel <- &msg
	configLog.Infof("Successfully Added Device Group [%v] with delete_op to config channel.", groupName)
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
	var request configmodels.DeviceGroups
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

	procReq := req.Body.(configmodels.DeviceGroups)
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

	var msg configmodels.ConfigMessage
	procReq.DeviceGroupName = groupName
	msg.MsgType = configmodels.Device_group
	msg.MsgMethod = msgOp
	msg.DevGroup = &procReq
	msg.DevGroupName = groupName
	configChannel <- &msg
	configLog.Infof("Successfully Added Device Group [%v] to config channel.", groupName)
	return true
}

func NetworkSliceDeleteHandler(c *gin.Context) bool {
	var sliceName string
	var exists bool
	if sliceName, exists = c.Params.Get("slice-name"); exists {
		configLog.Infof("Received Deleted slice : %v from Roc/simapp", sliceName)
	}
	var msg configmodels.ConfigMessage
	msg.MsgMethod = configmodels.Delete_op
	msg.MsgType = configmodels.Network_slice
	msg.SliceName = sliceName
	configChannel <- &msg
	configLog.Infof("Successfully Added Device Group [%v] with delete_op to config channel.", sliceName)
	return true
}

func NetworkSlicePostHandler(c *gin.Context, msgOp int) bool {
	var sliceName string
	var exists bool
	if sliceName, exists = c.Params.Get("slice-name"); exists {
		configLog.Infof("Received slice : %v", sliceName)
	}

	var err error
	var request configmodels.Slice
	s := strings.Split(c.GetHeader("Content-Type"), ";")
	switch s[0] {
	case "application/json":
		err = c.ShouldBindJSON(&request)
	}
	if err != nil {
		configLog.Infof(" err %v", err)
		return false
	}
	// configLog.Infof("Printing request full after binding : %v ", request)

	req := httpwrapper.NewRequest(c.Request, request)

	procReq := req.Body.(configmodels.Slice)
	group := procReq.SiteDeviceGroup
	slices.Sort(group)
	configLog.Infof("Number of device groups %v", len(group))
	for i := 0; i < len(group); i++ {
		configLog.Infof("  device groups(%v) - %v \n", i+1, group[i])
	}

	for index, filter := range procReq.ApplicationFilteringRules {
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

		configLog.Infof("\tApp MBR Uplink   : %v", procReq.ApplicationFilteringRules[index].AppMbrUplink)
		configLog.Infof("\tApp MBR Downlink : %v", procReq.ApplicationFilteringRules[index].AppMbrDownlink)
		if filter.TrafficClass != nil {
			configLog.Infof("\t\tTraffic Class : %v", filter.TrafficClass)
		}
	}
	site := procReq.SiteInfo
	for e := 0; e < len(site.GNodeBs); e++ {
		enb := site.GNodeBs[e]
		configLog.Infof("    enb (%v) - name - %v , tac = %v \n", e+1, enb.Name, enb.Tac)
	}

	var msg configmodels.ConfigMessage
	msg.MsgMethod = msgOp
	procReq.SliceName = sliceName
	msg.MsgType = configmodels.Network_slice
	msg.Slice = &procReq
	msg.SliceName = sliceName
	configChannel <- &msg
	configLog.Infof("Successfully Added Slice [%v] to config channel.", sliceName)
	return true
}
