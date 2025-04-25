package server

import (
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type CreateProfileParams struct {
	Name string `json:"name"`

	UeIPPool        string `json:"ue-ip-pool,omitempty"`
	DNS             string `json:"dns,omitempty"`
	Mtu             int32  `json:"mtu,omitempty"`
	BitrateUplink   string `json:"bitrate-uplink,omitempty"`
	BitrateDownlink string `json:"bitrate-downlink,omitempty"`
	Var5qi          int32  `json:"var5qi,omitempty"`
	PriorityLevel   int32  `json:"priority-level,omitempty"`
}

type GetProfileResponse struct {
	Name string `json:"name"`

	UeIPPool        string `json:"ue-ip-pool,omitempty"`
	DNS             string `json:"dns,omitempty"`
	Mtu             int32  `json:"mtu,omitempty"`
	BitrateUplink   string `json:"bitrate-uplink,omitempty"`
	BitrateDownlink string `json:"bitrate-downlink,omitempty"`
	Var5qi          int32  `json:"var5qi,omitempty"`
	PriorityLevel   int32  `json:"priority-level,omitempty"`
}

const (
	ListProfilesAction  = "list_profiles"
	GetProfileAction    = "get_profile"
	CreateProfileAction = "create_profile"
	UpdateProfileAction = "update_profile"
	DeleteProfileAction = "delete_profile"
)

func isProfileNameValid(name string) bool {
	return len(name) > 0 && len(name) < 256
}

func isUeIPPoolValid(ueIPPool string) bool {
	_, _, err := net.ParseCIDR(ueIPPool)
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
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		dbProfiles, err := dbInstance.ListProfiles()
		if err != nil {
			writeError(c, http.StatusInternalServerError, "Profiles not found")
			return
		}
		profileList := make([]GetProfileResponse, 0)
		for _, dbProfile := range dbProfiles {
			profileResponse := GetProfileResponse{
				Name:            dbProfile.Name,
				UeIPPool:        dbProfile.UeIPPool,
				DNS:             dbProfile.DNS,
				BitrateDownlink: dbProfile.BitrateDownlink,
				BitrateUplink:   dbProfile.BitrateUplink,
				Var5qi:          dbProfile.Var5qi,
				PriorityLevel:   dbProfile.PriorityLevel,
			}
			profileList = append(profileList, profileResponse)
		}
		writeResponse(c, profileList, http.StatusOK)
		logger.LogAuditEvent(
			ListProfilesAction,
			email,
			c.ClientIP(),
			"User listed profiles",
		)
	}
}

func GetProfile(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		profileName, exists := c.Params.Get("name")
		if !exists {
			writeError(c, http.StatusBadRequest, "Missing name parameter")
			return
		}
		dbProfile, err := dbInstance.GetProfile(profileName)
		if err != nil {
			writeError(c, http.StatusNotFound, "Profile not found")
			return
		}

		profileResponse := GetProfileResponse{
			Name:            dbProfile.Name,
			UeIPPool:        dbProfile.UeIPPool,
			DNS:             dbProfile.DNS,
			Mtu:             dbProfile.Mtu,
			BitrateDownlink: dbProfile.BitrateDownlink,
			BitrateUplink:   dbProfile.BitrateUplink,
			Var5qi:          dbProfile.Var5qi,
			PriorityLevel:   dbProfile.PriorityLevel,
		}
		writeResponse(c, profileResponse, http.StatusOK)
		logger.LogAuditEvent(
			GetProfileAction,
			email,
			c.ClientIP(),
			"User retrieved profile: "+profileName,
		)
	}
}

