// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"

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

// RetrieveDnnInformation gets the corresponding dnn info from S-NSSAI and DNN
func RetrieveDnnInformation(ctx context.Context, ueSnssai models.Snssai, dnn string) (*SnssaiSmfDnnInfo, error) {
	supportedSnssai, err := GetSnssaiInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get snssai information: %v", err)
	}

	if supportedSnssai.Snssai.Sst != ueSnssai.Sst {
		return nil, fmt.Errorf("ue requested sst %d, but sst %d is supported", ueSnssai.Sst, supportedSnssai.Snssai.Sst)
	}

	if supportedSnssai.Snssai.Sd != ueSnssai.Sd {
		return nil, fmt.Errorf("ue requested sd %s, but sd %s is supported", ueSnssai.Sd, supportedSnssai.Snssai.Sd)
	}

	return supportedSnssai.DnnInfos[dnn], nil
}

func AllocateLocalSEID() (uint64, error) {
	atomic.AddUint64(&smfContext.LocalSEIDCount, 1)
	return smfContext.LocalSEIDCount, nil
}

func SMFSelf() *SMFContext {
	return &smfContext
}

func GetSnssaiInfo(ctx context.Context) (*SnssaiSmfInfo, error) {
	self := SMFSelf()
	operator, err := self.DBInstance.GetOperator(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get operator information from db: %v", err)
	}

	dataNetworks, _, err := self.DBInstance.ListDataNetworksPage(ctx, 1, 1000)
	if err != nil {
		return nil, fmt.Errorf("failed to list policies from db: %v", err)
	}

	snssaiInfo := &SnssaiSmfInfo{
		Snssai: models.Snssai{
			Sst: operator.Sst,
			Sd:  operator.GetHexSd(),
		},
		PlmnID: models.PlmnID{
			Mcc: operator.Mcc,
			Mnc: operator.Mnc,
		},
		DnnInfos: make(map[string]*SnssaiSmfDnnInfo),
	}

	for _, dn := range dataNetworks {
		dnn := dn.Name
		dnsPrimary := dn.DNS
		mtu := dn.MTU
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
