package server

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/nms/models"
	"github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/util/httpwrapper"
)

const DNN = "internet"

const (
	KPS = 1000
	MPS = 1000000
	GPS = 1000000000
)

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

func GetNetworkSlices(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		networkSlices, err := dbInstance.ListNetworkSlices()
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusNotFound, gin.H{"error": "Unable to retrieve network slices"})
			return
		}
		c.JSON(http.StatusOK, networkSlices)
	}
}

func convertDBNetworkSliceToNetworkSlice(dbNetworkSlice *db.NetworkSlice) *models.Slice {
	networkSlice := &models.Slice{
		SliceName: dbNetworkSlice.Name,
		SliceId: models.SliceSliceId{
			Sst: dbNetworkSlice.Sst,
			Sd:  dbNetworkSlice.Sd,
		},
		SiteDeviceGroup: dbNetworkSlice.GetDeviceGroups(),
		SiteInfo: models.SliceSiteInfo{
			Plmn: models.SliceSiteInfoPlmn{
				Mcc: dbNetworkSlice.Mcc,
				Mnc: dbNetworkSlice.Mnc,
			},
			GNodeBs: make([]models.SliceSiteInfoGNodeBs, 0),
			Upf:     make(map[string]interface{}),
		},
		ApplicationFilteringRules: make([]models.SliceApplicationFilteringRules, 0),
	}
	dbGnodeBs, err := dbNetworkSlice.GetGNodeBs()
	if err != nil {
		logger.NmsLog.Warnln(err)
	}
	for _, dbRadio := range dbGnodeBs {
		radio := models.SliceSiteInfoGNodeBs{
			Name: dbRadio.Name,
			Tac:  dbRadio.Tac,
		}
		networkSlice.SiteInfo.GNodeBs = append(networkSlice.SiteInfo.GNodeBs, radio)
	}
	dbUpf, err := dbNetworkSlice.GetUpf()
	if err != nil {
		logger.NmsLog.Warnln(err)
	}
	networkSlice.SiteInfo.Upf["upf-name"] = dbUpf.Name
	networkSlice.SiteInfo.Upf["upf-port"] = dbUpf.Port
	return networkSlice
}

func convertNetworkSliceToDBNetworkSlice(networkSlice *models.Slice) *db.NetworkSlice {
	upfPort := networkSlice.SiteInfo.Upf["upf-port"].(string)
	upfName := networkSlice.SiteInfo.Upf["upf-name"].(string)
	upfPortInt, err := strconv.Atoi(upfPort)
	if err != nil {
		logger.NmsLog.Warnln(err)
		return nil
	}
	dbNetworkSlice := &db.NetworkSlice{
		Name: networkSlice.SliceName,
		Sst:  networkSlice.SliceId.Sst,
		Sd:   networkSlice.SliceId.Sd,
		Mcc:  networkSlice.SiteInfo.Plmn.Mcc,
		Mnc:  networkSlice.SiteInfo.Plmn.Mnc,
	}
	dbNetworkSlice.SetUpf(db.UPF{
		Name: upfName,
		Port: upfPortInt,
	})

	dbNetworkSlice.SetDeviceGroups(networkSlice.SiteDeviceGroup)
	dbGnodeBs := make([]db.GNodeB, 0)
	for _, radio := range networkSlice.SiteInfo.GNodeBs {
		dbRadio := db.GNodeB{
			Name: radio.Name,
			Tac:  radio.Tac,
		}
		dbGnodeBs = append(dbGnodeBs, dbRadio)
	}
	dbNetworkSlice.SetGNodeBs(dbGnodeBs)
	return dbNetworkSlice
}

func GetNetworkSlice(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		name := c.Param("slice-name")
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing slice-name parameter"})
			return
		}
		dbNetworkSlice, err := dbInstance.GetNetworkSliceByName(name)
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusNotFound, gin.H{"error": "Unable to retrieve network slice"})
			return
		}
		networkSlice := convertDBNetworkSliceToNetworkSlice(dbNetworkSlice)
		c.JSON(http.StatusOK, networkSlice)
	}
}

