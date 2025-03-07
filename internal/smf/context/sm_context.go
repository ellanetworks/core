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
	"sync/atomic"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/qos"
	"github.com/ellanetworks/core/internal/smf/util"
	"github.com/google/uuid"
	"github.com/omec-project/nas/nasMessage"
	"go.uber.org/zap"
)

const (
	CONNECTED               = "Connected"
	DISCONNECTED            = "Disconnected"
	IDLE                    = "Idle"
	PDU_SESS_REL_CMD string = "PDUSessionReleaseCommand"
)

var (
	smContextPool    sync.Map
	canonicalRef     sync.Map
	seidSMContextMap sync.Map
)

var smContextActive uint64

type UeIpAddr struct {
	Ip          net.IP
	UpfProvided bool
}

type SMContext struct {
	Ref                          string
	Supi                         string
	Pei                          string
	Identifier                   string
	Gpsi                         string
	Dnn                          string
	UeTimeZone                   string
	ServingNfId                  string
	SmStatusNotifyUri            string
	UpCnxState                   models.UpCnxState
	AnType                       models.AccessType
	RatType                      models.RatType
	PresenceInLadn               models.PresenceState
	HoState                      models.HoState
	DnnConfiguration             models.DnnConfiguration
	Snssai                       *models.Snssai
	ServingNetwork               *models.PlmnID
	UeLocation                   *models.UserLocation
	PDUAddress                   *UeIpAddr
	Tunnel                       *UPTunnel
	BPManager                    *BPManager
	DNNInfo                      *SnssaiSmfDnnInfo
	ProtocolConfigurationOptions *ProtocolConfigurationOptions
	SubGsmLog                    *zap.SugaredLogger
	SubPfcpLog                   *zap.SugaredLogger
	SubPduSessLog                *zap.SugaredLogger
	SubCtxLog                    *zap.SugaredLogger
	SubFsmLog                    *zap.SugaredLogger
	SmPolicyUpdates              []*qos.PolicyUpdate
	SmPolicyData                 qos.SmCtxtPolicyData
	PendingUPF                   PendingUPF
	PFCPContext                  map[string]*PFCPSessionContext
	SMLock                       sync.Mutex
	// SMContextState                      SMContextState
	PDUSessionID                        int32
	OldPduSessionID                     int32
	SelectedPDUSessionType              uint8
	PDUSessionRelease_DUE_TO_DUP_PDU_ID bool
	LocalPurged                         bool
	Pti                                 uint8
	EstAcceptCause5gSMValue             uint8
}

func canonicalName(identifier string, pduSessID int32) (canonical string) {
	return fmt.Sprintf("%s-%d", identifier, pduSessID)
}

func ResolveRef(identifier string, pduSessID int32) (ref string, err error) {
	if value, ok := canonicalRef.Load(canonicalName(identifier, pduSessID)); ok {
		ref = value.(string)
		err = nil
	} else {
		ref = ""
		err = fmt.Errorf(
			"UE '%s' - PDUSessionID '%d' not found in SMContext", identifier, pduSessID)
	}
	return
}

func incSMContextActive() {
	atomic.AddUint64(&smContextActive, 1)
}

func decSMContextActive() {
	atomic.AddUint64(&smContextActive, ^uint64(0))
}

func NewSMContext(identifier string, pduSessID int32) (smContext *SMContext) {
	smContext = new(SMContext)
	// Create Ref and identifier
	smContext.Ref = uuid.New().URN()
	smContextPool.Store(smContext.Ref, smContext)
	canonicalRef.Store(canonicalName(identifier, pduSessID), smContext.Ref)

	smContext.Identifier = identifier
	smContext.PDUSessionID = pduSessID
	smContext.PFCPContext = make(map[string]*PFCPSessionContext)

	// initialize SM Policy Data
	smContext.SmPolicyUpdates = make([]*qos.PolicyUpdate, 0)
	smContext.SmPolicyData.Initialize()

	smContext.ProtocolConfigurationOptions = &ProtocolConfigurationOptions{
		DNSIPv4Request: false,
		DNSIPv6Request: false,
	}

	// Sess Stats
	incSMContextActive()

	// initialise log tags
	smContext.initLogTags()

	return smContext
}

func (smContext *SMContext) initLogTags() {
	smContext.SubPfcpLog = logger.SmfLog.With("uuid", smContext.Ref, "id", smContext.Identifier, "pduid", smContext.PDUSessionID)
	smContext.SubCtxLog = logger.SmfLog.With("uuid", smContext.Ref, "id", smContext.Identifier, "pduid", smContext.PDUSessionID)
	smContext.SubPduSessLog = logger.SmfLog.With("uuid", smContext.Ref, "id", smContext.Identifier, "pduid", smContext.PDUSessionID)
	smContext.SubGsmLog = logger.SmfLog.With("uuid", smContext.Ref, "id", smContext.Identifier, "pduid", smContext.PDUSessionID)
	smContext.SubFsmLog = logger.SmfLog.With("uuid", smContext.Ref, "id", smContext.Identifier, "pduid", smContext.PDUSessionID)
}

