package server

import (
	"net/http"
	"slices"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/models"
	"github.com/yeastengine/ella/internal/smf/context"
)

const DNN = "internet"

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

type CreateNetworkSliceParams struct {
	Name     string   `json:"name,omitempty"`
	Sst      string   `json:"sst,omitempty"`
	Sd       string   `json:"sd,omitempty"`
	Profiles []string `json:"profiles"`
	Mcc      string   `json:"mcc,omitempty"`
	Mnc      string   `json:"mnc,omitempty"`
	GNodeBs  []GNodeB `json:"gNodeBs"`
	Upf      UPF      `json:"upf,omitempty"`
}

type GetNetworkSliceResponse struct {
	Name     string   `json:"name,omitempty"`
	Sst      string   `json:"sst,omitempty"`
	Sd       string   `json:"sd,omitempty"`
	Profiles []string `json:"profiles"`
	Mcc      string   `json:"mcc,omitempty"`
	Mnc      string   `json:"mnc,omitempty"`
	GNodeBs  []GNodeB `json:"gNodeBs"`
	Upf      UPF      `json:"upf,omitempty"`
}

func ListNetworkSlices(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		dbNetworkSlices, err := dbInstance.ListNetworkSlices()
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Network slice not founds")
			return
		}
		networkSliceList := make([]GetNetworkSliceResponse, 0)
		for _, dbNetworkSlice := range dbNetworkSlices {
			dbGnodeBs, err := dbNetworkSlice.GetGNodeBs()
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
			dbUpf, err := dbNetworkSlice.GetUpf()
			if err != nil {
				logger.NmsLog.Warnln(err)
			}
			networkSlice := GetNetworkSliceResponse{
				Name:     dbNetworkSlice.Name,
				Sst:      dbNetworkSlice.Sst,
				Sd:       dbNetworkSlice.Sd,
				Profiles: dbNetworkSlice.ListProfiles(),
				Mcc:      dbNetworkSlice.Mcc,
				Mnc:      dbNetworkSlice.Mnc,
				GNodeBs:  gNodeBs,
				Upf: UPF{
					Name: dbUpf.Name,
					Port: dbUpf.Port,
				},
			}
			networkSliceList = append(networkSliceList, networkSlice)
		}
		err = writeResponse(c.Writer, networkSliceList, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func GetNetworkSlice(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		name := c.Param("name")
		if name == "" {
			writeError(c.Writer, http.StatusBadRequest, "Missing name parameter")
			return
		}
		dbNetworkSlice, err := dbInstance.GetNetworkSlice(name)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Network slice not found")
			return
		}
		dbGnodeBs, err := dbNetworkSlice.GetGNodeBs()
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
		dbUpf, err := dbNetworkSlice.GetUpf()
		if err != nil {
			logger.NmsLog.Warnln(err)
		}
		networkSlice := &GetNetworkSliceResponse{
			Name:     dbNetworkSlice.Name,
			Sst:      dbNetworkSlice.Sst,
			Sd:       dbNetworkSlice.Sd,
			Profiles: dbNetworkSlice.ListProfiles(),
			Mcc:      dbNetworkSlice.Mcc,
			Mnc:      dbNetworkSlice.Mnc,
			GNodeBs:  gNodeBs,
			Upf: UPF{
				Name: dbUpf.Name,
				Port: dbUpf.Port,
			},
		}

		err = writeResponse(c.Writer, networkSlice, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func CreateNetworkSlice(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		var createNetworkSliceParams CreateNetworkSliceParams
		err := c.ShouldBindJSON(&createNetworkSliceParams)
		if err != nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid request data")
			return
		}
		if createNetworkSliceParams.Name == "" {
			writeError(c.Writer, http.StatusBadRequest, "name is missing")
			return
		}
		if createNetworkSliceParams.Sst == "" {
			writeError(c.Writer, http.StatusBadRequest, "sst is missing")
			return
		}
		if createNetworkSliceParams.Sd == "" {
			writeError(c.Writer, http.StatusBadRequest, "sd is missing")
			return
		}
		if createNetworkSliceParams.Mcc == "" {
			writeError(c.Writer, http.StatusBadRequest, "mcc is missing")
			return
		}
		if createNetworkSliceParams.Mnc == "" {
			writeError(c.Writer, http.StatusBadRequest, "mnc is missing")
			return
		}
		if createNetworkSliceParams.Upf.Name == "" {
			writeError(c.Writer, http.StatusBadRequest, "upf name is missing")
			return
		}
		if createNetworkSliceParams.Upf.Port == 0 {
			writeError(c.Writer, http.StatusBadRequest, "upf port is missing")
			return
		}

		_, err = dbInstance.GetNetworkSlice(createNetworkSliceParams.Name)
		if err == nil {
			writeError(c.Writer, http.StatusBadRequest, "Network slice already exists")
			return
		}

		profiles := createNetworkSliceParams.Profiles
		slices.Sort(profiles)

		sVal, err := strconv.ParseUint(createNetworkSliceParams.Sst, 10, 32)
		if err != nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid SST")
			return
		}
		for _, dgName := range createNetworkSliceParams.Profiles {
			dbProfile, err := dbInstance.GetProfile(dgName)
			if err != nil {
				logger.NmsLog.Warnf("Could not get profile %v", dgName)
				continue
			}
			imsis, err := dbProfile.GetImsis()
			if err != nil {
				logger.NmsLog.Warnf("Could not get imsis %v", dbProfile.Imsis)
				continue
			}
			for _, imsi := range imsis {
				mcc := createNetworkSliceParams.Mcc
				mnc := createNetworkSliceParams.Mnc
				ueId := "imsi-" + imsi
				sst := int32(sVal)
				sd := createNetworkSliceParams.Sd
				plmnID := mcc + mnc
				bitRateUplink := convertToString(uint64(dbProfile.BitrateUplink))
				bitRateDownlink := convertToString(uint64(dbProfile.BitrateDownlink))
				var5qi := 9
				priorityLevel := 8
				err = dbInstance.UpdateSubscriberProfile(ueId, DNN, sd, sst, plmnID, bitRateUplink, bitRateDownlink, var5qi, priorityLevel)
				if err != nil {
					logger.NmsLog.Warnf("Could not update subscriber %v", ueId)
					continue
				}
			}
		}

		dbNetworkSlice := &db.NetworkSlice{
			Name: createNetworkSliceParams.Name,
			Sst:  createNetworkSliceParams.Sst,
			Sd:   createNetworkSliceParams.Sd,
			Mcc:  createNetworkSliceParams.Mcc,
			Mnc:  createNetworkSliceParams.Mnc,
		}
		dbNetworkSlice.SetUpf(db.UPF{
			Name: createNetworkSliceParams.Upf.Name,
			Port: createNetworkSliceParams.Upf.Port,
		})

		dbNetworkSlice.SetProfiles(createNetworkSliceParams.Profiles)
		dbGnodeBs := make([]db.GNodeB, 0)
		for _, radio := range createNetworkSliceParams.GNodeBs {
			dbRadio := db.GNodeB{
				Name: radio.Name,
				Tac:  radio.Tac,
			}
			dbGnodeBs = append(dbGnodeBs, dbRadio)
		}
		dbNetworkSlice.SetGNodeBs(dbGnodeBs)

		err = dbInstance.CreateNetworkSlice(dbNetworkSlice)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to create network slice")
			return
		}
		updateSMF(dbInstance)
		logger.NmsLog.Infof("Network slice %s created successfully", createNetworkSliceParams.Name)
		message := SuccessResponse{Message: "Network slice created successfully"}
		err = writeResponse(c.Writer, message, http.StatusCreated)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func DeleteNetworkSlice(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		sliceName, exists := c.Params.Get("name")
		if !exists {
			writeError(c.Writer, http.StatusBadRequest, "name is missing")
			return
		}
		dbNetworkSlice, err := dbInstance.GetNetworkSlice(sliceName)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Network slice not found")
			return
		}
		err = dbInstance.DeleteNetworkSlice(sliceName)
		if err != nil {
			logger.NmsLog.Warnln(err)
		}
		dgnames := getDeleteGroupsList(nil, dbNetworkSlice)
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
		response := SuccessResponse{Message: "Network slice deleted successfully"}
		err = writeResponse(c.Writer, response, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func getDeleteGroupsList(slice *db.NetworkSlice, prevSlice *db.NetworkSlice) (names []string) {
	for prevSlice == nil {
		return
	}
	prevSliceProfiles := prevSlice.ListProfiles()
	if slice != nil {
		for _, pdgName := range prevSliceProfiles {
			var found bool
			sliceProfiles := slice.ListProfiles()
			for _, dgName := range sliceProfiles {
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
		names = append(names, prevSliceProfiles...)
	}
	return
}

func updateSMF(dbInstance *db.Database) {
	networkSlices := make([]*models.NetworkSlice, 0)
	dbNetworkSlices, err := dbInstance.ListNetworkSlices()
	if err != nil {
		logger.NmsLog.Warnln(err)
	}
	for _, dbNetworkSlice := range dbNetworkSlices {
		networkSlice := &models.NetworkSlice{
			Name:     dbNetworkSlice.Name,
			Sst:      dbNetworkSlice.Sst,
			Sd:       dbNetworkSlice.Sd,
			Profiles: dbNetworkSlice.ListProfiles(),
			Mcc:      dbNetworkSlice.Mcc,
			Mnc:      dbNetworkSlice.Mnc,
			GNodeBs:  make([]models.GNodeB, 0),
		}
		dbGnodeBs, err := dbNetworkSlice.GetGNodeBs()
		if err != nil {
			logger.NmsLog.Warnln(err)
		}
		for _, dbRadio := range dbGnodeBs {
			radio := models.GNodeB{
				Name: dbRadio.Name,
				Tac:  dbRadio.Tac,
			}
			networkSlice.GNodeBs = append(networkSlice.GNodeBs, radio)
		}
		dbUpf, err := dbNetworkSlice.GetUpf()
		if err != nil {
			logger.NmsLog.Warnln(err)
		}
		networkSlice.Upf.Name = dbUpf.Name
		networkSlice.Upf.Port = dbUpf.Port
		networkSlices = append(networkSlices, networkSlice)
	}
	profiles := make([]models.Profile, 0)
	dbProfiles, err := dbInstance.ListProfiles()
	if err != nil {
		logger.NmsLog.Warnln(err)
	}
	for _, dbProfile := range dbProfiles {
		profile := models.Profile{
			Name:            dbProfile.Name,
			Dnn:             DNN,
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
			logger.NmsLog.Warnln(err)
		}
		profile.Imsis = imsis
		profiles = append(profiles, profile)
	}
	context.UpdateSMFContext(networkSlices, profiles)
}