func CreateProfile(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		var createProfileParams CreateProfileParams
		err := c.ShouldBindJSON(&createProfileParams)
		if err != nil {
			writeError(c, http.StatusBadRequest, "Invalid request data")
			return
		}
		if createProfileParams.Name == "" {
			writeError(c, http.StatusBadRequest, "name is missing")
			return
		}
		if createProfileParams.UeIPPool == "" {
			writeError(c, http.StatusBadRequest, "ue-ip-pool is missing")
			return
		}
		if createProfileParams.DNS == "" {
			writeError(c, http.StatusBadRequest, "dns is missing")
			return
		}
		if createProfileParams.Mtu == 0 {
			writeError(c, http.StatusBadRequest, "mtu is missing")
			return
		}
		if createProfileParams.BitrateUplink == "" {
			writeError(c, http.StatusBadRequest, "bitrate-uplink is missing")
			return
		}
		if createProfileParams.BitrateDownlink == "" {
			writeError(c, http.StatusBadRequest, "bitrate-downlink is missing")
			return
		}
		if createProfileParams.Var5qi == 0 {
			writeError(c, http.StatusBadRequest, "Var5qi is missing")
			return
		}
		if createProfileParams.PriorityLevel == 0 {
			writeError(c, http.StatusBadRequest, "priority-level is missing")
			return
		}
		if !isProfileNameValid(createProfileParams.Name) {
			writeError(c, http.StatusBadRequest, "Invalid name format. Must be less than 256 characters")
			return
		}
		if !isUeIPPoolValid(createProfileParams.UeIPPool) {
			writeError(c, http.StatusBadRequest, "Invalid ue-ip-pool format. Must be in CIDR format")
			return
		}
		if !isValidDNS(createProfileParams.DNS) {
			writeError(c, http.StatusBadRequest, "Invalid dns format. Must be a valid IP address")
			return
		}
		if !isValidMTU(createProfileParams.Mtu) {
			writeError(c, http.StatusBadRequest, "Invalid mtu format. Must be an integer between 0 and 65535")
			return
		}
		if !isValidBitrate(createProfileParams.BitrateUplink) {
			writeError(c, http.StatusBadRequest, "Invalid bitrate-uplink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps")
			return
		}
		if !isValidBitrate(createProfileParams.BitrateDownlink) {
			writeError(c, http.StatusBadRequest, "Invalid bitrate-downlink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps")
			return
		}
		if !isValid5Qi(createProfileParams.Var5qi) {
			writeError(c, http.StatusBadRequest, "Invalid Var5qi format. Must be an integer between 1 and 255")
			return
		}
		if !isValidPriorityLevel(createProfileParams.PriorityLevel) {
			writeError(c, http.StatusBadRequest, "Invalid priority-level format. Must be an integer between 1 and 255")
			return
		}

		_, err = dbInstance.GetProfile(createProfileParams.Name)
		if err == nil {
			writeError(c, http.StatusBadRequest, "Profile already exists")
			return
		}

		dbProfile := &db.Profile{
			Name:            createProfileParams.Name,
			UeIPPool:        createProfileParams.UeIPPool,
			DNS:             createProfileParams.DNS,
			Mtu:             createProfileParams.Mtu,
			BitrateDownlink: createProfileParams.BitrateDownlink,
			BitrateUplink:   createProfileParams.BitrateUplink,
			Var5qi:          createProfileParams.Var5qi,
			PriorityLevel:   createProfileParams.PriorityLevel,
		}
		err = dbInstance.CreateProfile(dbProfile)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "Failed to create profile")
			return
		}
		response := SuccessResponse{Message: "Profile created successfully"}
		writeResponse(c, response, http.StatusCreated)
		logger.LogAuditEvent(
			CreateProfileAction,
			email,
			c.ClientIP(),
			"User created profile: "+createProfileParams.Name,
		)
	}
}

