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

type SMFContext struct {
	DBInstance     *db.Database
	UPF            *UPF
	CPNodeID       net.IP
	LocalSEIDCount uint64
}

type DNS struct {
	IPv4Addr net.IP
	IPv6Addr net.IP
}

// SnssaiSmfDnnInfo records the SMF per S-NSSAI DNN information
type SnssaiSmfDnnInfo struct {
	DNS DNS
	MTU uint16
}

// SnssaiSmfInfo records the SMF S-NSSAI related information
type SnssaiSmfInfo struct {
	DnnInfos *SnssaiSmfDnnInfo
	Snssai   models.Snssai
}

// RetrieveDnnInformation gets the corresponding dnn info from S-NSSAI and DNN
func RetrieveDnnInformation(ctx context.Context, ueSnssai models.Snssai, dnn string) (*SnssaiSmfDnnInfo, error) {
	supportedSnssai, err := GetSnssaiInfo(ctx, dnn)
	if err != nil {
		return nil, fmt.Errorf("failed to get snssai information: %v", err)
	}

	if supportedSnssai.Snssai.Sst != ueSnssai.Sst {
		return nil, fmt.Errorf("ue requested sst %d, but sst %d is supported", ueSnssai.Sst, supportedSnssai.Snssai.Sst)
	}

	if supportedSnssai.Snssai.Sd != ueSnssai.Sd {
		return nil, fmt.Errorf("ue requested sd %s, but sd %s is supported", ueSnssai.Sd, supportedSnssai.Snssai.Sd)
	}

	return supportedSnssai.DnnInfos, nil
}

func AllocateLocalSEID() (uint64, error) {
	atomic.AddUint64(&smfContext.LocalSEIDCount, 1)
	return smfContext.LocalSEIDCount, nil
}

func SMFSelf() *SMFContext {
	return &smfContext
}

func GetSnssaiInfo(ctx context.Context, dnn string) (*SnssaiSmfInfo, error) {
	self := SMFSelf()

	operator, err := self.DBInstance.GetOperator(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get operator information from db: %v", err)
	}

	dataNetwork, err := self.DBInstance.GetDataNetwork(ctx, dnn)
	if err != nil {
		return nil, fmt.Errorf("failed to list policies from db: %v", err)
	}

	if dataNetwork == nil {
		return nil, fmt.Errorf("data network %s not found", dnn)
	}

	snssaiInfo := &SnssaiSmfInfo{
		Snssai: models.Snssai{
			Sst: operator.Sst,
			Sd:  operator.GetHexSd(),
		},
		DnnInfos: &SnssaiSmfDnnInfo{
			DNS: DNS{
				IPv4Addr: net.ParseIP(dataNetwork.DNS).To4(),
			},
			MTU: uint16(dataNetwork.MTU),
		},
	}

	return snssaiInfo, nil
}