func GetSMContext(ref string) (smContext *SMContext) {
	if value, ok := smContextPool.Load(ref); ok {
		smContext = value.(*SMContext)
	}
	return
}

func GetPDUSessionCount() int {
	return int(smContextActive)
}

func RemoveSMContext(ref string) {
	var smContext *SMContext
	if value, ok := smContextPool.Load(ref); ok {
		smContext = value.(*SMContext)
	}

	for _, pfcpSessionContext := range smContext.PFCPContext {
		seidSMContextMap.Delete(pfcpSessionContext.LocalSEID)
	}

	// Release UE IP-Address
	err := smContext.ReleaseUeIpAddr()
	if err != nil {
		smContext.SubCtxLog.Errorf("release UE IP-Address failed, %v", err)
	}

	smContextPool.Delete(ref)

	canonicalRef.Delete(canonicalName(smContext.Supi, smContext.PDUSessionID))
	decSMContextActive()
	smContext.SubCtxLog.Infof("SM Context removed: %s", ref)
}

func GetSMContextBySEID(SEID uint64) (smContext *SMContext) {
	if value, ok := seidSMContextMap.Load(SEID); ok {
		smContext = value.(*SMContext)
	}
	return
}

func (smContext *SMContext) ReleaseUeIpAddr() error {
	smfSelf := SMF_Self()
	if smContext.PDUAddress == nil {
		return nil
	}
	if ip := smContext.PDUAddress.Ip; ip != nil && !smContext.PDUAddress.UpfProvided {
		err := smfSelf.DbInstance.ReleaseIP(smContext.Supi)
		if err != nil {
			return fmt.Errorf("failed to release IP Address, %v", err)
		}
		smContext.SubPduSessLog.Infof("Released IP Address: %s", smContext.PDUAddress.Ip.String())
		smContext.PDUAddress.Ip = net.IPv4(0, 0, 0, 0)
	}
	return nil
}

func (smContext *SMContext) SetCreateData(createData *models.SmContextCreateData) {
	smContext.Gpsi = createData.Gpsi
	smContext.Supi = createData.Supi
	smContext.Dnn = createData.Dnn
	smContext.Snssai = createData.SNssai
	smContext.ServingNetwork = createData.ServingNetwork
	smContext.AnType = createData.AnType
	smContext.RatType = createData.RatType
	smContext.PresenceInLadn = createData.PresenceInLadn
	smContext.UeLocation = createData.UeLocation
	smContext.UeTimeZone = createData.UeTimeZone
	smContext.OldPduSessionID = createData.OldPduSessionID
	smContext.ServingNfId = createData.ServingNfId
}

func (smContext *SMContext) BuildCreatedData() (createdData *models.SmContextCreatedData) {
	createdData = new(models.SmContextCreatedData)
	createdData.SNssai = smContext.Snssai
	return
}

func (smContext *SMContext) PDUAddressToNAS() (addr [12]byte, addrLen uint8) {
	copy(addr[:], smContext.PDUAddress.Ip)
	switch smContext.SelectedPDUSessionType {
	case nasMessage.PDUSessionTypeIPv4:
		addrLen = 4 + 1
	case nasMessage.PDUSessionTypeIPv6:
	case nasMessage.PDUSessionTypeIPv4IPv6:
		addrLen = 12 + 1
	}
	return
}

func (smContext *SMContext) GetNodeIDByLocalSEID(seid uint64) (nodeID NodeID) {
	for _, pfcpCtx := range smContext.PFCPContext {
		if pfcpCtx.LocalSEID == seid {
			nodeID = pfcpCtx.NodeID
		}
	}

	return
}

func (smContext *SMContext) AllocateLocalSEIDForDataPath(dataPath *DataPath) error {
	for curDataPathNode := dataPath.FirstDPNode; curDataPathNode != nil; curDataPathNode = curDataPathNode.Next() {
		NodeIDtoIP := curDataPathNode.UPF.NodeID.ResolveNodeIdToIp().String()
		if _, exist := smContext.PFCPContext[NodeIDtoIP]; !exist {
			allocatedSEID, err := AllocateLocalSEID()
			if err != nil {
				return fmt.Errorf("failed allocating SEID, %v", err)
			}
			smContext.PFCPContext[NodeIDtoIP] = &PFCPSessionContext{
				PDRs:      make(map[uint16]*PDR),
				NodeID:    curDataPathNode.UPF.NodeID,
				LocalSEID: allocatedSEID,
			}

			seidSMContextMap.Store(allocatedSEID, smContext)
		}
	}
	return nil
}

func (smContext *SMContext) PutPDRtoPFCPSession(nodeID NodeID, pdrList map[string]*PDR) error {
	NodeIDtoIP := nodeID.ResolveNodeIdToIp().String()
	if pfcpSessCtx, exist := smContext.PFCPContext[NodeIDtoIP]; exist {
		for name, pdr := range pdrList {
			pfcpSessCtx.PDRs[pdrList[name].PDRID] = pdr
		}
	} else {
		return fmt.Errorf("error, can't find PFCPContext[%s] to put PDR(%v)", NodeIDtoIP, pdrList)
	}
	return nil
}

