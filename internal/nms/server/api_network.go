package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yeastengine/ella/internal/config"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/models"
	"github.com/yeastengine/ella/internal/smf/context"
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

type UPF struct {
	Name string `json:"name,omitempty"`
	Port int    `json:"port,omitempty"`
}

type UpdateNetworkParams struct {
	Mcc     string   `json:"mcc,omitempty"`
	Mnc     string   `json:"mnc,omitempty"`
	GNodeBs []GNodeB `json:"gNodeBs"`
	Upf     UPF      `json:"upf,omitempty"`
}

type GetNetworkResponse struct {
	Mcc     string   `json:"mcc,omitempty"`
	Mnc     string   `json:"mnc,omitempty"`
	GNodeBs []GNodeB `json:"gNodeBs"`
	Upf     UPF      `json:"upf,omitempty"`
}

func GetNetwork(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		dbNetwork, err := dbInstance.GetNetwork()
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Network not found")
			return
		}
		dbGnodeBs, err := dbNetwork.GetGNodeBs()
		if err != nil {
			logger.NmsLog.Warnln(err)
		}
		gNodeBs := make([]GNodeB, 0)
		for _, dbRadio := range dbGnodeBs {
			radio := GNodeB{
				Name: dbRadio.Name,
				Tac:  dbRadio.Tac,
			}
			gNodeBs = append(gNodeBs, radio)
		}
		dbUpf, err := dbNetwork.GetUpf()
		if err != nil {
			logger.NmsLog.Warnln(err)
		}
		upf := UPF{}
		if dbUpf != nil {
			upf.Name = dbUpf.Name
			upf.Port = dbUpf.Port
		}
		network := &GetNetworkResponse{
			Mcc:     dbNetwork.Mcc,
			Mnc:     dbNetwork.Mnc,
			GNodeBs: gNodeBs,
			Upf:     upf,
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
		if updateNetworkParams.Upf.Name == "" {
			writeError(c.Writer, http.StatusBadRequest, "upf name is missing")
			return
		}
		if updateNetworkParams.Upf.Port == 0 {
			writeError(c.Writer, http.StatusBadRequest, "upf port is missing")
			return
		}

		dbNetwork := &db.Network{
			Mcc: updateNetworkParams.Mcc,
			Mnc: updateNetworkParams.Mnc,
		}
		err = dbNetwork.SetUpf(db.UPF{
			Name: updateNetworkParams.Upf.Name,
			Port: updateNetworkParams.Upf.Port,
		})
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to create network")
			return
		}

		dbGnodeBs := make([]db.GNodeB, 0)
		for _, radio := range updateNetworkParams.GNodeBs {
			dbRadio := db.GNodeB{
				Name: radio.Name,
				Tac:  radio.Tac,
			}
			dbGnodeBs = append(dbGnodeBs, dbRadio)
		}
		err = dbNetwork.SetGNodeBs(dbGnodeBs)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to create network")
			return
		}
		err = dbInstance.UpdateNetwork(dbNetwork)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to create network")
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
	network := &models.NetworkSlice{
		Mcc:     dbNetwork.Mcc,
		Mnc:     dbNetwork.Mnc,
		GNodeBs: make([]models.GNodeB, 0),
	}
	dbGnodeBs, err := dbNetwork.GetGNodeBs()
	if err != nil {
		logger.NmsLog.Warnln(err)
		return
	}
	for _, dbRadio := range dbGnodeBs {
		radio := models.GNodeB{
			Name: dbRadio.Name,
			Tac:  dbRadio.Tac,
		}
		network.GNodeBs = append(network.GNodeBs, radio)
	}
	dbUpf, err := dbNetwork.GetUpf()
	if err != nil {
		logger.NmsLog.Warnln(err)
		return
	}
	if dbUpf != nil {
		network.Upf.Name = dbUpf.Name
		network.Upf.Port = dbUpf.Port
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
			Dnn:             config.DNN,
			UeIpPool:        dbProfile.UeIpPool,
			DnsPrimary:      dbProfile.DnsPrimary,
			DnsSecondary:    dbProfile.DnsSecondary,
			BitrateDownlink: dbProfile.BitrateDownlink,
			BitrateUplink:   dbProfile.BitrateUplink,
			Var5qi:          dbProfile.Var5qi,
			PriorityLevel:   dbProfile.PriorityLevel,
		}
		profiles = append(profiles, profile)
	}
	context.UpdateSMFContext(network, profiles)
}
