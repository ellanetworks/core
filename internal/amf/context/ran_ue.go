// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/ngap/ngapConvert"
	"github.com/omec-project/ngap/ngapType"
	"go.uber.org/zap"
)

type RelAction int

const (
	RanUeNgapIdUnspecified int64 = 0xffffffff
)

const (
	UeContextN2NormalRelease RelAction = iota
	UeContextReleaseHandover
	UeContextReleaseUeContext
	UeContextReleaseDueToNwInitiatedDeregistraion
)

type RanUe struct {
	RanUeNgapId                      int64
	AmfUeNgapId                      int64
	HandOverType                     ngapType.HandoverType
	SuccessPduSessionId              []int32
	SourceUe                         *RanUe
	TargetUe                         *RanUe
	Tai                              models.Tai
	Location                         models.UserLocation
	SupportVoPS                      bool
	SupportedFeatures                string
	LastActTime                      *time.Time
	AmfUe                            *AmfUe
	Ran                              *AmfRan
	RoutingID                        string
	Trsr                             string /* Trace Recording Session Reference */
	ReleaseAction                    RelAction
	OldAmfName                       string
	InitialUEMessage                 []byte
	RRCEstablishmentCause            string // Received from initial ue message; pattern: ^[0-9a-fA-F]+$
	UeContextRequest                 bool
	SentInitialContextSetupRequest   bool
	RecvdInitialContextSetupResponse bool /*Received Initial context setup response or not */
	Log                              *zap.SugaredLogger
}

func (ranUe *RanUe) Remove() error {
	if ranUe == nil {
		return fmt.Errorf("ran ue is nil")
	}
	ran := ranUe.Ran
	if ran == nil {
		return fmt.Errorf("ran not found in ranUe not found")
	}
	if ranUe.AmfUe != nil {
		ranUe.AmfUe.DetachRanUe(ran.AnType)
		ranUe.DetachAmfUe()
	}

	for index, ranUe1 := range ran.RanUeList {
		if ranUe1 == ranUe {
			ran.RanUeList = append(ran.RanUeList[:index], ran.RanUeList[index+1:]...)
			break
		}
	}
	amfUeNGAPIDGenerator.FreeID(ranUe.AmfUeNgapId)
	logger.AmfLog.Infof("ran ue removed: %d", ranUe.RanUeNgapId)
	return nil
}

func (ranUe *RanUe) DetachAmfUe() {
	ranUe.AmfUe = nil
}

func (ranUe *RanUe) SwitchToRan(newRan *AmfRan, ranUeNgapId int64) error {
	if ranUe == nil {
		return fmt.Errorf("ran ue is nil")
	}

	if newRan == nil {
		return fmt.Errorf("new ran is nil")
	}

	oldRan := ranUe.Ran

	// remove ranUe from oldRan
	for index, ranUe1 := range oldRan.RanUeList {
		if ranUe1 == ranUe {
			oldRan.RanUeList = append(oldRan.RanUeList[:index], oldRan.RanUeList[index+1:]...)
			break
		}
	}

	// add ranUe to newRan
	newRan.RanUeList = append(newRan.RanUeList, ranUe)

	// switch to newRan
	ranUe.Ran = newRan
	ranUe.RanUeNgapId = ranUeNgapId

	logger.AmfLog.Infof("ran ue (%d) switch to new Ran: %s", ranUe.RanUeNgapId, ranUe.Ran.Name)
	return nil
}

