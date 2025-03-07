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

	"github.com/ellanetworks/core/internal/models"
)

type UeContext struct {
	SmPolicyData              map[string]*UeSmPolicyData // use smPolicyId(ue.Supi-pduSessionId) as key
	Supi                      string
	Gpsi                      string
	Pei                       string
	AMPolicyData              map[string]*UeAMPolicyData // use PolAssoId(ue.Supi-numPolId) as key
	PolAssociationIDGenerator uint32
}

type UeAMPolicyData struct {
	AccessType  models.AccessType
	ServingPlmn *models.PlmnID
	Guami       *models.Guami
	Pras        map[string]models.PresenceInfo
	PcfUe       *UeContext
	ServAreaRes *models.ServiceAreaRestriction
	UserLoc     *models.UserLocation
	TimeZone    string
	Triggers    []models.RequestTrigger
	Rfsp        int32
}

type UeSmPolicyData struct {
	SmPolicyData  *models.SmPolicyData // Svbscription Data
	PolicyContext *models.SmPolicyContextData
	AppSessions   map[string]bool // related appSessionId
	PcfUe         *UeContext
}

func (ue *UeContext) NewUeAMPolicyData(assolID string, req models.PolicyAssociationRequest) *UeAMPolicyData {
	ue.Gpsi = req.Gpsi
	ue.Pei = req.Pei
	ue.AMPolicyData[assolID] = &UeAMPolicyData{
		ServAreaRes: req.ServAreaRes,
		AccessType:  req.AccessType,
		ServingPlmn: req.ServingPlmn,
		TimeZone:    req.TimeZone,
		Rfsp:        req.Rfsp,
		Guami:       req.Guami,
		UserLoc:     req.UserLoc,
		PcfUe:       ue,
	}
	ue.AMPolicyData[assolID].Pras = make(map[string]models.PresenceInfo)
	return ue.AMPolicyData[assolID]
}

// returns UeSmPolicyData and insert related info to Ue with smPolId
func (ue *UeContext) NewUeSmPolicyData(key string, request models.SmPolicyContextData, smData *models.SmPolicyData) *UeSmPolicyData {
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
func (ue *UeContext) FindAMPolicy(anType models.AccessType, plmnID *models.PlmnID) *UeAMPolicyData {
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
