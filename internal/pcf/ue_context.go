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
	Supi         string
	AMPolicyData *UeAMPolicyData // use PolAssoId(ue.Supi-numPolId) as key
}

type UeAMPolicyData struct {
	AccessType  models.AccessType
	ServingPlmn *models.PlmnID
	UserLoc     *models.UserLocation
	Triggers    []models.RequestTrigger
	Rfsp        int32
}

func (ue *UeContext) NewUeAMPolicyData(req models.PolicyAssociationRequest) *UeAMPolicyData {
	ue.AMPolicyData = &UeAMPolicyData{
		AccessType:  req.AccessType,
		ServingPlmn: req.ServingPlmn,
		Rfsp:        req.Rfsp,
		UserLoc:     req.UserLoc,
	}
	return ue.AMPolicyData
}

// returns AM Policy which AccessType and plmnID match
func (ue *UeContext) FindAMPolicy(anType models.AccessType, plmnID *models.PlmnID) *UeAMPolicyData {
	if ue == nil || plmnID == nil || ue.AMPolicyData == nil {
		return nil
	}

	if ue.AMPolicyData.AccessType == anType && reflect.DeepEqual(*ue.AMPolicyData.ServingPlmn, *plmnID) {
		return ue.AMPolicyData
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
