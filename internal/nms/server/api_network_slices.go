package server

import (
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	openAPIModels "github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	dbModels "github.com/yeastengine/ella/internal/db/models"
	"github.com/yeastengine/ella/internal/db/queries"
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

var imsiData map[string]*openAPIModels.AuthenticationSubscription

func init() {
	imsiData = make(map[string]*openAPIModels.AuthenticationSubscription)
}

func SnssaiModelsToHex(snssai dbModels.Snssai) string {
	sst := fmt.Sprintf("%02x", snssai.Sst)
	return sst + snssai.Sd
}

func GetNetworkSlices(c *gin.Context) {
	setCorsHeader(c)
	networkSlices, err := queries.ListNetworkSliceNames()
	if err != nil {
		logger.NmsLog.Warnln(err)
		c.JSON(http.StatusInternalServerError, nil)
		return
	}
	c.JSON(http.StatusOK, networkSlices)
}

func convertDBNetworkSliceToNetworkSlice(dbNetworkSlice *dbModels.NetworkSlice) *models.Slice {
	networkSlice := &models.Slice{
		SliceName: dbNetworkSlice.Name,
		SliceId: models.SliceSliceId{
			Sst: dbNetworkSlice.Sst,
			Sd:  dbNetworkSlice.Sd,
		},
		SiteDeviceGroup: dbNetworkSlice.DeviceGroups,
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
	for _, dbRadio := range dbNetworkSlice.GNodeBs {
		radio := models.SliceSiteInfoGNodeBs{
			Name: dbRadio.Name,
			Tac:  dbRadio.Tac,
		}
		networkSlice.SiteInfo.GNodeBs = append(networkSlice.SiteInfo.GNodeBs, radio)
	}
	networkSlice.SiteInfo.Upf["upf-name"] = dbNetworkSlice.Upf.Name
	networkSlice.SiteInfo.Upf["upf-port"] = dbNetworkSlice.Upf.Port
	return networkSlice
}

func convertNetworkSliceToDBNetworkSlice(networkSlice *models.Slice) (*dbModels.NetworkSlice, error) {
	upfName, ok := networkSlice.SiteInfo.Upf["upf-name"].(string)
	if !ok {
		return nil, fmt.Errorf("could not convert upf-name to string")
	}
	upfPort, ok := networkSlice.SiteInfo.Upf["upf-port"].(string)
	if !ok {
		return nil, fmt.Errorf("could not convert upf-port to string")
	}
	dbNetworkSlice := &dbModels.NetworkSlice{
		Name:         networkSlice.SliceName,
		Sst:          networkSlice.SliceId.Sst,
		Sd:           networkSlice.SliceId.Sd,
		DeviceGroups: networkSlice.SiteDeviceGroup,
		Mcc:          networkSlice.SiteInfo.Plmn.Mcc,
		Mnc:          networkSlice.SiteInfo.Plmn.Mnc,
		GNodeBs:      make([]dbModels.Radio, 0),
		Upf: dbModels.UPF{
			Name: upfName,
			Port: upfPort,
		},
	}
	for _, radio := range networkSlice.SiteInfo.GNodeBs {
		dbRadio := dbModels.Radio{
			Name: radio.Name,
			Tac:  radio.Tac,
		}
		dbNetworkSlice.GNodeBs = append(dbNetworkSlice.GNodeBs, dbRadio)
	}
	return dbNetworkSlice, nil
}

func GetNetworkSliceByName(c *gin.Context) {
	setCorsHeader(c)
	logger.NmsLog.Infoln("Get Network Slice by name")
	dbNetworkSlice, err := queries.GetNetworkSliceByName(c.Param("slice-name"))
	if err != nil {
		logger.NmsLog.Warnln(err)
		c.JSON(http.StatusInternalServerError, nil)
		return
	}
	if dbNetworkSlice.Name == "" {
		c.JSON(http.StatusNotFound, nil)
		return
	}
	networkSlice := convertDBNetworkSliceToNetworkSlice(dbNetworkSlice)
	c.JSON(http.StatusOK, networkSlice)
}

func NetworkSliceSliceNameDelete(c *gin.Context) {
	if ret := NetworkSliceDeleteHandler(c); ret {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{})
	}
}

func NetworkSliceSliceNamePost(c *gin.Context) {
	if ret := NetworkSlicePostHandler(c, models.Post_op); ret {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{})
	}
}

