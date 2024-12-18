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

	Dnn             string `json:"dnn,omitempty"`
	UeIpPool        string `json:"ue-ip-pool,omitempty"`
	DnsPrimary      string `json:"dns-primary,omitempty"`
	DnsSecondary    string `json:"dns-secondary,omitempty"`
	Mtu             int32  `json:"mtu,omitempty"`
	BitrateUplink   int64  `json:"bitrate-uplink,omitempty"`
	BitrateDownlink int64  `json:"bitrate-downlink,omitempty"`
	BitrateUnit     string `json:"bitrate-unit,omitempty"`
	Qci             int32  `json:"qci,omitempty"`
	Arp             int32  `json:"arp,omitempty"`
	Pdb             int32  `json:"pdb,omitempty"`
	Pelr            int32  `json:"pelr,omitempty"`
}

type GetProfileResponse struct {
	Name  string   `json:"name"`
	Imsis []string `json:"imsis"`

	Dnn             string `json:"dnn,omitempty"`
	UeIpPool        string `json:"ue-ip-pool,omitempty"`
	DnsPrimary      string `json:"dns-primary,omitempty"`
	DnsSecondary    string `json:"dns-secondary,omitempty"`
	Mtu             int32  `json:"mtu,omitempty"`
	BitrateUplink   int64  `json:"bitrate-uplink,omitempty"`
	BitrateDownlink int64  `json:"bitrate-downlink,omitempty"`
	BitrateUnit     string `json:"bitrate-unit,omitempty"`
	Qci             int32  `json:"qci,omitempty"`
	Arp             int32  `json:"arp,omitempty"`
	Pdb             int32  `json:"pdb,omitempty"`
	Pelr            int32  `json:"pelr,omitempty"`
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
			writeError(c.Writer, http.StatusInternalServerError, "Profiles not found")
			return
		}
		var profileList []GetProfileResponse
		for _, dbProfile := range dbProfiles {
			profileResponse := GetProfileResponse{
				Name:            dbProfile.Name,
				UeIpPool:        dbProfile.UeIpPool,
				DnsPrimary:      dbProfile.DnsPrimary,
				DnsSecondary:    dbProfile.DnsSecondary,
				BitrateDownlink: dbProfile.BitrateDownlink,
				BitrateUplink:   dbProfile.BitrateUplink,
				BitrateUnit:     dbProfile.BitrateUnit,
				Qci:             dbProfile.Qci,
				Arp:             dbProfile.Arp,
				Pdb:             dbProfile.Pdb,
				Pelr:            dbProfile.Pelr,
			}
			imsis, err := dbProfile.GetImsis()
			if err != nil {
				writeError(c.Writer, http.StatusInternalServerError, "Profile not found")
				return
			}
			profileResponse.Imsis = imsis
			profileList = append(profileList, profileResponse)
		}
		err = writeResponse(c.Writer, profileList, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func GetProfile(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		groupName, exists := c.Params.Get("name")
		if !exists {
			writeError(c.Writer, http.StatusBadRequest, "Missing name parameter")
			return
		}
		dbProfile, err := dbInstance.GetProfile(groupName)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Profile not found")
			return
		}

		profileResponse := GetProfileResponse{
			Name:            dbProfile.Name,
			UeIpPool:        dbProfile.UeIpPool,
			DnsPrimary:      dbProfile.DnsPrimary,
			DnsSecondary:    dbProfile.DnsSecondary,
			Mtu:             dbProfile.Mtu,
			BitrateDownlink: dbProfile.BitrateDownlink,
			BitrateUplink:   dbProfile.BitrateUplink,
			BitrateUnit:     dbProfile.BitrateUnit,
			Qci:             dbProfile.Qci,
			Arp:             dbProfile.Arp,
			Pdb:             dbProfile.Pdb,
			Pelr:            dbProfile.Pelr,
		}
		imsis, err := dbProfile.GetImsis()
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Profile not found")
			return
		}
		profileResponse.Imsis = imsis
		err = writeResponse(c.Writer, profileResponse, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func CreateProfile(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		var createProfileParams CreateProfileParams
		err := c.ShouldBindJSON(&createProfileParams)
		if err != nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid request data")
			return
		}
		if createProfileParams.Name == "" {
			writeError(c.Writer, http.StatusBadRequest, "name is missing")
			return
		}
		if createProfileParams.UeIpPool == "" {
			writeError(c.Writer, http.StatusBadRequest, "ue-ip-pool is missing")
			return
		}
		if createProfileParams.DnsPrimary == "" {
			writeError(c.Writer, http.StatusBadRequest, "dns-primary is missing")
			return
		}
		if createProfileParams.Mtu == 0 {
			writeError(c.Writer, http.StatusBadRequest, "mtu is missing")
			return
		}
		if createProfileParams.BitrateUplink == 0 {
			writeError(c.Writer, http.StatusBadRequest, "bitrate-uplink is missing")
			return
		}
		if createProfileParams.BitrateDownlink == 0 {
			writeError(c.Writer, http.StatusBadRequest, "bitrate-downlink is missing")
			return
		}
		if createProfileParams.BitrateUnit == "" {
			writeError(c.Writer, http.StatusBadRequest, "bitrate-unit is missing")
			return
		}
		if createProfileParams.Qci == 0 {
			writeError(c.Writer, http.StatusBadRequest, "qci is missing")
			return
		}
		if createProfileParams.Arp == 0 {
			writeError(c.Writer, http.StatusBadRequest, "arp is missing")
			return
		}
		if createProfileParams.Pdb == 0 {
			writeError(c.Writer, http.StatusBadRequest, "pdb is missing")
			return
		}
		if createProfileParams.Pelr == 0 {
			writeError(c.Writer, http.StatusBadRequest, "pelr is missing")
			return
		}
		_, err = dbInstance.GetProfile(createProfileParams.Name)
		if err == nil {
			writeError(c.Writer, http.StatusBadRequest, "Profile already exists")
			return
		}

		slice := isProfileExistInSlice(dbInstance, createProfileParams.Name)
		if slice != nil {
			sVal, err := strconv.ParseUint(slice.Sst, 10, 32)
			if err != nil {
				writeError(c.Writer, http.StatusBadRequest, "Invalid SST")
				return
			}

			for _, imsi := range createProfileParams.Imsis {
				dnn := createProfileParams.Dnn
				ueId := "imsi-" + imsi
				plmnId := slice.Mcc + slice.Mnc
				bitRateUplink := convertToString(uint64(createProfileParams.BitrateUplink))
				bitRateDownlink := convertToString(uint64(createProfileParams.BitrateDownlink))
				var5qi := 9
				priorityLevel := 8
				err = dbInstance.UpdateSubscriberProfile(ueId, dnn, slice.Sd, int32(sVal), plmnId, bitRateUplink, bitRateDownlink, var5qi, priorityLevel)
				if err != nil {
					writeError(c.Writer, http.StatusBadRequest, "Failed to update subscriber")
					return
				}
			}
		}
		dbProfile := &db.Profile{
			Name:            createProfileParams.Name,
			UeIpPool:        createProfileParams.UeIpPool,
			DnsPrimary:      createProfileParams.DnsPrimary,
			DnsSecondary:    createProfileParams.DnsSecondary,
			Mtu:             createProfileParams.Mtu,
			BitrateDownlink: createProfileParams.BitrateDownlink,
			BitrateUplink:   createProfileParams.BitrateUplink,
			BitrateUnit:     createProfileParams.BitrateUnit,
			Qci:             createProfileParams.Qci,
			Arp:             createProfileParams.Arp,
			Pdb:             createProfileParams.Pdb,
			Pelr:            createProfileParams.Pelr,
		}
		dbProfile.SetImsis(createProfileParams.Imsis)
		err = dbInstance.CreateProfile(dbProfile)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Failed to create profile")
			return
		}
		updateSMF(dbInstance)
		logger.NmsLog.Infof("Created Profile: %v", createProfileParams.Name)
		response := SuccessResponse{Message: "Profile created successfully"}
		err = writeResponse(c.Writer, response, http.StatusCreated)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func DeleteProfile(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		groupName, exists := c.Params.Get("name")
		if !exists {
			writeError(c.Writer, http.StatusBadRequest, "Missing name parameter")
			return
		}
		profile, err := dbInstance.GetProfile(groupName)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Profile not found")
			return
		}
		deleteProfileConfig(dbInstance, profile)
		err = dbInstance.DeleteProfile(groupName)
		if err != nil {
			writeResponse(c.Writer, gin.H{"error": "Failed to delete profile"}, http.StatusInternalServerError)
			return
		}
		updateSMF(dbInstance)
		logger.NmsLog.Infof("Deleted Profile: %v", groupName)
		response := SuccessResponse{Message: "Profile deleted successfully"}
		writeResponse(c.Writer, response, http.StatusOK)
	}
}

func deleteProfileConfig(dbInstance *db.Database, dbProfile *db.Profile) {
	slice := isProfileExistInSlice(dbInstance, dbProfile.Name)
	if slice != nil {
		dimsis, err := dbProfile.GetImsis()
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

func isProfileExistInSlice(dbInstance *db.Database, profileName string) *db.NetworkSlice {
	dBSlices, err := dbInstance.ListNetworkSlices()
	if err != nil {
		logger.NmsLog.Warnln(err)
		return nil
	}
	for name, slice := range dBSlices {
		profiles := slice.ListProfiles()
		for _, dgName := range profiles {
			if dgName == profileName {
				logger.NmsLog.Infof("Profile [%v] is part of slice: %v", dgName, name)
				return &slice
			}
		}
	}

	return nil
}
