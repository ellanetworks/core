package server

import (
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	openAPIModels "github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/nms/logger"
	"github.com/yeastengine/ella/internal/nms/models"
	"github.com/yeastengine/ella/internal/smf/context"
)

const (
	KPS = 1000
	MPS = 1000000
	GPS = 1000000000
)

var imsiData map[string]*openAPIModels.AuthenticationSubscription

func init() {
	imsiData = make(map[string]*openAPIModels.AuthenticationSubscription)
}

func GetNetworkSlices(c *gin.Context) {
	setCorsHeader(c)
	networkSlices, err := db.ListNetworkSliceNames()
	if err != nil {
		logger.NMSLog.Warnln(err)
		c.JSON(http.StatusInternalServerError, nil)
		return
	}
	c.JSON(http.StatusOK, networkSlices)
}

func convertDBNetworkSliceToNetworkSlice(dbNetworkSlice *db.Slice) *models.Slice {
	networkSlice := &models.Slice{
		SliceName: dbNetworkSlice.SliceName,
		SliceId: models.SliceSliceId{
			Sst: dbNetworkSlice.SliceId.Sst,
			Sd:  dbNetworkSlice.SliceId.Sd,
		},
		SiteDeviceGroup: dbNetworkSlice.SiteDeviceGroup,
		SiteInfo: models.SliceSiteInfo{
			SiteName: dbNetworkSlice.SiteInfo.SiteName,
			Plmn: models.SliceSiteInfoPlmn{
				Mcc: dbNetworkSlice.SiteInfo.Plmn.Mcc,
				Mnc: dbNetworkSlice.SiteInfo.Plmn.Mnc,
			},
			GNodeBs: make([]models.SliceSiteInfoGNodeBs, 0),
			Upf:     make(map[string]interface{}),
		},
		ApplicationFilteringRules: make([]models.SliceApplicationFilteringRules, 0),
	}
	for _, dbGnb := range dbNetworkSlice.SiteInfo.GNodeBs {
		gnb := models.SliceSiteInfoGNodeBs{
			Name: dbGnb.Name,
			Tac:  dbGnb.Tac,
		}
		networkSlice.SiteInfo.GNodeBs = append(networkSlice.SiteInfo.GNodeBs, gnb)
	}
	for key, value := range dbNetworkSlice.SiteInfo.Upf {
		networkSlice.SiteInfo.Upf[key] = value
	}
	for _, dbAppFilterRule := range dbNetworkSlice.ApplicationFilteringRules {
		appFilterRule := models.SliceApplicationFilteringRules{
			RuleName:       dbAppFilterRule.RuleName,
			Priority:       dbAppFilterRule.Priority,
			Action:         dbAppFilterRule.Action,
			Endpoint:       dbAppFilterRule.Endpoint,
			Protocol:       dbAppFilterRule.Protocol,
			StartPort:      dbAppFilterRule.StartPort,
			EndPort:        dbAppFilterRule.EndPort,
			AppMbrUplink:   dbAppFilterRule.AppMbrUplink,
			AppMbrDownlink: dbAppFilterRule.AppMbrDownlink,
			BitrateUnit:    dbAppFilterRule.BitrateUnit,
			TrafficClass: &models.TrafficClassInfo{
				Name: dbAppFilterRule.TrafficClass.Name,
				Qci:  dbAppFilterRule.TrafficClass.Qci,
				Arp:  dbAppFilterRule.TrafficClass.Arp,
				Pdb:  dbAppFilterRule.TrafficClass.Pdb,
				Pelr: dbAppFilterRule.TrafficClass.Pelr,
			},
			RuleTrigger: dbAppFilterRule.RuleTrigger,
		}
		networkSlice.ApplicationFilteringRules = append(networkSlice.ApplicationFilteringRules, appFilterRule)
	}
	return networkSlice
}

