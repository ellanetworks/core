// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nssf"
	"github.com/omec-project/openapi/models"
)

func NSSelectionGetForRegistration(ue *context.AmfUe, requestedNssai []models.MappingOfSnssai) (
	*models.ProblemDetails, error,
) {
	amfSelf := context.AMFSelf()
	sliceInfo := models.SliceInfoForRegistration{
		SubscribedNssai: ue.SubscribedNssai,
	}

	for _, snssai := range requestedNssai {
		sliceInfo.RequestedNssai = append(sliceInfo.RequestedNssai, *snssai.ServingSnssai)
		if snssai.HomeSnssai != nil {
			sliceInfo.MappingOfNssai = append(sliceInfo.MappingOfNssai, snssai)
		}
	}

	amfType := models.NfType_AMF
	params := nssf.NsselectionQueryParameter{
		NfType:                          &amfType,
		NfID:                            amfSelf.NfID,
		SliceInfoRequestForRegistration: &sliceInfo,
	}

	res, err := nssf.GetNSSelection(params)
	if err != nil {
		logger.AmfLog.Warnf("GetNSSelection failed: %+v", err)
		return nil, err
	}
	ue.NetworkSliceInfo = res
	for _, allowedNssai := range res.AllowedNssaiList {
		ue.AllowedNssai[allowedNssai.AccessType] = allowedNssai.AllowedSnssaiList
	}
	ue.ConfiguredNssai = res.ConfiguredNssai
	return nil, nil
}

func NSSelectionGetForPduSession(ue *context.AmfUe, snssai models.Snssai) (
	*models.AuthorizedNetworkSliceInfo, *models.ProblemDetails, error,
) {
	amfSelf := context.AMFSelf()
	sliceInfoForPduSession := models.SliceInfoForPduSession{
		SNssai:            &snssai,
		RoamingIndication: models.RoamingIndication_NON_ROAMING,
	}

	amfType := models.NfType_AMF
	params := nssf.NsselectionQueryParameter{
		NfType:                        &amfType,
		NfID:                          amfSelf.NfID,
		SliceInfoRequestForPduSession: &sliceInfoForPduSession,
	}

	res, err := nssf.GetNSSelection(params)
	if err != nil {
		logger.AmfLog.Warnf("GetNSSelection failed: %+v", err)
		return nil, nil, err
	}
	return res, nil, nil
}
