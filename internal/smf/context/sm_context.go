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
	UpCnxState                     models.UpCnxState
	AllowedSessionType             models.PduSessionType
	Snssai                         *models.Snssai
	PDUAddress                     net.IP
	Tunnel                         *UPTunnel
	DNNInfo                        *SnssaiSmfDnnInfo
	SmPolicyUpdates                []*qos.PolicyUpdate
	SmPolicyData                   qos.SmCtxtPolicyData
	PFCPContext                    map[string]*PFCPSessionContext // key: UPF NodeID
	PDUSessionID                   int32
	SelectedPDUSessionType         uint8
	PDUSessionReleaseDueToDupPduID bool
	Pti                            uint8
	EstAcceptCause5gSMValue        uint8
}

func CanonicalName(identifier string, pduSessID int32) string {
	return fmt.Sprintf("%s-%d", identifier, pduSessID)
}

func NewSMContext(supi string, pduSessID int32) *SMContext {
	smfContext.Mutex.Lock()
	defer smfContext.Mutex.Unlock()

	smContext := new(SMContext)

	ref := CanonicalName(supi, pduSessID)
	smfContext.smContextPool[ref] = smContext
	smContext.PDUSessionID = pduSessID
	smContext.PFCPContext = make(map[string]*PFCPSessionContext)
	smContext.SmPolicyUpdates = make([]*qos.PolicyUpdate, 0)

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

	err := smContext.ReleaseUeIPAddr(ctx)
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

func (smContext *SMContext) ReleaseUeIPAddr(ctx context.Context) error {
	smfSelf := SMFSelf()
	if smContext.PDUAddress == nil {
		return nil
	}
	if ip := smContext.PDUAddress; ip != nil {
		err := smfSelf.DBInstance.ReleaseIP(ctx, smContext.Supi)
		if err != nil {
			return fmt.Errorf("failed to release IP Address, %v", err)
		}
		logger.SmfLog.Info("Released IP Address", zap.String("IP", smContext.PDUAddress.String()), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
		smContext.PDUAddress = net.IPv4(0, 0, 0, 0)
	}
	return nil
}

func (smContext *SMContext) SetCreateData(createData *models.SmContextCreateData) {
	smContext.Supi = createData.Supi
	smContext.Dnn = createData.Dnn
	smContext.Snssai = createData.SNssai
}

func (smContext *SMContext) PDUAddressToNAS() ([12]byte, uint8) {
	var addr [12]byte

	copy(addr[:], smContext.PDUAddress)

	switch smContext.SelectedPDUSessionType {
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

func (smContext *SMContext) isAllowedPDUSessionType(requestedPDUSessionType uint8) error {
	allowedPDUSessionType := smContext.AllowedSessionType
	if allowedPDUSessionType == "" {
		return fmt.Errorf("this SMContext [%s-%d] has no subscription pdu session type info", smContext.Supi, smContext.PDUSessionID)
	}

	allowIPv4 := false
	allowIPv6 := false
	allowEthernet := false

	switch smContext.AllowedSessionType {
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
		return fmt.Errorf("PduSessionTypeIPv4 is not allowed in DNN[%s] configuration", smContext.Dnn)
	}

	smContext.EstAcceptCause5gSMValue = 0
	switch requestedPDUSessionType {
	case nasMessage.PDUSessionTypeIPv4:
		if allowIPv4 {
			smContext.SelectedPDUSessionType = nasMessage.PDUSessionTypeIPv4
		} else {
			return fmt.Errorf("PduSessionTypeIPv4 is not allowed in DNN[%s] configuration", smContext.Dnn)
		}
	case nasMessage.PDUSessionTypeIPv6:
		if allowIPv6 {
			smContext.SelectedPDUSessionType = nasMessage.PDUSessionTypeIPv6
		} else {
			return fmt.Errorf("PduSessionTypeIPv6 is not allowed in DNN[%s] configuration", smContext.Dnn)
		}
	case nasMessage.PDUSessionTypeIPv4IPv6:
		if allowIPv4 && allowIPv6 {
			smContext.SelectedPDUSessionType = nasMessage.PDUSessionTypeIPv4IPv6
		} else if allowIPv4 {
			smContext.SelectedPDUSessionType = nasMessage.PDUSessionTypeIPv4
			smContext.EstAcceptCause5gSMValue = nasMessage.Cause5GSMPDUSessionTypeIPv4OnlyAllowed
		} else if allowIPv6 {
			smContext.SelectedPDUSessionType = nasMessage.PDUSessionTypeIPv6
			smContext.EstAcceptCause5gSMValue = nasMessage.Cause5GSMPDUSessionTypeIPv6OnlyAllowed
		} else {
			return fmt.Errorf("PduSessionTypeIPv4v6 is not allowed in DNN[%s] configuration", smContext.Dnn)
		}
	case nasMessage.PDUSessionTypeEthernet:
		if allowEthernet {
			smContext.SelectedPDUSessionType = nasMessage.PDUSessionTypeEthernet
		} else {
			return fmt.Errorf("PduSessionTypeEthernet is not allowed in DNN[%s] configuration", smContext.Dnn)
		}
	default:
		return fmt.Errorf("requested PDU Sesstion type[%d] is not supported", requestedPDUSessionType)
	}
	return nil
}

// SelectedSessionRule - return the SMF selected session rule for this SM Context
func (smContext *SMContext) SelectedSessionRule() *models.SessionRule {
	// Policy update in progress
	if len(smContext.SmPolicyUpdates) > 0 {
		return smContext.SmPolicyUpdates[0].SessRuleUpdate.ActiveSessRule
	}

	return smContext.SmPolicyData.SmCtxtSessionRules.ActiveRule
}

func (smContext *SMContext) GeneratePDUSessionEstablishmentReject(cause uint8) *models.PostSmContextsErrorResponse {
	buf, err := BuildGSMPDUSessionEstablishmentReject(smContext, cause)
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
		err := qos.CommitSmPolicyDecision(&smContext.SmPolicyData, smContext.SmPolicyUpdates[0])
		if err != nil {
			logger.SmfLog.Error("failed to commit SM Policy Decision", zap.Error(err))
		}
	}

	// Release 0th index update
	if len(smContext.SmPolicyUpdates) >= 1 {
		smContext.SmPolicyUpdates = smContext.SmPolicyUpdates[1:]
	}

	return nil
}

func PDUSessionsByIMSI(imsi string) []*SMContext {
	smfContext.Mutex.Lock()
	defer smfContext.Mutex.Unlock()

	var out []*SMContext

	for _, smContext := range smfContext.smContextPool {
		if smContext.Supi == imsi {
			out = append(out, smContext)
		}
	}

	return out
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
