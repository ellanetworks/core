// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

type RelAction int

const (
	UeContextN2NormalRelease RelAction = iota
	UeContextReleaseHandover
	UeContextReleaseUeContext
	UeContextReleaseDueToNwInitiatedDeregistraion
)

type RanUe struct {
	RanUeNgapID                      int64
	AmfUeNgapID                      int64
	HandOverType                     ngapType.HandoverType
	SourceUe                         *RanUe
	TargetUe                         *RanUe
	Tai                              models.Tai
	Location                         models.UserLocation
	AmfUe                            *AmfUe
	Radio                            *Radio
	ReleaseAction                    RelAction
	RRCEstablishmentCause            string // Received from initial ue message; pattern: ^[0-9a-fA-F]+$
	UeContextRequest                 bool
	SentInitialContextSetupRequest   bool
	RecvdInitialContextSetupResponse bool /*Received Initial context setup response or not */
	Log                              *zap.Logger
}

func (ranUe *RanUe) Remove() error {
	if ranUe == nil {
		return fmt.Errorf("ran ue is nil")
	}

	if ranUe.AmfUe != nil {
		ranUe.AmfUe.RanUe = nil
		ranUe.AmfUe = nil
	}

	ran := ranUe.Radio
	if ran == nil {
		return fmt.Errorf("ran not found in ranUe")
	}

	for _, ranUe1 := range ran.RanUEs {
		if ranUe1 == ranUe {
			delete(ran.RanUEs, ranUe.RanUeNgapID)
			break
		}
	}

	amfUeNGAPIDGenerator.FreeID(ranUe.AmfUeNgapID)
	logger.AmfLog.Info("ran ue removed", zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))

	return nil
}

func (ranUe *RanUe) SwitchToRan(newRan *Radio, ranUeNgapID int64) error {
	if ranUe == nil {
		return fmt.Errorf("ran ue is nil")
	}

	if newRan == nil {
		return fmt.Errorf("new ran is nil")
	}

	oldRan := ranUe.Radio

	// remove ranUe from oldRan
	for _, ranUe1 := range oldRan.RanUEs {
		if ranUe1 == ranUe {
			delete(oldRan.RanUEs, ranUe.RanUeNgapID)
			break
		}
	}

	// add ranUe to newRan
	newRan.RanUEs[ranUeNgapID] = ranUe

	// switch to newRan
	ranUe.Radio = newRan
	ranUe.RanUeNgapID = ranUeNgapID

	logger.AmfLog.Info("ran ue switch to new Ran", zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))

	return nil
}

func (ranUe *RanUe) UpdateLocation(ctx context.Context, amf *AMF, userLocationInformation *ngapType.UserLocationInformation) {
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
		plmnID := util.PlmnIDToModels(tAI.PLMNIdentity)
		tac := hex.EncodeToString(tAI.TAC.Value)

		if ranUe.Location.EutraLocation.Tai == nil {
			ranUe.Location.EutraLocation.Tai = new(models.Tai)
		}

		ranUe.Location.EutraLocation.Tai.PlmnID = &plmnID
		ranUe.Location.EutraLocation.Tai.Tac = tac
		ranUe.Tai = *ranUe.Location.EutraLocation.Tai

		eUTRACGI := locationInfoEUTRA.EUTRACGI
		ePlmnID := util.PlmnIDToModels(eUTRACGI.PLMNIdentity)
		eutraCellID := ngapConvert.BitStringToHex(&eUTRACGI.EUTRACellIdentity.Value)

		if ranUe.Location.EutraLocation.Ecgi == nil {
			ranUe.Location.EutraLocation.Ecgi = new(models.Ecgi)
		}

		ranUe.Location.EutraLocation.Ecgi.PlmnID = &ePlmnID
		ranUe.Location.EutraLocation.Ecgi.EutraCellID = eutraCellID

		ranUe.Location.EutraLocation.UeLocationTimestamp = &curTime
		if locationInfoEUTRA.TimeStamp != nil {
			ranUe.Location.EutraLocation.AgeOfLocationInformation = ngapConvert.TimeStampToInt32(
				locationInfoEUTRA.TimeStamp.Value)
		}

		if ranUe.AmfUe != nil {
			ranUe.AmfUe.Location = ranUe.Location
			ranUe.AmfUe.Tai = *ranUe.AmfUe.Location.NrLocation.Tai
		}
	case ngapType.UserLocationInformationPresentUserLocationInformationNR:
		locationInfoNR := userLocationInformation.UserLocationInformationNR

		if ranUe.Location.NrLocation == nil {
			ranUe.Location.NrLocation = new(models.NrLocation)
		}

		tAI := locationInfoNR.TAI
		plmnID := util.PlmnIDToModels(tAI.PLMNIdentity)
		tac := hex.EncodeToString(tAI.TAC.Value)

		if ranUe.Location.NrLocation.Tai == nil {
			ranUe.Location.NrLocation.Tai = new(models.Tai)
		}

		ranUe.Location.NrLocation.Tai.PlmnID = &plmnID
		ranUe.Location.NrLocation.Tai.Tac = tac
		ranUe.Tai = *ranUe.Location.NrLocation.Tai

		nRCGI := locationInfoNR.NRCGI
		nRPlmnID := util.PlmnIDToModels(nRCGI.PLMNIdentity)
		nRCellID := ngapConvert.BitStringToHex(&nRCGI.NRCellIdentity.Value)

		if ranUe.Location.NrLocation.Ncgi == nil {
			ranUe.Location.NrLocation.Ncgi = new(models.Ncgi)
		}

		ranUe.Location.NrLocation.Ncgi.PlmnID = &nRPlmnID
		ranUe.Location.NrLocation.Ncgi.NrCellID = nRCellID

		ranUe.Location.NrLocation.UeLocationTimestamp = &curTime
		if locationInfoNR.TimeStamp != nil {
			ranUe.Location.NrLocation.AgeOfLocationInformation = ngapConvert.TimeStampToInt32(locationInfoNR.TimeStamp.Value)
		}

		if ranUe.AmfUe != nil {
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

		operatorInfo, err := amf.GetOperatorInfo(ctx)
		if err != nil {
			logger.AmfLog.Error("Error getting supported TAI list", zap.Error(err))
			return
		}

		tmp, err := strconv.ParseUint(operatorInfo.Tais[0].Tac, 10, 32)
		if err != nil {
			logger.AmfLog.Error("Error parsing TAC", zap.String("Tac", operatorInfo.Tais[0].Tac), zap.Error(err))
		}

		ranUe.Location.N3gaLocation.N3gppTai = &models.Tai{
			PlmnID: operatorInfo.Tais[0].PlmnID,
			Tac:    fmt.Sprintf("%06x", tmp),
		}

		ranUe.Tai = *ranUe.Location.N3gaLocation.N3gppTai

		if ranUe.AmfUe != nil {
			ranUe.AmfUe.Location = ranUe.Location
			ranUe.AmfUe.Tai = *ranUe.Location.N3gaLocation.N3gppTai
		}
	case ngapType.UserLocationInformationPresentNothing:
	}
}
