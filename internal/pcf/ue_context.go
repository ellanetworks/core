// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pcf

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/omec-project/openapi/models"
)

type UeContext struct {
	SmPolicyData                map[string]*UeSmPolicyData // use smPolicyId(ue.Supi-pduSessionId) as key
	AfRoutReq                   *models.AfRoutingRequirement
	AspId                       string
	PolicyDataSubscriptionStore *models.PolicyDataSubscription
	PolicyDataChangeStore       *models.PolicyDataChangeNotification
	Supi                        string
	Gpsi                        string
	Pei                         string
	AMPolicyData                map[string]*UeAMPolicyData // use PolAssoId(ue.Supi-numPolId) as key
	GroupIds                    []string
	PolAssociationIDGenerator   uint32
}

type UeAMPolicyData struct {
	PolAssoId         string
	AccessType        models.AccessType
	NotificationUri   string
	ServingPlmn       *models.NetworkId
	AltNotifIpv4Addrs []string
	AltNotifIpv6Addrs []string
	AmfStatusUri      string
	Guami             *models.Guami
	ServiveName       string
	// TraceReq *TraceData
	// about AF request
	Pras map[string]models.PresenceInfo
	// related to UDR Subscription Data
	AmPolicyData *models.AmPolicyData // Svbscription Data
	// Corresponding UE
	PcfUe *UeContext
	// Policy Association
	ServAreaRes *models.ServiceAreaRestriction
	UserLoc     *models.UserLocation
	TimeZone    string
	SuppFeat    string
	Triggers    []models.RequestTrigger
	Rfsp        int32
}

type UeSmPolicyData struct {
	PackFiltMapToPccRuleId map[string]string // use PackFiltId as Key
	RemainGbrUL            *float64
	RemainGbrDL            *float64
	SmPolicyData           *models.SmPolicyData // Svbscription Data
	PolicyContext          *models.SmPolicyContextData
	PolicyDecision         *models.SmPolicyDecision
	AppSessions            map[string]bool // related appSessionId
	PcfUe                  *UeContext
	PackFiltIdGenarator    int32
	PccRuleIdGenarator     int32
	ChargingIdGenarator    int32
}

func (ue *UeContext) NewUeAMPolicyData(assolId string, req models.PolicyAssociationRequest) *UeAMPolicyData {
	ue.Gpsi = req.Gpsi
	ue.Pei = req.Pei
	ue.GroupIds = req.GroupIds
	ue.AMPolicyData[assolId] = &UeAMPolicyData{
		PolAssoId:         assolId,
		ServAreaRes:       req.ServAreaRes,
		AltNotifIpv4Addrs: req.AltNotifIpv4Addrs,
		AltNotifIpv6Addrs: req.AltNotifIpv6Addrs,
		AccessType:        req.AccessType,
		NotificationUri:   req.NotificationUri,
		ServingPlmn:       req.ServingPlmn,
		TimeZone:          req.TimeZone,
		Rfsp:              req.Rfsp,
		Guami:             req.Guami,
		UserLoc:           req.UserLoc,
		ServiveName:       req.ServiveName,
		PcfUe:             ue,
	}
	ue.AMPolicyData[assolId].Pras = make(map[string]models.PresenceInfo)
	return ue.AMPolicyData[assolId]
}

// returns UeSmPolicyData and insert related info to Ue with smPolId
func (ue *UeContext) NewUeSmPolicyData(
	key string, request models.SmPolicyContextData, smData *models.SmPolicyData,
) *UeSmPolicyData {
	if smData == nil {
		return nil
	}
	data := UeSmPolicyData{}
	data.PolicyContext = &request
	data.SmPolicyData = smData
	data.PackFiltIdGenarator = 1
	data.PackFiltMapToPccRuleId = make(map[string]string)
	data.AppSessions = make(map[string]bool)
	data.PccRuleIdGenarator = 1
	data.ChargingIdGenarator = 1
	data.PcfUe = ue
	ue.SmPolicyData[key] = &data
	return &data
}

// returns AM Policy which AccessType and plmnId match
func (ue *UeContext) FindAMPolicy(anType models.AccessType, plmnId *models.NetworkId) *UeAMPolicyData {
	if ue == nil || plmnId == nil {
		return nil
	}
	for _, amPolicy := range ue.AMPolicyData {
		if amPolicy.AccessType == anType && reflect.DeepEqual(*amPolicy.ServingPlmn, *plmnId) {
			return amPolicy
		}
	}
	return nil
}

// Convert bitRate string to float64 with uint Kbps
func ConvertBitRateToKbps(bitRate string) (kBitRate float64, err error) {
	list := strings.Split(bitRate, " ")
	if len(list) != 2 {
		err = fmt.Errorf("bitRate format error")
		return 0, err
	}
	// parse exponential value with 2 as base
	exp := 0.0
	switch list[1] {
	case "Tbps":
		exp = 30.0
	case "Gbps":
		exp = 20.0
	case "Mbps":
		exp = 10.0
	case "Kbps":
		exp = 0.0
	case "bps":
		exp = -10.0
	default:
		err = fmt.Errorf("bitRate format error")
		return 0, err
	}
	// parse value from string to float64
	kBitRate, err = strconv.ParseFloat(list[0], 64)
	if err == nil {
		kBitRate = kBitRate * math.Pow(2, exp)
	} else {
		kBitRate = 0.0
	}
	return kBitRate, err
}