func convertNetworkSliceToDBNetworkSlice(networkSlice *models.Slice) *db.Slice {
	dbNetworkSlice := &db.Slice{
		SliceName: networkSlice.SliceName,
		SliceId: db.SliceSliceId{
			Sst: networkSlice.SliceId.Sst,
			Sd:  networkSlice.SliceId.Sd,
		},
		SiteDeviceGroup: networkSlice.SiteDeviceGroup,
		SiteInfo: db.SliceSiteInfo{
			SiteName: networkSlice.SiteInfo.SiteName,
			Plmn: db.SliceSiteInfoPlmn{
				Mcc: networkSlice.SiteInfo.Plmn.Mcc,
				Mnc: networkSlice.SiteInfo.Plmn.Mnc,
			},
			GNodeBs: make([]db.SliceSiteInfoGNodeBs, 0),
			Upf:     make(map[string]interface{}),
		},
		ApplicationFilteringRules: make([]db.SliceApplicationFilteringRules, 0),
	}
	for _, gnb := range networkSlice.SiteInfo.GNodeBs {
		dbGnb := db.SliceSiteInfoGNodeBs{
			Name: gnb.Name,
			Tac:  gnb.Tac,
		}
		dbNetworkSlice.SiteInfo.GNodeBs = append(dbNetworkSlice.SiteInfo.GNodeBs, dbGnb)
	}
	for key, value := range networkSlice.SiteInfo.Upf {
		dbNetworkSlice.SiteInfo.Upf[key] = value
	}
	for _, appFilterRule := range networkSlice.ApplicationFilteringRules {
		dbAppFilterRule := db.SliceApplicationFilteringRules{
			RuleName:       appFilterRule.RuleName,
			Priority:       appFilterRule.Priority,
			Action:         appFilterRule.Action,
			Endpoint:       appFilterRule.Endpoint,
			Protocol:       appFilterRule.Protocol,
			StartPort:      appFilterRule.StartPort,
			EndPort:        appFilterRule.EndPort,
			AppMbrUplink:   appFilterRule.AppMbrUplink,
			AppMbrDownlink: appFilterRule.AppMbrDownlink,
			BitrateUnit:    appFilterRule.BitrateUnit,
			TrafficClass: &db.TrafficClassInfo{
				Name: appFilterRule.TrafficClass.Name,
				Qci:  appFilterRule.TrafficClass.Qci,
				Arp:  appFilterRule.TrafficClass.Arp,
				Pdb:  appFilterRule.TrafficClass.Pdb,
				Pelr: appFilterRule.TrafficClass.Pelr,
			},
			RuleTrigger: appFilterRule.RuleTrigger,
		}
		dbNetworkSlice.ApplicationFilteringRules = append(dbNetworkSlice.ApplicationFilteringRules, dbAppFilterRule)
	}
	return dbNetworkSlice
}

func GetNetworkSliceByName(c *gin.Context) {
	setCorsHeader(c)
	logger.NMSLog.Infoln("Get Network Slice by name")
	dbNetworkSlice, err := db.GetNetworkSliceByName(c.Param("slice-name"))
	if err != nil {
		logger.NMSLog.Warnln(err)
		c.JSON(http.StatusInternalServerError, nil)
		return
	}
	if dbNetworkSlice.SliceName == "" {
		c.JSON(http.StatusNotFound, nil)
		return
	}
	networkSlice := convertDBNetworkSliceToNetworkSlice(dbNetworkSlice)
	c.JSON(http.StatusOK, networkSlice)
}

func NetworkSliceSliceNameDelete(c *gin.Context) {
	if ret := NetworkSliceDeleteHandler(c); ret {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{})
	}
}

func NetworkSliceSliceNamePost(c *gin.Context) {
	if ret := NetworkSlicePostHandler(c, models.Post_op); ret {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{})
	}
}

func NetworkSliceSliceNamePut(c *gin.Context) {
	if ret := NetworkSlicePostHandler(c, models.Put_op); ret {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{})
	}
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

func NetworkSliceDeleteHandler(c *gin.Context) bool {
	sliceName, exists := c.Params.Get("slice-name")
	if !exists {
		logger.ConfigLog.Errorf("slice-name is missing")
		return false
	}
	prevdbSlice, err := db.GetNetworkSliceByName(sliceName)
	if err != nil {
		logger.NMSLog.Warnln(err)
	}
	err = db.DeleteNetworkSlice(sliceName)
	if err != nil {
		logger.NMSLog.Warnln(err)
	}
	prevSlice := convertDBNetworkSliceToNetworkSlice(prevdbSlice)
	dgnames := getDeleteGroupsList(nil, prevSlice)
	for _, dgname := range dgnames {
		devGroupConfig := db.GetDeviceGroupByName(dgname)
		if devGroupConfig != nil {
			for _, imsi := range devGroupConfig.Imsis {
				mcc := prevSlice.SiteInfo.Plmn.Mcc
				mnc := prevSlice.SiteInfo.Plmn.Mnc
				err := db.DeleteAmPolicy(imsi)
				if err != nil {
					logger.NMSLog.Warnln(err)
				}
				err = db.DeleteSmPolicy(imsi)
				if err != nil {
					logger.NMSLog.Warnln(err)
				}
				err = db.DeleteAmData(imsi, mcc, mnc)
				if err != nil {
					logger.NMSLog.Warnln(err)
				}
				err = db.DeleteSmData(imsi, mcc, mnc)
				if err != nil {
					logger.NMSLog.Warnln(err)
				}
				err = db.DeleteSmfSelection(imsi, mcc, mnc)
				if err != nil {
					logger.NMSLog.Warnln(err)
				}
			}
		}
	}
	updateSMF()
	logger.ConfigLog.Infof("Deleted Network Slice: %v", sliceName)
	return true
}

