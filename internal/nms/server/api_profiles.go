package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/logger"
)

type CreateProfileParams struct {
	Name string `json:"name"`

	UeIpPool        string `json:"ue-ip-pool,omitempty"`
	DnsPrimary      string `json:"dns-primary,omitempty"`
	DnsSecondary    string `json:"dns-secondary,omitempty"`
	Mtu             int32  `json:"mtu,omitempty"`
	BitrateUplink   string `json:"bitrate-uplink,omitempty"`
	BitrateDownlink string `json:"bitrate-downlink,omitempty"`
	Var5qi          int32  `json:"var5qi,omitempty"`
	PriorityLevel   int32  `json:"priority-level,omitempty"`
}

type GetProfileResponse struct {
	Name string `json:"name"`

	UeIpPool        string `json:"ue-ip-pool,omitempty"`
	DnsPrimary      string `json:"dns-primary,omitempty"`
	DnsSecondary    string `json:"dns-secondary,omitempty"`
	Mtu             int32  `json:"mtu,omitempty"`
	BitrateUplink   string `json:"bitrate-uplink,omitempty"`
	BitrateDownlink string `json:"bitrate-downlink,omitempty"`
	Var5qi          int32  `json:"var5qi,omitempty"`
	PriorityLevel   int32  `json:"priority-level,omitempty"`
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
				DnsPrimary:      dbProfile.DnsPrimary,
				DnsSecondary:    dbProfile.DnsSecondary,
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
			DnsPrimary:      dbProfile.DnsPrimary,
			DnsSecondary:    dbProfile.DnsSecondary,
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
		if createProfileParams.DnsPrimary == "" {
			writeError(c.Writer, http.StatusBadRequest, "dns-primary is missing")
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

		_, err = dbInstance.GetProfile(createProfileParams.Name)
		if err == nil {
			writeError(c.Writer, http.StatusBadRequest, "Profile already exists")
			return
		}

		dbProfile := &db.Profile{
			Name:            createProfileParams.Name,
			UeIpPool:        createProfileParams.UeIpPool,
			DnsPrimary:      createProfileParams.DnsPrimary,
			DnsSecondary:    createProfileParams.DnsSecondary,
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
		if updateProfileParams.DnsPrimary == "" {
			writeError(c.Writer, http.StatusBadRequest, "dns-primary is missing")
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

		profile, err := dbInstance.GetProfile(groupName)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Profile not found")
			return
		}

		profile.Name = updateProfileParams.Name
		profile.UeIpPool = updateProfileParams.UeIpPool
		profile.DnsPrimary = updateProfileParams.DnsPrimary
		profile.DnsSecondary = updateProfileParams.DnsSecondary
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
