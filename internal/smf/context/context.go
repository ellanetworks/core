// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"fmt"
	"net"
	"sync/atomic"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

var smfContext SMFContext

type N3InterfaceUpfInfoItem struct {
	NetworkInstance string
}

type SMFContext struct {
	DBInstance     *db.Database
	UPF            *UPF
	CPNodeID       NodeID
	LocalSEIDCount uint64
}

// RetrieveDnnInformation gets the corresponding dnn info from S-NSSAI and DNN
func RetrieveDnnInformation(Snssai models.Snssai, dnn string) *SnssaiSmfDnnInfo {
	snssaiInfo := GetSnssaiInfo()
	for _, snssaiInfo := range snssaiInfo {
		if snssaiInfo.Snssai.Sst == Snssai.Sst && snssaiInfo.Snssai.Sd == Snssai.Sd {
			return snssaiInfo.DnnInfos[dnn]
		}
	}
	return nil
}

func AllocateLocalSEID() (uint64, error) {
	atomic.AddUint64(&smfContext.LocalSEIDCount, 1)
	return smfContext.LocalSEIDCount, nil
}

func SMFSelf() *SMFContext {
	return &smfContext
}

func BuildUserPlaneInformationFromConfig() (*UPF, error) {
	smfSelf := SMFSelf()
	operator, err := smfSelf.DBInstance.GetOperator()
	if err != nil {
		return nil, fmt.Errorf("failed to get operator information from db: %v", err)
	}

	upfNodeID := NewNodeID(config.UpfNodeID)
	upf := NewUPF(upfNodeID, config.DNN)
	upf.SNssaiInfos = []SnssaiUPFInfo{
		{
			SNssai: SNssai{
				Sst: operator.Sst,
				Sd:  operator.GetHexSd(),
			},
			DnnList: []DnnUPFInfoItem{
				{
					Dnn: config.DNN,
				},
			},
		},
	}

	return upf, nil
}

func GetSnssaiInfo() []SnssaiSmfInfo {
	self := SMFSelf()
	operator, err := self.DBInstance.GetOperator()
	if err != nil {
		logger.SmfLog.Warn("failed to get operator information from db", zap.Error(err))
		return nil
	}
	profiles, err := self.DBInstance.ListProfiles()
	if err != nil {
		logger.SmfLog.Warn("failed to get profiles from db", zap.Error(err))
		return nil
	}
	snssaiInfoList := make([]SnssaiSmfInfo, 0)
	snssaiInfo := SnssaiSmfInfo{
		Snssai: SNssai{
			Sst: operator.Sst,
			Sd:  operator.GetHexSd(),
		},
		PlmnID: models.PlmnID{
			Mcc: operator.Mcc,
			Mnc: operator.Mnc,
		},
		DnnInfos: make(map[string]*SnssaiSmfDnnInfo),
	}

	for _, profile := range profiles {
		dnn := config.DNN
		dnsPrimary := profile.DNS
		mtu := profile.Mtu
		dnnInfo := SnssaiSmfDnnInfo{
			DNS: DNS{
				IPv4Addr: net.ParseIP(dnsPrimary).To4(),
			},
			MTU: uint16(mtu),
		}
		snssaiInfo.DnnInfos[dnn] = &dnnInfo
	}
	snssaiInfoList = append(snssaiInfoList, snssaiInfo)
	return snssaiInfoList
}