func UpdateProfile(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		groupName, exists := c.Params.Get("name")
		if !exists {
			writeError(c, http.StatusBadRequest, "Missing name parameter")
			return
		}
		var updateProfileParams CreateProfileParams
		err := c.ShouldBindJSON(&updateProfileParams)
		if err != nil {
			writeError(c, http.StatusBadRequest, "Invalid request data")
			return
		}
		if updateProfileParams.Name == "" {
			writeError(c, http.StatusBadRequest, "name is missing")
			return
		}
		if updateProfileParams.UeIPPool == "" {
			writeError(c, http.StatusBadRequest, "ue-ip-pool is missing")
			return
		}
		if updateProfileParams.DNS == "" {
			writeError(c, http.StatusBadRequest, "dns is missing")
			return
		}
		if updateProfileParams.Mtu == 0 {
			writeError(c, http.StatusBadRequest, "mtu is missing")
			return
		}
		if updateProfileParams.BitrateUplink == "" {
			writeError(c, http.StatusBadRequest, "bitrate-uplink is missing")
			return
		}
		if updateProfileParams.BitrateDownlink == "" {
			writeError(c, http.StatusBadRequest, "bitrate-downlink is missing")
			return
		}
		if updateProfileParams.Var5qi == 0 {
			writeError(c, http.StatusBadRequest, "Var5qi is missing")
			return
		}
		if updateProfileParams.PriorityLevel == 0 {
			writeError(c, http.StatusBadRequest, "priority-level is missing")
			return
		}
		if !isProfileNameValid(updateProfileParams.Name) {
			writeError(c, http.StatusBadRequest, "Invalid name format. Must be less than 256 characters")
			return
		}
		if !isUeIPPoolValid(updateProfileParams.UeIPPool) {
			writeError(c, http.StatusBadRequest, "Invalid ue-ip-pool format. Must be in CIDR format")
			return
		}
		if !isValidDNS(updateProfileParams.DNS) {
			writeError(c, http.StatusBadRequest, "Invalid dns format. Must be a valid IP address")
			return
		}
		if !isValidMTU(updateProfileParams.Mtu) {
			writeError(c, http.StatusBadRequest, "Invalid mtu format. Must be an integer between 0 and 65535")
			return
		}
		if !isValidBitrate(updateProfileParams.BitrateUplink) {
			writeError(c, http.StatusBadRequest, "Invalid bitrate-uplink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps")
			return
		}
		if !isValidBitrate(updateProfileParams.BitrateDownlink) {
			writeError(c, http.StatusBadRequest, "Invalid bitrate-downlink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps")
			return
		}
		if !isValid5Qi(updateProfileParams.Var5qi) {
			writeError(c, http.StatusBadRequest, "Invalid Var5qi format. Must be an integer between 1 and 255")
			return
		}
		if !isValidPriorityLevel(updateProfileParams.PriorityLevel) {
			writeError(c, http.StatusBadRequest, "Invalid priority-level format. Must be an integer between 1 and 255")
			return
		}

		profile, err := dbInstance.GetProfile(groupName)
		if err != nil {
			writeError(c, http.StatusNotFound, "Profile not found")
			return
		}

		profile.Name = updateProfileParams.Name
		profile.UeIPPool = updateProfileParams.UeIPPool
		profile.DNS = updateProfileParams.DNS
		profile.Mtu = updateProfileParams.Mtu
		profile.BitrateDownlink = updateProfileParams.BitrateDownlink
		profile.BitrateUplink = updateProfileParams.BitrateUplink
		profile.Var5qi = updateProfileParams.Var5qi
		profile.PriorityLevel = updateProfileParams.PriorityLevel
		err = dbInstance.UpdateProfile(profile)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "Failed to update profile")
			return
		}

		response := SuccessResponse{Message: "Profile updated successfully"}
		writeResponse(c, response, http.StatusOK)
		logger.LogAuditEvent(
			UpdateProfileAction,
			email,
			c.ClientIP(),
			"User updated profile: "+updateProfileParams.Name,
		)
	}
}

func DeleteProfile(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		profileName, exists := c.Params.Get("name")
		if !exists {
			writeError(c, http.StatusBadRequest, "Missing name parameter")
			return
		}
		_, err := dbInstance.GetProfile(profileName)
		if err != nil {
			writeError(c, http.StatusNotFound, "Profile not found")
			return
		}
		subsInProfile, err := dbInstance.SubscribersInProfile(profileName)
		if err != nil {
			logger.APILog.Warn("Failed to check subscribers in profile", zap.Error(err))
			writeError(c, http.StatusInternalServerError, "Failed to count subscribers")
			return
		}
		if subsInProfile {
			writeError(c, http.StatusConflict, "Profile has subscribers")
			return
		}
		err = dbInstance.DeleteProfile(profileName)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "Failed to delete profile")
			return
		}
		response := SuccessResponse{Message: "Profile deleted successfully"}
		writeResponse(c, response, http.StatusOK)
		logger.LogAuditEvent(
			DeleteProfileAction,
			email,
			c.ClientIP(),
			"User deleted profile: "+profileName,
		)
	}
}
