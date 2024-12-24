package server

import (
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/gin-gonic/gin"
)

const (
	KPS = 1000
	MPS = 1000000
	GPS = 1000000000
)

type GNodeB struct {
	Name string `json:"name,omitempty"`
	Tac  int32  `json:"tac,omitempty"`
}

type UpdateNetworkParams struct {
	Mcc string `json:"mcc,omitempty"`
	Mnc string `json:"mnc,omitempty"`
}

type GetNetworkResponse struct {
	Mcc string `json:"mcc,omitempty"`
	Mnc string `json:"mnc,omitempty"`
}

// Mcc is a 3-decimal digit
func isValidMcc(mcc string) bool {
	if len(mcc) != 3 {
		return false
	}
	for _, c := range mcc {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Mnc is a 2 or 3-decimal digit
func isValidMnc(mnc string) bool {
	if len(mnc) != 2 && len(mnc) != 3 {
		return false
	}
	for _, c := range mnc {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func GetNetwork(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		dbNetwork, err := dbInstance.GetNetwork()
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Network not found")
			return
		}

		network := &GetNetworkResponse{
			Mcc: dbNetwork.Mcc,
			Mnc: dbNetwork.Mnc,
		}

		err = writeResponse(c.Writer, network, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func UpdateNetwork(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		var updateNetworkParams UpdateNetworkParams
		err := c.ShouldBindJSON(&updateNetworkParams)
		if err != nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid request data")
			return
		}
		if updateNetworkParams.Mcc == "" {
			writeError(c.Writer, http.StatusBadRequest, "mcc is missing")
			return
		}
		if updateNetworkParams.Mnc == "" {
			writeError(c.Writer, http.StatusBadRequest, "mnc is missing")
			return
		}
		if !isValidMcc(updateNetworkParams.Mcc) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid mcc format. Must be a 3-decimal digit.")
			return
		}
		if !isValidMnc(updateNetworkParams.Mnc) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid mnc format. Must be a 2 or 3-decimal digit.")
			return
		}

		dbNetwork := &db.Network{
			Mcc: updateNetworkParams.Mcc,
			Mnc: updateNetworkParams.Mnc,
		}

		err = dbInstance.UpdateNetwork(dbNetwork)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to update network")
			return
		}
		updateSMF(dbInstance)
		logger.NmsLog.Infof("Network updated successfully")
		message := SuccessResponse{Message: "Network updated successfully"}
		err = writeResponse(c.Writer, message, http.StatusCreated)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func updateSMF(dbInstance *db.Database) {
	dbNetwork, err := dbInstance.GetNetwork()
	if err != nil {
		logger.NmsLog.Warnln(err)
		return
	}
	network := &models.Network{
		Mcc: dbNetwork.Mcc,
		Mnc: dbNetwork.Mnc,
	}

	profiles := make([]models.Profile, 0)
	dbProfiles, err := dbInstance.ListProfiles()
	if err != nil {
		logger.NmsLog.Warnln(err)
		return
	}
	for _, dbProfile := range dbProfiles {
		profile := models.Profile{
			Name:            dbProfile.Name,
			UeIpPool:        dbProfile.UeIpPool,
			Dns:             dbProfile.Dns,
			BitrateDownlink: dbProfile.BitrateDownlink,
			BitrateUplink:   dbProfile.BitrateUplink,
			Var5qi:          dbProfile.Var5qi,
			PriorityLevel:   dbProfile.PriorityLevel,
		}
		profiles = append(profiles, profile)
	}
	dbRadios, err := dbInstance.ListRadios()
	if err != nil {
		logger.NmsLog.Warnln(err)
		return
	}
	radios := make([]models.Radio, 0)
	for _, dbRadio := range dbRadios {
		radio := models.Radio{
			Name: dbRadio.Name,
			Tac:  dbRadio.Tac,
		}
		radios = append(radios, radio)
	}
	context.UpdateSMFContext(network, profiles, radios)
}
