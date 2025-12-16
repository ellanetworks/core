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
	"sync"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/qos"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
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
	SmPolicyUpdates                *qos.PolicyUpdate
	SmPolicyData                   qos.SmCtxtPolicyData
	PFCPContext                    map[string]*PFCPSessionContext // key: UPF NodeID
	PDUSessionID                   uint8
	PDUSessionReleaseDueToDupPduID bool
}

func CanonicalName(identifier string, pduSessID uint8) string {
	return fmt.Sprintf("%s-%d", identifier, pduSessID)
}

func NewSMContext(supi string, pduSessID uint8) *SMContext {
	smfContext.Mutex.Lock()
	defer smfContext.Mutex.Unlock()

	smContext := new(SMContext)

	ref := CanonicalName(supi, pduSessID)
	smfContext.smContextPool[ref] = smContext
	smContext.PDUSessionID = pduSessID
	smContext.PFCPContext = make(map[string]*PFCPSessionContext)

	return smContext
}

func GetSMContext(ref string) *SMContext {
	smfContext.Mutex.Lock()
	defer smfContext.Mutex.Unlock()

	value, ok := smfContext.smContextPool[ref]
	if !ok {
		return nil
	}

	return value
}

func GetPDUSessionCount() int {
	smfContext.Mutex.Lock()
	defer smfContext.Mutex.Unlock()

	return len(smfContext.smContextPool)
}

func RemoveSMContext(ctx context.Context, ref string) {
	smfContext.Mutex.Lock()
	defer smfContext.Mutex.Unlock()

	smContext, ok := smfContext.smContextPool[ref]
	if !ok {
		return
	}

	for _, pfcpSessionContext := range smContext.PFCPContext {
		delete(smfContext.seidSMContextMap, pfcpSessionContext.LocalSEID)
	}

	err := ReleaseUeIPAddr(ctx, smContext.Supi)
	if err != nil {
		logger.SmfLog.Error("release UE IP-Address failed", zap.Error(err), zap.String("smContextRef", ref))
	}

	delete(smfContext.smContextPool, ref)

	logger.SmfLog.Info("SM Context removed", zap.String("smContextRef", ref))
}

func GetSMContextBySEID(SEID uint64) *SMContext {
	smfContext.Mutex.Lock()
	defer smfContext.Mutex.Unlock()

	value, ok := smfContext.seidSMContextMap[SEID]
	if !ok {
		return nil
	}

	return value
}

func ReleaseUeIPAddr(ctx context.Context, supi string) error {
	smfSelf := SMFSelf()

	err := smfSelf.DBInstance.ReleaseIP(ctx, supi)
	if err != nil {
		return fmt.Errorf("failed to release IP Address, %v", err)
	}

	logger.SmfLog.Info("Released IP Address", zap.String("supi", supi))

	return nil
}

func (smContext *SMContext) SetCreateData(createData *models.SmContextCreateData) {
	smContext.Supi = createData.Supi
	smContext.Dnn = createData.Dnn
	smContext.Snssai = createData.SNssai
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

func (smContext *SMContext) AllocateLocalSEIDForDataPath(dataPath *DataPath) error {
	curDataPathNode := dataPath.DPNode
	NodeIDtoIP := curDataPathNode.UPF.NodeID.String()

	_, exist := smContext.PFCPContext[NodeIDtoIP]
	if exist {
		return nil
	}

	allocatedSEID := AllocateLocalSEID()

	smContext.PFCPContext[NodeIDtoIP] = &PFCPSessionContext{
		LocalSEID: allocatedSEID,
	}

	smfContext.Mutex.Lock()
	defer smfContext.Mutex.Unlock()
	smfContext.seidSMContextMap[allocatedSEID] = smContext

	return nil
}

func isAllowedPDUSessionType(allowedPDUSessionType models.PduSessionType, requestedPDUSessionType uint8) (uint8, error) {
	allowIPv4 := false
	allowIPv6 := false
	allowEthernet := false

	switch allowedPDUSessionType {
	case models.PduSessionTypeIPv4:
		allowIPv4 = true
	case models.PduSessionTypeIPv6:
		allowIPv6 = true
	case models.PduSessionTypeIPv4v6:
		allowIPv4 = true
		allowIPv6 = true
	case models.PduSessionTypeEthernet:
		allowEthernet = true
	}

	if !allowIPv4 {
		return 0, fmt.Errorf("PduSessionTypeIPv4 is not allowed")
	}

	switch requestedPDUSessionType {
	case nasMessage.PDUSessionTypeIPv4:
		if !allowIPv4 {
			return 0, fmt.Errorf("PduSessionTypeIPv4 is not allowed")
		}
	case nasMessage.PDUSessionTypeIPv6:
		if !allowIPv6 {
			return 0, fmt.Errorf("PduSessionTypeIPv6 is not allowed")
		}
	case nasMessage.PDUSessionTypeIPv4IPv6:
		if allowIPv4 && allowIPv6 {
		} else if allowIPv4 {
			return nasMessage.Cause5GSMPDUSessionTypeIPv4OnlyAllowed, nil
		} else if allowIPv6 {
			return nasMessage.Cause5GSMPDUSessionTypeIPv6OnlyAllowed, nil
		} else {
			return 0, fmt.Errorf("PduSessionTypeIPv4v6 is not allowed")
		}
	case nasMessage.PDUSessionTypeEthernet:
		if !allowEthernet {
			return 0, fmt.Errorf("PduSessionTypeEthernet is not allowed")
		}
	default:
		return 0, fmt.Errorf("requested PDU Session type[%d] is not supported", requestedPDUSessionType)
	}

	return 0, nil
}

// SelectedSessionRule - return the SMF selected session rule for this SM Context
func SelectedSessionRule(smPolicyUpdates *qos.PolicyUpdate, qosPolicyData qos.SmCtxtPolicyData) *models.SessionRule {
	if smPolicyUpdates != nil {
		return smPolicyUpdates.SessRuleUpdate.ActiveSessRule
	}

	return qosPolicyData.SmCtxtSessionRules.ActiveRule
}

func GeneratePDUSessionEstablishmentReject(pduSessionID uint8, pti uint8, cause uint8) *models.PostSmContextsErrorResponse {
	buf, err := BuildGSMPDUSessionEstablishmentReject(pduSessionID, pti, cause)
	if err != nil {
		return &models.PostSmContextsErrorResponse{}
	}

	return &models.PostSmContextsErrorResponse{
		BinaryDataN1SmMessage: buf,
		Cause:                 cause,
	}
}

func (smContext *SMContext) CommitSmPolicyDecision(status bool) error {
	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if status {
		err := qos.CommitSmPolicyDecision(&smContext.SmPolicyData, smContext.SmPolicyUpdates)
		if err != nil {
			logger.SmfLog.Error("failed to commit SM Policy Decision", zap.Error(err))
		}
	}

	smContext.SmPolicyUpdates = nil

	return nil
}

func PDUSessionsByDNN(dnn string) []*SMContext {
	smfContext.Mutex.Lock()
	defer smfContext.Mutex.Unlock()

	var out []*SMContext

	for _, smContext := range smfContext.smContextPool {
		if smContext.Dnn == dnn {
			out = append(out, smContext)
		}
	}

	return out
}
