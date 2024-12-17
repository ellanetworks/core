package server

import (
	"net/http"
	"slices"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/nms/models"
	"github.com/yeastengine/ella/internal/smf/context"
)

const DNN = "internet"

const (
	KPS = 1000
	MPS = 1000000
	GPS = 1000000000
)

func ListNetworkSlices(dbInstance *db.Database) gin.HandlerFunc {
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
		SiteDeviceGroup: dbNetworkSlice.ListProfiles(),
		SiteInfo: models.SliceSiteInfo{
			Plmn: models.SliceSiteInfoPlmn{
				Mcc: dbNetworkSlice.Mcc,
				Mnc: dbNetworkSlice.Mnc,
			},
			GNodeBs: make([]models.SliceSiteInfoGNodeBs, 0),
			Upf:     make(map[string]interface{}),
		},
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

	dbNetworkSlice.SetProfiles(networkSlice.SiteDeviceGroup)
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
		dbNetworkSlice, err := dbInstance.GetNetworkSlice(name)
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusNotFound, gin.H{"error": "Unable to retrieve network slice"})
			return
		}
		networkSlice := convertDBNetworkSliceToNetworkSlice(dbNetworkSlice)
		c.JSON(http.StatusOK, networkSlice)
	}
}

func CreateNetworkSlice(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		var createNetworkSliceParams models.Slice
		err := c.ShouldBindJSON(&createNetworkSliceParams)
		if err != nil {
			logger.NmsLog.Errorf(" err %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
			return
		}

		_, err = dbInstance.GetNetworkSlice(createNetworkSliceParams.SliceName)
		if err == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Network slice already exists"})
			return
		}

		group := createNetworkSliceParams.SiteDeviceGroup
		slices.Sort(group)

		sVal, err := strconv.ParseUint(createNetworkSliceParams.SliceId.Sst, 10, 32)
		if err != nil {
			logger.NmsLog.Errorf("Could not parse SST %v", createNetworkSliceParams.SliceId.Sst)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid SST"})
			return
		}
		for _, dgName := range createNetworkSliceParams.SiteDeviceGroup {
			dbDeviceGroup, err := dbInstance.GetProfile(dgName)
			if err != nil {
				logger.NmsLog.Warnf("Could not get device group %v", dgName)
				continue
			}
			imsis, err := dbDeviceGroup.GetImsis()
			if err != nil {
				logger.NmsLog.Warnf("Could not get imsis %v", dbDeviceGroup.Imsis)
				continue
			}
			for _, imsi := range imsis {
				mcc := createNetworkSliceParams.SiteInfo.Plmn.Mcc
				mnc := createNetworkSliceParams.SiteInfo.Plmn.Mnc
				ueId := "imsi-" + imsi
				sst := int32(sVal)
				sd := createNetworkSliceParams.SliceId.Sd
				plmnID := mcc + mnc
				bitRateUplink := convertToString(uint64(dbDeviceGroup.DnnMbrUplink))
				bitRateDownlink := convertToString(uint64(dbDeviceGroup.DnnMbrDownlink))
				var5qi := 9
				priorityLevel := 8
				err = dbInstance.UpdateSubscriberProfile(ueId, DNN, sd, sst, plmnID, bitRateUplink, bitRateDownlink, var5qi, priorityLevel)
				if err != nil {
					logger.NmsLog.Warnf("Could not update subscriber %v", ueId)
					continue
				}
			}
		}
		dbNetworkSlice := convertNetworkSliceToDBNetworkSlice(&createNetworkSliceParams)
		err = dbInstance.CreateNetworkSlice(dbNetworkSlice)
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create network slice"})
			return
		}
		updateSMF(dbInstance)
		logger.NmsLog.Infof("Network slice %s created successfully", createNetworkSliceParams.SliceName)
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
		dbNetworkSlice, err := dbInstance.GetNetworkSlice(sliceName)
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusNotFound, gin.H{"error": "Unable to retrieve network slice"})
			return
		}
		err = dbInstance.DeleteNetworkSlice(sliceName)
		if err != nil {
			logger.NmsLog.Warnln(err)
		}
		prevSlice := convertDBNetworkSliceToNetworkSlice(dbNetworkSlice)
		dgnames := getDeleteGroupsList(nil, prevSlice)
		for _, dgname := range dgnames {
			devGroupConfig, err := dbInstance.GetProfile(dgname)
			if err != nil {
				logger.NmsLog.Warnln(err)
				continue
			}
			imsis, err := devGroupConfig.GetImsis()
			if err != nil {
				logger.NmsLog.Warnln(err)
				continue
			}
			for _, imsi := range imsis {
				ueId := "imsi-" + imsi
				err = dbInstance.UpdateSubscriberProfile(ueId, DNN, "", 0, "", "", "", 0, 0)
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
	deviceGroups := make([]models.Profile, 0)
	dbDeviceGroups, err := dbInstance.ListProfiles()
	if err != nil {
		logger.NmsLog.Warnln(err)
	}
	for _, dbDeviceGroup := range dbDeviceGroups {
		deviceGroup := models.Profile{
			Name:           dbDeviceGroup.Name,
			Dnn:            DNN,
			UeIpPool:       dbDeviceGroup.UeIpPool,
			DnsPrimary:     dbDeviceGroup.DnsPrimary,
			DnsSecondary:   dbDeviceGroup.DnsSecondary,
			DnnMbrDownlink: dbDeviceGroup.DnnMbrDownlink,
			DnnMbrUplink:   dbDeviceGroup.DnnMbrUplink,
			BitrateUnit:    dbDeviceGroup.BitrateUnit,
			Qci:            dbDeviceGroup.Qci,
			Arp:            dbDeviceGroup.Arp,
			Pdb:            dbDeviceGroup.Pdb,
			Pelr:           dbDeviceGroup.Pelr,
		}
		imsis, err := dbDeviceGroup.GetImsis()
		if err != nil {
			logger.NmsLog.Warnln(err)
		}
		deviceGroup.Imsis = imsis
		deviceGroups = append(deviceGroups, deviceGroup)
	}
	context.UpdateSMFContext(networkSlices, deviceGroups)
}