func NetworkSliceSliceNamePut(c *gin.Context) {
	if ret := NetworkSlicePostHandler(c, models.Put_op); ret {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{})
	}
}

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

func NetworkSliceDeleteHandler(c *gin.Context) bool {
	sliceName, exists := c.Params.Get("slice-name")
	if !exists {
		logger.NmsLog.Errorf("slice-name is missing")
		return false
	}
	prevdbSlice, err := queries.GetNetworkSliceByName(sliceName)
	if err != nil {
		logger.NmsLog.Warnln(err)
	}
	err = queries.DeleteNetworkSlice(sliceName)
	if err != nil {
		logger.NmsLog.Warnln(err)
	}
	prevSlice := convertDBNetworkSliceToNetworkSlice(prevdbSlice)
	dgnames := getDeleteGroupsList(nil, prevSlice)
	for _, dgname := range dgnames {
		devGroupConfig := queries.GetProfile(dgname)
		if devGroupConfig != nil {
			for _, imsi := range devGroupConfig.Imsis {
				err := queries.DeleteAmPolicy(imsi)
				if err != nil {
					logger.NmsLog.Warnln(err)
				}
				err = queries.DeleteSmPolicy(imsi)
				if err != nil {
					logger.NmsLog.Warnln(err)
				}
				err = queries.DeleteAmData(imsi)
				if err != nil {
					logger.NmsLog.Warnln(err)
				}
				err = queries.DeleteSmData(imsi)
				if err != nil {
					logger.NmsLog.Warnln(err)
				}
				err = queries.DeleteSmfSelection(imsi)
				if err != nil {
					logger.NmsLog.Warnln(err)
				}
			}
		}
	}
	updateSMF()
	logger.NmsLog.Infof("Deleted Network Slice: %v", sliceName)
	return true
}

