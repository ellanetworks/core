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
func RetrieveDnnInformation(ctx context.Context, Snssai models.Snssai, dnn string) (*SnssaiSmfDnnInfo, error) {
	snssaiInfo, err := GetSnssaiInfo(ctx)
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
