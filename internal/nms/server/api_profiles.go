package server

import (
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

type CreateProfileParams struct {
	Name string `json:"name"`

	UeIpPool        string `json:"ue-ip-pool,omitempty"`
	Dns             string `json:"dns,omitempty"`
	Mtu             int32  `json:"mtu,omitempty"`
	BitrateUplink   string `json:"bitrate-uplink,omitempty"`
	BitrateDownlink string `json:"bitrate-downlink,omitempty"`
	Var5qi          int32  `json:"var5qi,omitempty"`
	PriorityLevel   int32  `json:"priority-level,omitempty"`
}

type GetProfileResponse struct {
	Name string `json:"name"`

	UeIpPool        string `json:"ue-ip-pool,omitempty"`
	Dns             string `json:"dns,omitempty"`
	Mtu             int32  `json:"mtu,omitempty"`
	BitrateUplink   string `json:"bitrate-uplink,omitempty"`
	BitrateDownlink string `json:"bitrate-downlink,omitempty"`
	Var5qi          int32  `json:"var5qi,omitempty"`
	PriorityLevel   int32  `json:"priority-level,omitempty"`
}

func isProfileNameValid(name string) bool {
	return len(name) > 0 && len(name) < 256
}

func isUeIpPoolValid(ueIpPool string) bool {
	_, _, err := net.ParseCIDR(ueIpPool)
	return err == nil
}

func isValidDNS(dns string) bool {
	return net.ParseIP(dns) != nil
}

func isValidMTU(mtu int32) bool {
	return mtu >= 0 && mtu <= 65535
}

func isValidBitrate(bitrate string) bool {
	s := strings.Split(bitrate, " ")
	if len(s) != 2 {
		return false
	}
	value := s[0]
	unit := s[1]
	if unit != "Mbps" && unit != "Gbps" {
		return false
	}

	valueInt, err := strconv.Atoi(value)
	if err != nil {
		return false
	}
	return valueInt > 0 && valueInt <= 1000
}

func isValid5Qi(var5qi int32) bool {
	return var5qi >= 1 && var5qi <= 255
}

func isValidPriorityLevel(priorityLevel int32) bool {
	return priorityLevel >= 1 && priorityLevel <= 255
}

func ListProfiles(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		dbProfiles, err := dbInstance.ListProfiles()
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Profiles not found")
			return
		}
		profileList := make([]GetProfileResponse, 0)
		for _, dbProfile := range dbProfiles {
			profileResponse := GetProfileResponse{
				Name:            dbProfile.Name,
				UeIpPool:        dbProfile.UeIpPool,
				Dns:             dbProfile.Dns,
				BitrateDownlink: dbProfile.BitrateDownlink,
				BitrateUplink:   dbProfile.BitrateUplink,
				Var5qi:          dbProfile.Var5qi,
				PriorityLevel:   dbProfile.PriorityLevel,
			}
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
			Dns:             dbProfile.Dns,
			Mtu:             dbProfile.Mtu,
			BitrateDownlink: dbProfile.BitrateDownlink,
			BitrateUplink:   dbProfile.BitrateUplink,
			Var5qi:          dbProfile.Var5qi,
			PriorityLevel:   dbProfile.PriorityLevel,
		}
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
		if createProfileParams.Dns == "" {
			writeError(c.Writer, http.StatusBadRequest, "dns is missing")
			return
		}
		if createProfileParams.Mtu == 0 {
			writeError(c.Writer, http.StatusBadRequest, "mtu is missing")
			return
		}
		if createProfileParams.BitrateUplink == "" {
			writeError(c.Writer, http.StatusBadRequest, "bitrate-uplink is missing")
			return
		}
		if createProfileParams.BitrateDownlink == "" {
			writeError(c.Writer, http.StatusBadRequest, "bitrate-downlink is missing")
			return
		}
		if createProfileParams.Var5qi == 0 {
			writeError(c.Writer, http.StatusBadRequest, "Var5qi is missing")
			return
		}
		if createProfileParams.PriorityLevel == 0 {
			writeError(c.Writer, http.StatusBadRequest, "priority-level is missing")
			return
		}
		if !isProfileNameValid(createProfileParams.Name) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid name format. Must be less than 256 characters")
			return
		}
		if !isUeIpPoolValid(createProfileParams.UeIpPool) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid ue-ip-pool format. Must be in CIDR format")
			return
		}
		if !isValidDNS(createProfileParams.Dns) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid dns format. Must be a valid IP address")
			return
		}
		if !isValidMTU(createProfileParams.Mtu) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid mtu format. Must be an integer between 0 and 65535")
			return
		}
		if !isValidBitrate(createProfileParams.BitrateUplink) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid bitrate-uplink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps")
			return
		}
		if !isValidBitrate(createProfileParams.BitrateDownlink) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid bitrate-downlink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps")
			return
		}
		if !isValid5Qi(createProfileParams.Var5qi) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid Var5qi format. Must be an integer between 1 and 255")
			return
		}
		if !isValidPriorityLevel(createProfileParams.PriorityLevel) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid priority-level format. Must be an integer between 1 and 255")
			return
		}

		_, err = dbInstance.GetProfile(createProfileParams.Name)
		if err == nil {
			writeError(c.Writer, http.StatusBadRequest, "Profile already exists")
			return
		}

		dbProfile := &db.Profile{
			Name:            createProfileParams.Name,
			UeIpPool:        createProfileParams.UeIpPool,
			Dns:             createProfileParams.Dns,
			Mtu:             createProfileParams.Mtu,
			BitrateDownlink: createProfileParams.BitrateDownlink,
			BitrateUplink:   createProfileParams.BitrateUplink,
			Var5qi:          createProfileParams.Var5qi,
			PriorityLevel:   createProfileParams.PriorityLevel,
		}
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