func NetworkSlicePostHandler(c *gin.Context, msgOp int) bool {
	sliceName, exists := c.Params.Get("slice-name")
	if !exists {
		logger.NmsLog.Errorf("slice-name is missing")
		return false
	}

	var err error
	var request models.Slice
	s := strings.Split(c.GetHeader("Content-Type"), ";")
	switch s[0] {
	case "application/json":
		err = c.ShouldBindJSON(&request)
	}
	if err != nil {
		return false
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
		if bitrate < 0 || bitrate > math.MaxInt32 {
			procReq.ApplicationFilteringRules[index].AppMbrUplink = math.MaxInt32
		} else {
			procReq.ApplicationFilteringRules[index].AppMbrUplink = int32(bitrate)
		}

		bitrate = convertToBps(int64(dl), unit)
		if bitrate < 0 || bitrate > math.MaxInt32 {
			procReq.ApplicationFilteringRules[index].AppMbrDownlink = math.MaxInt32
		} else {
			procReq.ApplicationFilteringRules[index].AppMbrDownlink = int32(bitrate)
		}
	}

	procReq.SliceName = sliceName
	sVal, err := strconv.ParseUint(procReq.SliceId.Sst, 10, 32)
	if err != nil {
		logger.NmsLog.Errorf("Could not parse SST %v", procReq.SliceId.Sst)
	}
	snssai := &dbModels.Snssai{
		Sd:  procReq.SliceId.Sd,
		Sst: int32(sVal),
	}
	for _, dgName := range procReq.SiteDeviceGroup {
		dbDeviceGroup := queries.GetProfile(dgName)
		if dbDeviceGroup != nil {
			for _, imsi := range dbDeviceGroup.Imsis {
				dnn := DNN
				mcc := procReq.SiteInfo.Plmn.Mcc
				mnc := procReq.SiteInfo.Plmn.Mnc
				err := queries.CreateAmPolicyData(imsi)
				if err != nil {
					logger.NmsLog.Warnln(err)
				}
				smPolicyData := &dbModels.SmPolicyData{
					UeId: "imsi-" + imsi,
					SmPolicySnssaiData: map[string]dbModels.SmPolicySnssaiData{
						SnssaiModelsToHex(*snssai): {
							Snssai: snssai,
							SmPolicyDnnData: map[string]dbModels.SmPolicyDnnData{
								dnn: {
									Dnn: dnn,
								},
							},
						},
					},
				}
				err = queries.CreateSmPolicyData(smPolicyData)
				if err != nil {
					logger.NmsLog.Warnln(err)
				}
				amData := &dbModels.AccessAndMobilitySubscriptionData{
					UeId:          "imsi-" + imsi,
					ServingPlmnId: mcc + mnc,
					Nssai: &dbModels.Nssai{
						SingleNssais: []dbModels.Snssai{*snssai},
					},
					SubscribedUeAmbr: &dbModels.AmbrRm{
						Downlink: convertToString(uint64(dbDeviceGroup.DnnMbrDownlink)),
						Uplink:   convertToString(uint64(dbDeviceGroup.DnnMbrUplink)),
					},
				}
				err = queries.CreateAmData(amData)
				if err != nil {
					logger.NmsLog.Warnln(err)
				}
				smData := &dbModels.SessionManagementSubscriptionData{
					UeId:          "imsi-" + imsi,
					ServingPlmnId: mcc + mnc,
					SingleNssai:   snssai,
					DnnConfigurations: map[string]dbModels.DnnConfiguration{
						dnn: {
							PduSessionTypes: &dbModels.PduSessionTypes{
								DefaultSessionType:  dbModels.PduSessionType_IPV4,
								AllowedSessionTypes: []dbModels.PduSessionType{dbModels.PduSessionType_IPV4},
							},
							SscModes: &dbModels.SscModes{
								DefaultSscMode: dbModels.SscMode__1,
								AllowedSscModes: []dbModels.SscMode{
									"SSC_MODE_2",
									"SSC_MODE_3",
								},
							},
							SessionAmbr: &dbModels.Ambr{
								Downlink: convertToString(uint64(dbDeviceGroup.DnnMbrDownlink)),
								Uplink:   convertToString(uint64(dbDeviceGroup.DnnMbrUplink)),
							},
							Var5gQosProfile: &dbModels.SubscribedDefaultQos{
								Var5qi: 9,
								Arp: &dbModels.Arp{
									PriorityLevel: 8,
								},
								PriorityLevel: 8,
							},
						},
					},
				}
				err = queries.CreateSmData(smData)
				if err != nil {
					logger.NmsLog.Warnln(err)
				}
				smfSelData := &dbModels.SmfSelectionSubscriptionData{
					UeId:          "imsi-" + imsi,
					ServingPlmnId: mcc + mnc,
					SubscribedSnssaiInfos: map[string]dbModels.SnssaiInfo{
						SnssaiModelsToHex(*snssai): {
							DnnInfos: []dbModels.DnnInfo{
								{
									Dnn: dnn,
								},
							},
						},
					},
				}
				err = queries.CreateSmfSelectionData(smfSelData)
				if err != nil {
					logger.NmsLog.Warnln(err)
				}
			}
		}
	}
	dbNetworkSlice, err := convertNetworkSliceToDBNetworkSlice(&procReq)
	if err != nil {
		logger.NmsLog.Warnf("Could not convert network slice to db network slice: %v", err)
		return false
	}
	err = queries.CreateNetworkSlice(dbNetworkSlice)
	if err != nil {
		logger.NmsLog.Warnln(err)
	}
	updateSMF()
	logger.NmsLog.Infof("Created Network Slice: %v", sliceName)
	return true
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

func updateSMF() {
	networkSlices := make([]*models.Slice, 0)
	networkSliceNames, err := queries.ListNetworkSliceNames()
	if err != nil {
		logger.NmsLog.Warnln(err)
	}
	for _, networkSliceName := range networkSliceNames {
		dbNetworkSlice, err := queries.GetNetworkSliceByName(networkSliceName)
		if err != nil {
			logger.NmsLog.Warnln(err)
			continue
		}
		networkSlice := convertDBNetworkSliceToNetworkSlice(dbNetworkSlice)
		networkSlices = append(networkSlices, networkSlice)
	}
	deviceGroups := make([]models.DeviceGroups, 0)
	deviceGroupNames, err := queries.ListProfiles()
	if err != nil {
		logger.NmsLog.Warnln(err)
	}
	for _, deviceGroupName := range deviceGroupNames {
		dbDeviceGroup := queries.GetProfile(deviceGroupName)
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
