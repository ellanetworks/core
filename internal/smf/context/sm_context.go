// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"fmt"
	"net"
	"sync"

	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasMessage"
)

type PFCPSessionContext struct {
	LocalSEID  uint64
	RemoteSEID uint64
}

type UPTunnel struct {
	DataPath      *DataPath
	ANInformation struct {
		IPAddress net.IP
		TEID      uint32
	}
}

type SMContext struct {
	Mutex sync.Mutex

	Supi                           string
	Dnn                            string
	Snssai                         *models.Snssai
	Tunnel                         *UPTunnel
	PolicyData                     *models.SmPolicyData
	PFCPContext                    *PFCPSessionContext
	PDUSessionID                   uint8
	PDUSessionReleaseDueToDupPduID bool
}

func CanonicalName(identifier string, pduSessID uint8) string {
	return fmt.Sprintf("%s-%d", identifier, pduSessID)
}

func PDUAddressToNAS(pduAddress net.IP, pduSessionType uint8) ([12]byte, uint8) {
	var addr [12]byte

	copy(addr[:], pduAddress)

	switch pduSessionType {
	case nasMessage.PDUSessionTypeIPv4:
		return addr, 4 + 1
	case nasMessage.PDUSessionTypeIPv4IPv6:
		return addr, 12 + 1
	default:
		return addr, 0
	}
}

func (smContext *SMContext) SetSMPolicyData(smPolicyData *models.SmPolicyData) {
	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	smContext.PolicyData = smPolicyData
}
