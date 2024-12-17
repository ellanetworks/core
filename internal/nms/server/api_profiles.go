package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/logger"
)

type CreateProfileParams struct {
	Name  string   `json:"name"`
	Imsis []string `json:"imsis"`

	Dnn            string `json:"dnn,omitempty"`
	UeIpPool       string `json:"ue-ip-pool,omitempty"`
	DnsPrimary     string `json:"dns-primary,omitempty"`
	DnsSecondary   string `json:"dns-secondary,omitempty"`
	Mtu            int32  `json:"mtu,omitempty"`
	DnnMbrUplink   int64  `json:"dnn-mbr-uplink,omitempty"`
	DnnMbrDownlink int64  `json:"dnn-mbr-downlink,omitempty"`
	BitrateUnit    string `json:"bitrate-unit,omitempty"`
	Qci            int32  `json:"qci,omitempty"`
	Arp            int32  `json:"arp,omitempty"`
	Pdb            int32  `json:"pdb,omitempty"`
	Pelr           int32  `json:"pelr,omitempty"`
}

type GetProfileResponse struct {
	Name  string   `json:"name"`
	Imsis []string `json:"imsis"`

	Dnn            string `json:"dnn,omitempty"`
	UeIpPool       string `json:"ue-ip-pool,omitempty"`
	DnsPrimary     string `json:"dns-primary,omitempty"`
	DnsSecondary   string `json:"dns-secondary,omitempty"`
	Mtu            int32  `json:"mtu,omitempty"`
	DnnMbrUplink   int64  `json:"dnn-mbr-uplink,omitempty"`
	DnnMbrDownlink int64  `json:"dnn-mbr-downlink,omitempty"`
	BitrateUnit    string `json:"bitrate-unit,omitempty"`
	Qci            int32  `json:"qci,omitempty"`
	Arp            int32  `json:"arp,omitempty"`
	Pdb            int32  `json:"pdb,omitempty"`
	Pelr           int32  `json:"pelr,omitempty"`
}

func convertToString(val uint64) string {
	var mbVal, gbVal, kbVal uint64
	kbVal = val / 1000
	mbVal = val / 1000000
	gbVal = val / 1000000000
	var retStr string
	if gbVal != 0 {
		retStr = strconv.FormatUint(gbVal, 10) + " Gbps"
	} else if mbVal != 0 {
		retStr = strconv.FormatUint(mbVal, 10) + " Mbps"
	} else if kbVal != 0 {
		retStr = strconv.FormatUint(kbVal, 10) + " Kbps"
	} else {
		retStr = strconv.FormatUint(val, 10) + " bps"
	}

	return retStr
}

func ListProfiles(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		dbProfiles, err := dbInstance.ListProfiles()
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to retrieve device groups"})
			return
		}
		var profileList []GetProfileResponse
		for _, profile := range dbProfiles {
			deviceGroup := GetProfileResponse{
				Name:           profile.Name,
				UeIpPool:       profile.UeIpPool,
				DnsPrimary:     profile.DnsPrimary,
				DnsSecondary:   profile.DnsSecondary,
				DnnMbrDownlink: profile.DnnMbrDownlink,
				DnnMbrUplink:   profile.DnnMbrUplink,
				BitrateUnit:    profile.BitrateUnit,
				Qci:            profile.Qci,
				Arp:            profile.Arp,
				Pdb:            profile.Pdb,
				Pelr:           profile.Pelr,
			}
			imsis, err := profile.GetImsis()
			if err != nil {
				logger.NmsLog.Warnln(err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to retrieve device groups"})
				return
			}
			deviceGroup.Imsis = imsis
			profileList = append(profileList, deviceGroup)
		}
		c.JSON(http.StatusOK, profileList)
	}
}

func GetProfile(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		groupName, exists := c.Params.Get("name")
		if !exists {
			logger.NmsLog.Errorf("name is missing")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing name parameter"})
			return
		}
		dbProfile, err := dbInstance.GetProfile(groupName)
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusNotFound, gin.H{"error": "Unable to retrieve device group"})
			return
		}

		deviceGroup := GetProfileResponse{
			Name:           dbProfile.Name,
			UeIpPool:       dbProfile.UeIpPool,
			DnsPrimary:     dbProfile.DnsPrimary,
			DnsSecondary:   dbProfile.DnsSecondary,
			Mtu:            dbProfile.Mtu,
			DnnMbrDownlink: dbProfile.DnnMbrDownlink,
			DnnMbrUplink:   dbProfile.DnnMbrUplink,
			BitrateUnit:    dbProfile.BitrateUnit,
			Qci:            dbProfile.Qci,
			Arp:            dbProfile.Arp,
			Pdb:            dbProfile.Pdb,
			Pelr:           dbProfile.Pelr,
		}
		imsis, err := dbProfile.GetImsis()
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to retrieve device group"})
			return
		}
		deviceGroup.Imsis = imsis
		c.JSON(http.StatusOK, deviceGroup)
	}
}

