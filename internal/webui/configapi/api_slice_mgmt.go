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
	"github.com/yeastengine/ella/internal/webui/dbadapter"
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
	groupName, exists := c.Params.Get("group-name")
	if !exists {
		configLog.Errorf("Delete group request is missing group-name")
		return false
	}
	filter := bson.M{"group-name": groupName}
	errDelOne := dbadapter.CommonDBClient.RestfulAPIDeleteOne(devGroupDataColl, filter)
	if errDelOne != nil {
		logger.DbLog.Warnln(errDelOne)
	}
	UpdateSMF()
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
	groupName, exists := c.Params.Get("group-name")
	if !exists {
		configLog.Errorf("Post group request is missing group-name")
		return false
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

	filter := bson.M{"group-name": groupName}
	devGroupDataBsonA := toBsonM(&procReq)
	_, errPost := dbadapter.CommonDBClient.RestfulAPIPost(devGroupDataColl, filter, devGroupDataBsonA)
	if errPost != nil {
		logger.DbLog.Warnln(errPost)
	}
	UpdateSMF()
	configLog.Infof("Created Device Group: %v", groupName)
	return true
}

func NetworkSliceDeleteHandler(c *gin.Context) bool {
	sliceName, exists := c.Params.Get("slice-name")
	if !exists {
		configLog.Errorf("Delete slice request is missing slice-name")
		return false
	}

	filter := bson.M{"slice-name": sliceName}
	errDelOne := dbadapter.CommonDBClient.RestfulAPIDeleteOne(sliceDataColl, filter)
	if errDelOne != nil {
		logger.DbLog.Warnln(errDelOne)
	}
	UpdateSMF()
	configLog.Infof("Deleted Network Slice: %v", sliceName)
	return true
}

func NetworkSlicePostHandler(c *gin.Context, msgOp int) bool {
	sliceName, exists := c.Params.Get("slice-name")
	if !exists {
		configLog.Errorf("Post slice request is missing slice-name")
		return false
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

	req := httpwrapper.NewRequest(c.Request, request)

	procReq := req.Body.(configmodels.Slice)
	group := procReq.SiteDeviceGroup
	slices.Sort(group)

	for index, _ := range procReq.ApplicationFilteringRules {
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
	site := procReq.SiteInfo
	for e := 0; e < len(site.GNodeBs); e++ {
		enb := site.GNodeBs[e]
		configLog.Infof("    enb (%v) - name - %v , tac = %v \n", e+1, enb.Name, enb.Tac)
	}

	filter := bson.M{"slice-name": sliceName}
	sliceDataBsonA := toBsonM(&procReq)
	_, errPost := dbadapter.CommonDBClient.RestfulAPIPost(sliceDataColl, filter, sliceDataBsonA)
	if errPost != nil {
		logger.DbLog.Warnln(errPost)
	}

	UpdateSMF()
	configLog.Infof("Created Network Slice: %v", sliceName)
	return true
}
