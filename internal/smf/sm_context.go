// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package smf

import (
	"fmt"
	"net"
	"sync"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
)

type PFCPSessionContext struct {
	LocalSEID  uint64
	RemoteSEID uint64
}

type UPTunnel struct {
	DataPath      *DataPath
	ANInformation struct {
		IPv4Address net.IP
		IPv6Address net.IP
		TEID        uint32
	}
}

type SMContext struct {
	Mutex sync.Mutex

	Supi                           etsi.SUPI
	Dnn                            string
	Snssai                         *models.Snssai
	Tunnel                         *UPTunnel
	PolicyData                     *Policy
	PFCPContext                    *PFCPSessionContext
	PDUSessionID                   uint8
	PDUAddress                     net.IP
	PDUSessionReleaseDueToDupPduID bool
}

func CanonicalName(identifier etsi.SUPI, pduSessID uint8) string {
	return fmt.Sprintf("%s-%d", identifier.String(), pduSessID)
}

func (smContext *SMContext) SetPolicyData(policy *Policy) {
	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	smContext.PolicyData = policy
}

func (smContext *SMContext) SetPFCPSession(seid uint64) {
	if smContext.PFCPContext != nil {
		return
	}

	smContext.PFCPContext = &PFCPSessionContext{
		LocalSEID: seid,
	}
}

func (smContext *SMContext) CanonicalName() string {
	return CanonicalName(smContext.Supi, smContext.PDUSessionID)
}