func CreateProfile(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		var createProfileParams CreateProfileParams
		err := c.ShouldBindJSON(&createProfileParams)
		if err != nil {
			logger.NmsLog.Errorf(" err %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
			return
		}
		if createProfileParams.Name == "" {
			logger.NmsLog.Errorf("name is missing")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing name parameter"})
			return
		}
		if createProfileParams.UeIpPool == "" {
			logger.NmsLog.Errorf("ue-ip-pool is missing")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing ue-ip-pool parameter"})
			return
		}
		if createProfileParams.DnsPrimary == "" {
			logger.NmsLog.Errorf("dns-primary is missing")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing dns-primary parameter"})
			return
		}
		if createProfileParams.Mtu == 0 {
			logger.NmsLog.Errorf("mtu is missing")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing mtu parameter"})
			return
		}
		if createProfileParams.DnnMbrUplink == 0 {
			logger.NmsLog.Errorf("dnn-mbr-uplink is missing")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing dnn-mbr-uplink parameter"})
			return
		}
		if createProfileParams.DnnMbrDownlink == 0 {
			logger.NmsLog.Errorf("dnn-mbr-downlink is missing")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing dnn-mbr-downlink parameter"})
			return
		}
		if createProfileParams.BitrateUnit == "" {
			logger.NmsLog.Errorf("bitrate-unit is missing")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing bitrate-unit parameter"})
			return
		}
		if createProfileParams.Qci == 0 {
			logger.NmsLog.Errorf("qci is missing")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing qci parameter"})
			return
		}
		if createProfileParams.Arp == 0 {
			logger.NmsLog.Errorf("arp is missing")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing arp parameter"})
			return
		}
		if createProfileParams.Pdb == 0 {
			logger.NmsLog.Errorf("pdb is missing")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing pdb parameter"})
			return
		}
		if createProfileParams.Pelr == 0 {
			logger.NmsLog.Errorf("pelr is missing")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing pelr parameter"})
			return
		}
		_, err = dbInstance.GetProfile(createProfileParams.Name)
		if err == nil {
			logger.NmsLog.Warnf("Device Group [%v] already exists", createProfileParams.Name)
			c.JSON(http.StatusConflict, gin.H{"error": "Device Group already exists"})
			return
		}

		slice := isProfileExistInSlice(dbInstance, createProfileParams.Name)
		if slice != nil {
			sVal, err := strconv.ParseUint(slice.Sst, 10, 32)
			if err != nil {
				logger.NmsLog.Errorf("Could not parse SST %v", slice.Sst)
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid SST"})
				return
			}

			for _, imsi := range createProfileParams.Imsis {
				dnn := createProfileParams.Dnn
				ueId := "imsi-" + imsi
				plmnId := slice.Mcc + slice.Mnc
				bitRateUplink := convertToString(uint64(createProfileParams.DnnMbrUplink))
				bitRateDownlink := convertToString(uint64(createProfileParams.DnnMbrDownlink))
				var5qi := 9
				priorityLevel := 8
				err = dbInstance.UpdateSubscriberProfile(ueId, dnn, slice.Sd, int32(sVal), plmnId, bitRateUplink, bitRateDownlink, var5qi, priorityLevel)
				if err != nil {
					logger.NmsLog.Warnf("Could not update subscriber %v", ueId)
					c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to update subscriber"})
					return
				}
			}
		}
		dbProfile := &db.Profile{
			Name:           createProfileParams.Name,
			UeIpPool:       createProfileParams.UeIpPool,
			DnsPrimary:     createProfileParams.DnsPrimary,
			DnsSecondary:   createProfileParams.DnsSecondary,
			Mtu:            createProfileParams.Mtu,
			DnnMbrDownlink: createProfileParams.DnnMbrDownlink,
			DnnMbrUplink:   createProfileParams.DnnMbrUplink,
			BitrateUnit:    createProfileParams.BitrateUnit,
			Qci:            createProfileParams.Qci,
			Arp:            createProfileParams.Arp,
			Pdb:            createProfileParams.Pdb,
			Pelr:           createProfileParams.Pelr,
		}
		dbProfile.SetImsis(createProfileParams.Imsis)
		err = dbInstance.CreateProfile(dbProfile)
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create device group"})
			return
		}
		updateSMF(dbInstance)
		logger.NmsLog.Infof("Created Profile: %v", createProfileParams.Name)
		c.JSON(http.StatusCreated, gin.H{"message": "Profile created successfully"})
	}
}

func DeleteProfile(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		groupName, exists := c.Params.Get("name")
		if !exists {
			logger.NmsLog.Errorf("name is missing")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing name parameter"})
			return
		}
		profile, err := dbInstance.GetProfile(groupName)
		if err != nil {
			logger.NmsLog.Warnf("Device Group [%v] not found", groupName)
			c.JSON(http.StatusNotFound, gin.H{"error": "Device Group not found"})
			return
		}
		deleteProfileConfig(dbInstance, profile)
		err = dbInstance.DeleteProfile(groupName)
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete device group"})
			return
		}
		updateSMF(dbInstance)
		logger.NmsLog.Infof("Deleted Device Group: %v", groupName)
		c.JSON(http.StatusOK, gin.H{"message": "Device Group deleted successfully"})
	}
}

func deleteProfileConfig(dbInstance *db.Database, deviceGroup *db.Profile) {
	slice := isProfileExistInSlice(dbInstance, deviceGroup.Name)
	if slice != nil {
		dimsis, err := deviceGroup.GetImsis()
		if err != nil {
			logger.NmsLog.Warnln(err)
			return
		}
		for _, imsi := range dimsis {
			ueId := "imsi-" + imsi
			err = dbInstance.UpdateSubscriberProfile(ueId, "", "", 0, "", "", "", 0, 0)
			if err != nil {
				logger.NmsLog.Warnln(err)
			}
		}
	}
}

func isProfileExistInSlice(dbInstance *db.Database, deviceGroupName string) *db.NetworkSlice {
	dBSlices, err := dbInstance.ListNetworkSlices()
	if err != nil {
		logger.NmsLog.Warnln(err)
		return nil
	}
	for name, slice := range dBSlices {
		profiles := slice.ListProfiles()
		for _, dgName := range profiles {
			if dgName == deviceGroupName {
				logger.NmsLog.Infof("Device Group [%v] is part of slice: %v", dgName, name)
				return &slice
			}
		}
	}

	return nil
}