func PostNetworkSlice(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		sliceName, exists := c.Params.Get("slice-name")
		if !exists {
			logger.NmsLog.Errorf("slice-name is missing")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing slice-name parameter"})
			return
		}
		_, err := dbInstance.GetNetworkSliceByName(sliceName)
		if err == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Network slice already exists"})
			return
		}
		var request models.Slice
		s := strings.Split(c.GetHeader("Content-Type"), ";")
		switch s[0] {
		case "application/json":
			err = c.ShouldBindJSON(&request)
		}
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
			return
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
			if bitrate < 0 || bitrate > 2147483647 {
				procReq.ApplicationFilteringRules[index].AppMbrUplink = 2147483647
			} else {
				procReq.ApplicationFilteringRules[index].AppMbrUplink = int32(bitrate)
			}

			bitrate = convertToBps(int64(dl), unit)
			if bitrate < 0 || bitrate > 2147483647 {
				procReq.ApplicationFilteringRules[index].AppMbrDownlink = 2147483647
			} else {
				procReq.ApplicationFilteringRules[index].AppMbrDownlink = int32(bitrate)
			}
		}

		procReq.SliceName = sliceName
		sVal, err := strconv.ParseUint(procReq.SliceId.Sst, 10, 32)
		if err != nil {
			logger.NmsLog.Errorf("Could not parse SST %v", procReq.SliceId.Sst)
		}
		for _, dgName := range procReq.SiteDeviceGroup {
			dbDeviceGroup, err := dbInstance.GetProfileByName(dgName)
			if err != nil {
				logger.NmsLog.Warnf("Could not get device group %v", dgName)
				continue
			}
			for _, imsi := range dbDeviceGroup.Imsis {
				dnn := DNN
				mcc := procReq.SiteInfo.Plmn.Mcc
				mnc := procReq.SiteInfo.Plmn.Mnc
				ueId := "imsi-" + imsi
				subscriber, err := dbInstance.GetSubscriberByUeID(ueId)
				if err != nil {
					logger.NmsLog.Warnf("Could not get subscriber %v", ueId)
					continue
				}
				subscriber.Sst = int32(sVal)
				subscriber.Sd = procReq.SliceId.Sd
				subscriber.Dnn = dnn
				subscriber.PlmnID = mcc + mnc
				subscriber.BitRateDownlink = convertToString(uint64(dbDeviceGroup.DnnMbrDownlink))
				subscriber.BitRateUplink = convertToString(uint64(dbDeviceGroup.DnnMbrUplink))

				subscriber.Var5qi = 9
				subscriber.PriorityLevel = 8
				err = dbInstance.CreateSubscriber(subscriber)
				if err != nil {
					logger.NmsLog.Warnf("Could not create subscriber %v", ueId)
					continue
				}
			}
		}
		dbNetworkSlice := convertNetworkSliceToDBNetworkSlice(&procReq)
		err = dbInstance.CreateNetworkSlice(dbNetworkSlice)
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create network slice"})
			return
		}
		updateSMF(dbInstance)
		c.JSON(http.StatusOK, gin.H{"message": "Network slice created successfully"})
	}
}

func DeleteNetworkSlice(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		sliceName, exists := c.Params.Get("slice-name")
		if !exists {
			logger.NmsLog.Errorf("slice-name is missing")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing slice-name parameter"})
			return
		}
		dbNetworkSlice, err := dbInstance.GetNetworkSliceByName(sliceName)
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusNotFound, gin.H{"error": "Unable to retrieve network slice"})
			return
		}
		err = dbInstance.DeleteNetworkSlice(dbNetworkSlice.ID)
		if err != nil {
			logger.NmsLog.Warnln(err)
		}
		prevSlice := convertDBNetworkSliceToNetworkSlice(dbNetworkSlice)
		dgnames := getDeleteGroupsList(nil, prevSlice)
		for _, dgname := range dgnames {
			devGroupConfig, err := dbInstance.GetProfileByName(dgname)
			if err != nil {
				logger.NmsLog.Warnln(err)
				continue
			}
			for _, imsi := range devGroupConfig.Imsis {
				ueId := "imsi-" + imsi
				subscriber, err := dbInstance.GetSubscriberByUeID(ueId)
				if err != nil {
					logger.NmsLog.Warnln(err)
					continue
				}
				subscriber.BitRateDownlink = ""
				subscriber.BitRateUplink = ""
				subscriber.Var5qi = 0
				subscriber.PriorityLevel = 0
				err = dbInstance.CreateSubscriber(subscriber)
				if err != nil {
					logger.NmsLog.Warnln(err)
				}
			}
		}
		updateSMF(dbInstance)
		c.JSON(http.StatusOK, gin.H{"message": "Network slice deleted successfully"})
	}
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

func updateSMF(dbInstance *db.Database) {
	networkSlices := make([]*models.Slice, 0)
	dbNetworkSlices, err := dbInstance.ListNetworkSlices()
	if err != nil {
		logger.NmsLog.Warnln(err)
	}
	for _, dbNetworkSlice := range dbNetworkSlices {
		networkSlice := convertDBNetworkSliceToNetworkSlice(&dbNetworkSlice)
		networkSlices = append(networkSlices, networkSlice)
	}
	deviceGroups := make([]models.DeviceGroups, 0)
	dbDeviceGroups, err := dbInstance.ListProfiles()
	if err != nil {
		logger.NmsLog.Warnln(err)
	}
	for _, dbDeviceGroup := range dbDeviceGroups {
		deviceGroup := models.DeviceGroups{
			DeviceGroupName: dbDeviceGroup.Name,
			Imsis:           dbDeviceGroup.Imsis,
			IpDomainExpanded: models.DeviceGroupsIpDomainExpanded{
				Dnn:          DNN,
				UeIpPool:     dbDeviceGroup.UeIpPool,
				DnsPrimary:   dbDeviceGroup.DnsPrimary,
				DnsSecondary: dbDeviceGroup.DnsSecondary,
				UeDnnQos: &models.DeviceGroupsIpDomainExpandedUeDnnQos{
					DnnMbrDownlink: dbDeviceGroup.DnnMbrDownlink,
					DnnMbrUplink:   dbDeviceGroup.DnnMbrUplink,
					BitrateUnit:    dbDeviceGroup.BitrateUnit,
					TrafficClass: &models.TrafficClassInfo{
						Name: dbDeviceGroup.Name,
						Qci:  dbDeviceGroup.Qci,
						Arp:  dbDeviceGroup.Arp,
						Pdb:  dbDeviceGroup.Pdb,
						Pelr: dbDeviceGroup.Pelr,
					},
				},
			},
		}
		deviceGroups = append(deviceGroups, deviceGroup)
	}
	context.UpdateSMFContext(networkSlices, deviceGroups)
}