func (ranUe *RanUe) UpdateLocation(userLocationInformation *ngapType.UserLocationInformation) {
	if userLocationInformation == nil {
		return
	}

	curTime := time.Now().UTC()
	switch userLocationInformation.Present {
	case ngapType.UserLocationInformationPresentUserLocationInformationEUTRA:
		locationInfoEUTRA := userLocationInformation.UserLocationInformationEUTRA
		if ranUe.Location.EutraLocation == nil {
			ranUe.Location.EutraLocation = new(models.EutraLocation)
		}

		tAI := locationInfoEUTRA.TAI
		plmnID := util.PlmnIdToModels(tAI.PLMNIdentity)
		tac := hex.EncodeToString(tAI.TAC.Value)

		if ranUe.Location.EutraLocation.Tai == nil {
			ranUe.Location.EutraLocation.Tai = new(models.Tai)
		}
		ranUe.Location.EutraLocation.Tai.PlmnId = &plmnID
		ranUe.Location.EutraLocation.Tai.Tac = tac
		ranUe.Tai = *ranUe.Location.EutraLocation.Tai

		eUTRACGI := locationInfoEUTRA.EUTRACGI
		ePlmnID := util.PlmnIdToModels(eUTRACGI.PLMNIdentity)
		eutraCellID := ngapConvert.BitStringToHex(&eUTRACGI.EUTRACellIdentity.Value)

		if ranUe.Location.EutraLocation.Ecgi == nil {
			ranUe.Location.EutraLocation.Ecgi = new(models.Ecgi)
		}
		ranUe.Location.EutraLocation.Ecgi.PlmnId = &ePlmnID
		ranUe.Location.EutraLocation.Ecgi.EutraCellId = eutraCellID
		ranUe.Location.EutraLocation.UeLocationTimestamp = &curTime
		if locationInfoEUTRA.TimeStamp != nil {
			ranUe.Location.EutraLocation.AgeOfLocationInformation = ngapConvert.TimeStampToInt32(
				locationInfoEUTRA.TimeStamp.Value)
		}
		if ranUe.AmfUe != nil {
			if ranUe.AmfUe.Tai != ranUe.Tai {
				ranUe.AmfUe.LocationChanged = true
			}
			ranUe.AmfUe.Location = ranUe.Location
			ranUe.AmfUe.Tai = *ranUe.AmfUe.Location.NrLocation.Tai
		}
	case ngapType.UserLocationInformationPresentUserLocationInformationNR:
		locationInfoNR := userLocationInformation.UserLocationInformationNR
		if ranUe.Location.NrLocation == nil {
			ranUe.Location.NrLocation = new(models.NrLocation)
		}

		tAI := locationInfoNR.TAI
		plmnID := util.PlmnIdToModels(tAI.PLMNIdentity)
		tac := hex.EncodeToString(tAI.TAC.Value)

		if ranUe.Location.NrLocation.Tai == nil {
			ranUe.Location.NrLocation.Tai = new(models.Tai)
		}
		ranUe.Location.NrLocation.Tai.PlmnId = &plmnID
		ranUe.Location.NrLocation.Tai.Tac = tac
		ranUe.Tai = *ranUe.Location.NrLocation.Tai

		nRCGI := locationInfoNR.NRCGI
		nRPlmnID := util.PlmnIdToModels(nRCGI.PLMNIdentity)
		nRCellID := ngapConvert.BitStringToHex(&nRCGI.NRCellIdentity.Value)

		if ranUe.Location.NrLocation.Ncgi == nil {
			ranUe.Location.NrLocation.Ncgi = new(models.Ncgi)
		}
		ranUe.Location.NrLocation.Ncgi.PlmnId = &nRPlmnID
		ranUe.Location.NrLocation.Ncgi.NrCellId = nRCellID
		ranUe.Location.NrLocation.UeLocationTimestamp = &curTime
		if locationInfoNR.TimeStamp != nil {
			ranUe.Location.NrLocation.AgeOfLocationInformation = ngapConvert.TimeStampToInt32(locationInfoNR.TimeStamp.Value)
		}
		if ranUe.AmfUe != nil {
			if ranUe.AmfUe.Tai != ranUe.Tai {
				ranUe.AmfUe.LocationChanged = true
			}
			ranUe.AmfUe.Location = ranUe.Location
			ranUe.AmfUe.Tai = *ranUe.AmfUe.Location.NrLocation.Tai
		}
	case ngapType.UserLocationInformationPresentUserLocationInformationN3IWF:
		locationInfoN3IWF := userLocationInformation.UserLocationInformationN3IWF
		if ranUe.Location.N3gaLocation == nil {
			ranUe.Location.N3gaLocation = new(models.N3gaLocation)
		}

		ip := locationInfoN3IWF.IPAddress
		port := locationInfoN3IWF.PortNumber

		ipv4Addr, ipv6Addr := ngapConvert.IPAddressToString(ip)

		ranUe.Location.N3gaLocation.UeIpv4Addr = ipv4Addr
		ranUe.Location.N3gaLocation.UeIpv6Addr = ipv6Addr
		ranUe.Location.N3gaLocation.PortNumber = ngapConvert.PortNumberToInt(port)

		supportTaiList := GetSupportTaiList()
		tmp, err := strconv.ParseUint(supportTaiList[0].Tac, 10, 32)
		if err != nil {
			logger.AmfLog.Errorf("Error parsing TAC: %v", err)
		}
		tac := fmt.Sprintf("%06x", tmp)
		ranUe.Location.N3gaLocation.N3gppTai = &models.Tai{
			PlmnId: supportTaiList[0].PlmnId,
			Tac:    tac,
		}
		ranUe.Tai = *ranUe.Location.N3gaLocation.N3gppTai

		if ranUe.AmfUe != nil {
			ranUe.AmfUe.Location = ranUe.Location
			ranUe.AmfUe.Tai = *ranUe.Location.N3gaLocation.N3gppTai
		}
	case ngapType.UserLocationInformationPresentNothing:
	}
}