func (smContext *SMContext) RemovePDRfromPFCPSession(nodeID NodeID, pdr *PDR) {
	NodeIDtoIP := nodeID.ResolveNodeIdToIp().String()
	pfcpSessCtx := smContext.PFCPContext[NodeIDtoIP]
	delete(pfcpSessCtx.PDRs, pdr.PDRID)
}

func (smContext *SMContext) isAllowedPDUSessionType(requestedPDUSessionType uint8) error {
	dnnPDUSessionType := smContext.DnnConfiguration.PduSessionTypes
	if dnnPDUSessionType == nil {
		return fmt.Errorf("this SMContext[%s] has no subscription pdu session type info", smContext.Ref)
	}

	allowIPv4 := false
	allowIPv6 := false
	allowEthernet := false

	for _, allowedPDUSessionType := range smContext.DnnConfiguration.PduSessionTypes.AllowedSessionTypes {
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
	}

	if !allowIPv4 {
		return fmt.Errorf("PduSessionTypeIPv4 is not allowed in DNN[%s] configuration", smContext.Dnn)
	}

	smContext.EstAcceptCause5gSMValue = 0
	switch util.PDUSessionTypeToModels(requestedPDUSessionType) {
	case models.PduSessionTypeIPv4:
		if allowIPv4 {
			smContext.SelectedPDUSessionType = util.ModelsToPDUSessionType(models.PduSessionTypeIPv4)
		} else {
			return fmt.Errorf("PduSessionTypeIPv4 is not allowed in DNN[%s] configuration", smContext.Dnn)
		}
	case models.PduSessionTypeIPv6:
		if allowIPv6 {
			smContext.SelectedPDUSessionType = util.ModelsToPDUSessionType(models.PduSessionTypeIPv6)
		} else {
			return fmt.Errorf("PduSessionTypeIPv6 is not allowed in DNN[%s] configuration", smContext.Dnn)
		}
	case models.PduSessionTypeIPv4v6:
		if allowIPv4 && allowIPv6 {
			smContext.SelectedPDUSessionType = util.ModelsToPDUSessionType(models.PduSessionTypeIPv4v6)
		} else if allowIPv4 {
			smContext.SelectedPDUSessionType = util.ModelsToPDUSessionType(models.PduSessionTypeIPv4)
			smContext.EstAcceptCause5gSMValue = nasMessage.Cause5GSMPDUSessionTypeIPv4OnlyAllowed
		} else if allowIPv6 {
			smContext.SelectedPDUSessionType = util.ModelsToPDUSessionType(models.PduSessionTypeIPv6)
			smContext.EstAcceptCause5gSMValue = nasMessage.Cause5GSMPDUSessionTypeIPv6OnlyAllowed
		} else {
			return fmt.Errorf("PduSessionTypeIPv4v6 is not allowed in DNN[%s] configuration", smContext.Dnn)
		}
	case models.PduSessionTypeEthernet:
		if allowEthernet {
			smContext.SelectedPDUSessionType = util.ModelsToPDUSessionType(models.PduSessionTypeEthernet)
		} else {
			return fmt.Errorf("PduSessionTypeEthernet is not allowed in DNN[%s] configuration", smContext.Dnn)
		}
	default:
		return fmt.Errorf("requested PDU Sesstion type[%d] is not supported", requestedPDUSessionType)
	}
	return nil
}

// SM Policy related operation

// SelectedSessionRule - return the SMF selected session rule for this SM Context
func (smContext *SMContext) SelectedSessionRule() *models.SessionRule {
	// Policy update in progress
	if len(smContext.SmPolicyUpdates) > 0 {
		return smContext.SmPolicyUpdates[0].SessRuleUpdate.ActiveSessRule
	} else {
		return smContext.SmPolicyData.SmCtxtSessionRules.ActiveRule
	}
}

func (smContext *SMContext) GeneratePDUSessionEstablishmentReject(cause uint8) *models.PostSmContextsErrorResponse {
	var rsp *models.PostSmContextsErrorResponse

	if buf, err := BuildGSMPDUSessionEstablishmentReject(smContext, cause); err != nil {
		rsp = &models.PostSmContextsErrorResponse{}
	} else {
		rsp = &models.PostSmContextsErrorResponse{
			BinaryDataN1SmMessage: buf,
		}
	}
	return rsp
}

func (smContext *SMContext) CommitSmPolicyDecision(status bool) error {
	// Lock SM context
	smContext.SMLock.Lock()
	defer smContext.SMLock.Unlock()

	if status {
		err := qos.CommitSmPolicyDecision(&smContext.SmPolicyData, smContext.SmPolicyUpdates[0])
		if err != nil {
			logger.SmfLog.Errorf("failed to commit SM Policy Decision, %v", err)
		}
	}

	// Release 0th index update
	if len(smContext.SmPolicyUpdates) >= 1 {
		smContext.SmPolicyUpdates = smContext.SmPolicyUpdates[1:]
	}

	return nil
}