func UpdateProfile(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		groupName, exists := c.Params.Get("name")
		if !exists {
			writeError(c.Writer, http.StatusBadRequest, "Missing name parameter")
			return
		}
		var updateProfileParams CreateProfileParams
		err := c.ShouldBindJSON(&updateProfileParams)
		if err != nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid request data")
			return
		}
		if updateProfileParams.Name == "" {
			writeError(c.Writer, http.StatusBadRequest, "name is missing")
			return
		}
		if updateProfileParams.UeIpPool == "" {
			writeError(c.Writer, http.StatusBadRequest, "ue-ip-pool is missing")
			return
		}
		if updateProfileParams.Dns == "" {
			writeError(c.Writer, http.StatusBadRequest, "dns is missing")
			return
		}
		if updateProfileParams.Mtu == 0 {
			writeError(c.Writer, http.StatusBadRequest, "mtu is missing")
			return
		}
		if updateProfileParams.BitrateUplink == "" {
			writeError(c.Writer, http.StatusBadRequest, "bitrate-uplink is missing")
			return
		}
		if updateProfileParams.BitrateDownlink == "" {
			writeError(c.Writer, http.StatusBadRequest, "bitrate-downlink is missing")
			return
		}
		if updateProfileParams.Var5qi == 0 {
			writeError(c.Writer, http.StatusBadRequest, "Var5qi is missing")
			return
		}
		if updateProfileParams.PriorityLevel == 0 {
			writeError(c.Writer, http.StatusBadRequest, "priority-level is missing")
			return
		}
		if !isProfileNameValid(updateProfileParams.Name) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid name format. Must be less than 256 characters")
			return
		}
		if !isUeIpPoolValid(updateProfileParams.UeIpPool) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid ue-ip-pool format. Must be in CIDR format")
			return
		}
		if !isValidDNS(updateProfileParams.Dns) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid dns format. Must be a valid IP address")
			return
		}
		if !isValidMTU(updateProfileParams.Mtu) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid mtu format. Must be an integer between 0 and 65535")
			return
		}
		if !isValidBitrate(updateProfileParams.BitrateUplink) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid bitrate-uplink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps")
			return
		}
		if !isValidBitrate(updateProfileParams.BitrateDownlink) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid bitrate-downlink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps")
			return
		}
		if !isValid5Qi(updateProfileParams.Var5qi) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid Var5qi format. Must be an integer between 1 and 255")
			return
		}
		if !isValidPriorityLevel(updateProfileParams.PriorityLevel) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid priority-level format. Must be an integer between 1 and 255")
			return
		}

		profile, err := dbInstance.GetProfile(groupName)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Profile not found")
			return
		}

		profile.Name = updateProfileParams.Name
		profile.UeIpPool = updateProfileParams.UeIpPool
		profile.Dns = updateProfileParams.Dns
		profile.Mtu = updateProfileParams.Mtu
		profile.BitrateDownlink = updateProfileParams.BitrateDownlink
		profile.BitrateUplink = updateProfileParams.BitrateUplink
		profile.Var5qi = updateProfileParams.Var5qi
		profile.PriorityLevel = updateProfileParams.PriorityLevel
		err = dbInstance.UpdateProfile(profile)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Failed to update profile")
			return
		}

		updateSMF(dbInstance)
		logger.NmsLog.Infof("Updated Profile: %v", updateProfileParams.Name)
		response := SuccessResponse{Message: "Profile updated successfully"}
		err = writeResponse(c.Writer, response, http.StatusOK)
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
		_, err := dbInstance.GetProfile(groupName)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Profile not found")
			return
		}
		err = dbInstance.DeleteProfile(groupName)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Failed to delete profile")
			return
		}
		updateSMF(dbInstance)
		logger.NmsLog.Infof("Deleted Profile: %v", groupName)
		response := SuccessResponse{Message: "Profile deleted successfully"}
		err = writeResponse(c.Writer, response, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
	}
}
