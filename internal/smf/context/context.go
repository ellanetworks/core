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
	"github.com/ellanetworks/core/internal/models"
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

func RetrieveDnnInformation(Snssai models.Snssai, dnn string) (*SnssaiSmfDnnInfo, error) {
	snssaiInfo, err := GetSnssaiInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get snssai information: %v", err)
	}
	if snssaiInfo.Snssai.Sst != Snssai.Sst {
		return nil, fmt.Errorf("expected sst %d, got %d", Snssai.Sst, snssaiInfo.Snssai.Sst)
	}
	if snssaiInfo.Snssai.Sd != Snssai.Sd {
		return nil, fmt.Errorf("expected sd %s, got %s", Snssai.Sd, snssaiInfo.Snssai.Sd)
	}
	return snssaiInfo.DnnInfos[dnn], nil
}

func AllocateLocalSEID() (uint64, error) {
	atomic.AddUint64(&smfContext.LocalSEIDCount, 1)
	return smfContext.LocalSEIDCount, nil
}

func SMFSelf() *SMFContext {
	return &smfContext
}

func GetSnssaiInfo() (*SnssaiSmfInfo, error) {
	self := SMFSelf()
	operator, err := self.DBInstance.GetOperator()
	if err != nil {
		return nil, fmt.Errorf("failed to get operator information from db: %v", err)
	}
	profiles, err := self.DBInstance.ListProfiles()
	if err != nil {
		return nil, fmt.Errorf("failed to list profiles from db: %v", err)
	}
	snssaiInfo := &SnssaiSmfInfo{
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
	return snssaiInfo, nil
}
