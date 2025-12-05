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
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// We should likely combine these three maps into a single sync.Map and unify the key ID
var (
	smContextPool    sync.Map // key: smContext.Ref, value: *SMContext
	canonicalRef     sync.Map // key: canonicalName(identifier, pduSessID), value: smContext.Ref
	seidSMContextMap sync.Map // key: PFCP SEID, value: *SMContext
)

type ProtocolConfigurationOptions struct {
	DNSIPv4Request     bool
	DNSIPv6Request     bool
	IPv4LinkMTURequest bool
}

type PFCPSessionContext struct {
	PDRs       map[uint16]*PDR
	NodeID     net.IP
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
	Ref                            string
	Supi                           string
	Identifier                     string
	Gpsi                           string
	Dnn                            string
	UeTimeZone                     string
	UpCnxState                     models.UpCnxState
	AnType                         models.AccessType
	RatType                        models.RatType
	DnnConfiguration               models.DnnConfiguration
	Snssai                         *models.Snssai
	ServingNetwork                 *models.PlmnID
	UeLocation                     *models.UserLocation
	PDUAddress                     net.IP
	Tunnel                         *UPTunnel
	DNNInfo                        *SnssaiSmfDnnInfo
	ProtocolConfigurationOptions   *ProtocolConfigurationOptions
	SubGsmLog                      *zap.Logger
	SubPfcpLog                     *zap.Logger
	SubPduSessLog                  *zap.Logger
	SubCtxLog                      *zap.Logger
	SmPolicyUpdates                []*qos.PolicyUpdate
	SmPolicyData                   qos.SmCtxtPolicyData
	PFCPContext                    map[string]*PFCPSessionContext
	SMLock                         sync.Mutex
	PDUSessionID                   int32
	SelectedPDUSessionType         uint8
	PDUSessionReleaseDueToDupPduID bool
	Pti                            uint8
	EstAcceptCause5gSMValue        uint8
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

func NewSMContext(identifier string, pduSessID int32) *SMContext {
	smContext := new(SMContext)
	// Create Ref and identifier
	smContext.Ref = uuid.New().URN()
	smContextPool.Store(smContext.Ref, smContext)
	canonicalRef.Store(canonicalName(identifier, pduSessID), smContext.Ref)

	smContext.Identifier = identifier
	smContext.PDUSessionID = pduSessID
	smContext.PFCPContext = make(map[string]*PFCPSessionContext)

	// initialize SM Policy Data
	smContext.SmPolicyUpdates = make([]*qos.PolicyUpdate, 0)

	smContext.ProtocolConfigurationOptions = &ProtocolConfigurationOptions{
		DNSIPv4Request: false,
		DNSIPv6Request: false,
	}

	// initialise log tags
	smContext.initLogTags()

	return smContext
}

func (smContext *SMContext) initLogTags() {
	smContext.SubPfcpLog = logger.SmfLog.With(zap.String("uuid", smContext.Ref), zap.String("id", smContext.Identifier), zap.Int32("pduid", smContext.PDUSessionID))
	smContext.SubCtxLog = logger.SmfLog.With(zap.String("uuid", smContext.Ref), zap.String("id", smContext.Identifier), zap.Int32("pduid", smContext.PDUSessionID))
	smContext.SubPduSessLog = logger.SmfLog.With(zap.String("uuid", smContext.Ref), zap.String("id", smContext.Identifier), zap.Int32("pduid", smContext.PDUSessionID))
	smContext.SubGsmLog = logger.SmfLog.With(zap.String("uuid", smContext.Ref), zap.String("id", smContext.Identifier), zap.Int32("pduid", smContext.PDUSessionID))
}

func GetSMContext(ref string) (smContext *SMContext) {
	if value, ok := smContextPool.Load(ref); ok {
		smContext = value.(*SMContext)
	}
	return
}

func GetPDUSessionCount() int {
	count := 0
	smContextPool.Range(func(_ any, _ any) bool {
		count++
		return true
	})
	return count
}

func RemoveSMContext(ctx context.Context, ref string) {
	var smContext *SMContext
	if value, ok := smContextPool.Load(ref); ok {
		smContext = value.(*SMContext)
	}

	for _, pfcpSessionContext := range smContext.PFCPContext {
		seidSMContextMap.Delete(pfcpSessionContext.LocalSEID)
	}

	// Release UE IP-Address
	err := smContext.ReleaseUeIPAddr(ctx)
	if err != nil {
		smContext.SubCtxLog.Error("release UE IP-Address failed", zap.Error(err))
	}

	smContextPool.Delete(ref)

	canonicalRef.Delete(canonicalName(smContext.Supi, smContext.PDUSessionID))

	smContext.SubCtxLog.Info("SM Context removed", zap.String("ref", ref))
}

func GetSMContextBySEID(SEID uint64) (smContext *SMContext) {
	if value, ok := seidSMContextMap.Load(SEID); ok {
		smContext = value.(*SMContext)
	}
	return
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
		smContext.SubPduSessLog.Info("Released IP Address", zap.String("IP", smContext.PDUAddress.String()))
		smContext.PDUAddress = net.IPv4(0, 0, 0, 0)
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
	smContext.UeLocation = createData.UeLocation
	smContext.UeTimeZone = createData.UeTimeZone
}

func (smContext *SMContext) BuildCreatedData() (createdData *models.SmContextCreatedData) {
	createdData = new(models.SmContextCreatedData)
	createdData.SNssai = smContext.Snssai
	return
}

func (smContext *SMContext) PDUAddressToNAS() (addr [12]byte, addrLen uint8) {
	copy(addr[:], smContext.PDUAddress)
	switch smContext.SelectedPDUSessionType {
	case nasMessage.PDUSessionTypeIPv4:
		addrLen = 4 + 1
	case nasMessage.PDUSessionTypeIPv6:
	case nasMessage.PDUSessionTypeIPv4IPv6:
		addrLen = 12 + 1
	}
	return
}

func (smContext *SMContext) GetNodeIDByLocalSEID(seid uint64) (nodeID net.IP) {
	for _, pfcpCtx := range smContext.PFCPContext {
		if pfcpCtx.LocalSEID == seid {
			nodeID = pfcpCtx.NodeID
		}
	}

	return
}

func (smContext *SMContext) AllocateLocalSEIDForDataPath(dataPath *DataPath) error {
	curDataPathNode := dataPath.DPNode
	NodeIDtoIP := curDataPathNode.UPF.NodeID.String()
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
	return nil
}

func (smContext *SMContext) PutPDRtoPFCPSession(nodeID net.IP, pdrList map[uint8]*PDR) error {
	NodeIDtoIP := nodeID.String()
	if pfcpSessCtx, exist := smContext.PFCPContext[NodeIDtoIP]; exist {
		for name, pdr := range pdrList {
			pfcpSessCtx.PDRs[pdrList[name].PDRID] = pdr
		}
	} else {
		return fmt.Errorf("error, can't find PFCPContext[%s] to put PDR(%v)", NodeIDtoIP, pdrList)
	}
	return nil
}

func (smContext *SMContext) RemovePDRfromPFCPSession(nodeID net.IP, pdr *PDR) {
	NodeIDtoIP := nodeID.String()
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
			Cause:                 cause,
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
	var out []*SMContext

	smContextPool.Range(func(_ any, v any) bool {
		sc, ok := v.(*SMContext)
		if !ok {
			return true
		}

		if sc.Supi == imsi {
			out = append(out, sc)
		}
		return true
	})
	return out
}

func PDUSessionsByDNN(dnn string) []*SMContext {
	var out []*SMContext

	smContextPool.Range(func(_ any, v any) bool {
		sc, ok := v.(*SMContext)
		if !ok {
			return true
		}

		if sc.Dnn == dnn {
			out = append(out, sc)
		}
		return true
	})
	return out
}