func NetworkSlicePostHandler(c *gin.Context, msgOp int) bool {
	sliceName, exists := c.Params.Get("slice-name")
	if !exists {
		logger.ConfigLog.Errorf("slice-name is missing")
		return false
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

	procReq.SliceName = sliceName
	sVal, err := strconv.ParseUint(procReq.SliceId.Sst, 10, 32)
	if err != nil {
		logger.NMSLog.Errorf("Could not parse SST %v", procReq.SliceId.Sst)
	}
	snssai := &db.Snssai{
		Sd:  procReq.SliceId.Sd,
		Sst: int32(sVal),
	}
	for _, dgName := range procReq.SiteDeviceGroup {
		dbDeviceGroup := db.GetDeviceGroupByName(dgName)
		if dbDeviceGroup != nil {
			for _, imsi := range dbDeviceGroup.Imsis {
				dnn := dbDeviceGroup.IpDomainExpanded.Dnn
				mcc := procReq.SiteInfo.Plmn.Mcc
				mnc := procReq.SiteInfo.Plmn.Mnc
				err := db.CreateAmPolicyData(imsi)
				if err != nil {
					logger.NMSLog.Warnln(err)
				}
				err = db.CreateSmPolicyData(snssai, dnn, imsi)
				if err != nil {
					logger.NMSLog.Warnln(err)
				}
				err = db.CreateAmProvisionedData(snssai, dbDeviceGroup.IpDomainExpanded.UeDnnQos, mcc, mnc, imsi)
				if err != nil {
					logger.NMSLog.Warnln(err)
				}
				err = db.CreateSmProvisionedData(snssai, dbDeviceGroup.IpDomainExpanded.UeDnnQos, mcc, mnc, dnn, imsi)
				if err != nil {
					logger.NMSLog.Warnln(err)
				}
				err = db.CreateSmfSelectionProviosionedData(snssai, mcc, mnc, dnn, imsi)
				if err != nil {
					logger.NMSLog.Warnln(err)
				}
			}
		}
	}
	dbNetworkSlice := convertNetworkSliceToDBNetworkSlice(&procReq)
	err = db.CreateNetworkSlice(dbNetworkSlice)
	if err != nil {
		logger.NMSLog.Warnln(err)
	}
	updateSMF()
	logger.ConfigLog.Infof("Created Network Slice: %v", sliceName)
	return true
}

func getDeleteGroupsList(slice, prevSlice *models.Slice) (names []string) {
	for prevSlice == nil {
		return
	}

	if slice != nil {
		for _, pdgName := range prevSlice.SiteDeviceGroup {
			var found bool
			for _, dgName := range slice.SiteDeviceGroup {
				if dgName == pdgName {
					found = true
					break
				}
			}
			if !found {
				names = append(names, pdgName)
			}
		}
	} else {
		names = append(names, prevSlice.SiteDeviceGroup...)
	}
	return
}

func updateSMF() {
	networkSlices := make([]*models.Slice, 0)
	networkSliceNames, err := db.ListNetworkSliceNames()
	if err != nil {
		logger.NMSLog.Warnln(err)
	}
	for _, networkSliceName := range networkSliceNames {
		dbNetworkSlice, err := db.GetNetworkSliceByName(networkSliceName)
		if err != nil {
			logger.NMSLog.Warnln(err)
			continue
		}
		networkSlice := convertDBNetworkSliceToNetworkSlice(dbNetworkSlice)
		networkSlices = append(networkSlices, networkSlice)
	}
	deviceGroups := make([]models.DeviceGroups, 0)
	deviceGroupNames, err := db.ListDeviceGroupNames()
	if err != nil {
		logger.NMSLog.Warnln(err)
	}
	for _, deviceGroupName := range deviceGroupNames {
		dbDeviceGroup := db.GetDeviceGroupByName(deviceGroupName)
		deviceGroup := models.DeviceGroups{
			DeviceGroupName: dbDeviceGroup.DeviceGroupName,
			Imsis:           dbDeviceGroup.Imsis,
			SiteInfo:        dbDeviceGroup.SiteInfo,
			IpDomainName:    dbDeviceGroup.IpDomainName,
			IpDomainExpanded: models.DeviceGroupsIpDomainExpanded{
				Dnn:          dbDeviceGroup.IpDomainExpanded.Dnn,
				UeIpPool:     dbDeviceGroup.IpDomainExpanded.UeIpPool,
				DnsPrimary:   dbDeviceGroup.IpDomainExpanded.DnsPrimary,
				DnsSecondary: dbDeviceGroup.IpDomainExpanded.DnsSecondary,
				UeDnnQos: &models.DeviceGroupsIpDomainExpandedUeDnnQos{
					DnnMbrDownlink: dbDeviceGroup.IpDomainExpanded.UeDnnQos.DnnMbrDownlink,
					DnnMbrUplink:   dbDeviceGroup.IpDomainExpanded.UeDnnQos.DnnMbrUplink,
					BitrateUnit:    dbDeviceGroup.IpDomainExpanded.UeDnnQos.BitrateUnit,
					TrafficClass: &models.TrafficClassInfo{
						Name: dbDeviceGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Name,
						Qci:  dbDeviceGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Qci,
						Arp:  dbDeviceGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Arp,
						Pdb:  dbDeviceGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Pdb,
						Pelr: dbDeviceGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Pelr,
					},
				},
			},
		}
		deviceGroups = append(deviceGroups, deviceGroup)
	}
	context.UpdateSMFContext(networkSlices, deviceGroups)
}
