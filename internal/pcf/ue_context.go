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
	SmPolicyData              map[string]*UeSmPolicyData // use smPolicyID(ue.Supi-pduSessionID) as key
	Supi                      string
	Gpsi                      string
	Pei                       string
	AMPolicyData              map[string]*UeAMPolicyData // use PolAssoID(ue.Supi-numPolId) as key
	GroupIds                  []string
	PolAssociationIDGenerator uint32
}

type UeAMPolicyData struct {
	PolAssoID         string
	AccessType        models.AccessType
	NotificationURI   string
	ServingPlmn       *models.NetworkId
	AltNotifIpv4Addrs []string
	AltNotifIpv6Addrs []string
	Guami             *models.Guami
	ServiveName       string
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
	RemainGbrUL    *float64
	RemainGbrDL    *float64
	SmPolicyData   *models.SmPolicyData // Svbscription Data
	PolicyContext  *models.SmPolicyContextData
	PolicyDecision *models.SmPolicyDecision
	AppSessions    map[string]bool // related appSessionId
	PcfUe          *UeContext
}

func (ue *UeContext) NewUeAMPolicyData(assolID string, req models.PolicyAssociationRequest) *UeAMPolicyData {
	ue.Gpsi = req.Gpsi
	ue.Pei = req.Pei
	ue.GroupIds = req.GroupIds
	ue.AMPolicyData[assolID] = &UeAMPolicyData{
		PolAssoID:         assolID,
		ServAreaRes:       req.ServAreaRes,
		AltNotifIpv4Addrs: req.AltNotifIpv4Addrs,
		AltNotifIpv6Addrs: req.AltNotifIpv6Addrs,
		AccessType:        req.AccessType,
		NotificationURI:   req.NotificationUri,
		ServingPlmn:       req.ServingPlmn,
		TimeZone:          req.TimeZone,
		Rfsp:              req.Rfsp,
		Guami:             req.Guami,
		UserLoc:           req.UserLoc,
		ServiveName:       req.ServiveName,
		PcfUe:             ue,
	}
	ue.AMPolicyData[assolID].Pras = make(map[string]models.PresenceInfo)
	return ue.AMPolicyData[assolID]
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
	data.AppSessions = make(map[string]bool)
	data.PcfUe = ue
	ue.SmPolicyData[key] = &data
	return &data
}

// returns AM Policy which AccessType and plmnID match
func (ue *UeContext) FindAMPolicy(anType models.AccessType, plmnID *models.NetworkId) *UeAMPolicyData {
	if ue == nil || plmnID == nil {
		return nil
	}
	for _, amPolicy := range ue.AMPolicyData {
		if amPolicy.AccessType == anType && reflect.DeepEqual(*amPolicy.ServingPlmn, *plmnID) {
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
