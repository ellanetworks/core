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
	PDUIPV4Address                 net.IP
	PDUIPV6Prefix                  net.IP  // delegated /64 prefix base address (lower 64 bits = 0)
	IPv6IID                        [8]byte // random Interface Identifier sent to UE
	PDUSessionType                 uint8   // negotiated type: nasMessage.PDUSessionTypeIPv4/IPv6/IPv4IPv6
	PDUSessionReleaseDueToDupPduID bool

	// outstandingPTIs holds the PTI of each 5GSM procedure awaiting a UE
	// completion or reject on this PDU session (TS 24.501 §7.3.1). A completion
	// or command-reject whose PTI is absent is a PTI mismatch (§7.3.1 a).
	ptiMu           sync.Mutex
	outstandingPTIs map[uint8]struct{}
}

// MarkPTIInUse records that a 5GSM procedure with the given PTI is outstanding
// on this PDU session (TS 24.501 §7.3.1).
func (smContext *SMContext) MarkPTIInUse(pti uint8) {
	smContext.ptiMu.Lock()
	defer smContext.ptiMu.Unlock()

	if smContext.outstandingPTIs == nil {
		smContext.outstandingPTIs = make(map[uint8]struct{})
	}

	smContext.outstandingPTIs[pti] = struct{}{}
}

// ClearPTIInUse records that the procedure with the given PTI has completed.
func (smContext *SMContext) ClearPTIInUse(pti uint8) {
	smContext.ptiMu.Lock()
	defer smContext.ptiMu.Unlock()

	delete(smContext.outstandingPTIs, pti)
}

// IsPTIInUse reports whether a procedure with the given PTI is outstanding.
func (smContext *SMContext) IsPTIInUse(pti uint8) bool {
	smContext.ptiMu.Lock()
	defer smContext.ptiMu.Unlock()

	_, ok := smContext.outstandingPTIs[pti]

	return ok
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
