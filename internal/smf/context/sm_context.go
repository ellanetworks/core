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

type SMContextState uint

const (
	SmStateInit SMContextState = iota
	SmStateActivePending
	SmStateActive
	SmStateInActivePending
	SmStateModify
	SmStatePfcpCreatePending
	SmStatePfcpModify
	SmStatePfcpRelease
	SmStateRelease
	SmStateN1N2TransferPending
	SmStateMax
)

type UeIpAddr struct {
	Ip          net.IP
	UpfProvided bool
}

type SMContext struct {
	Ref string `json:"ref" yaml:"ref" bson:"ref"`

	// SUPI or PEI
	Supi              string `json:"supi,omitempty" yaml:"supi" bson:"supi,omitempty"`
	Pei               string `json:"pei,omitempty" yaml:"pei" bson:"pei,omitempty"`
	Identifier        string `json:"identifier" yaml:"identifier" bson:"identifier"`
	Gpsi              string `json:"gpsi,omitempty" yaml:"gpsi" bson:"gpsi,omitempty"`
	Dnn               string `json:"dnn" yaml:"dnn" bson:"dnn"`
	UeTimeZone        string `json:"ueTimeZone,omitempty" yaml:"ueTimeZone" bson:"ueTimeZone,omitempty"` // ignore
	ServingNfId       string `json:"servingNfId,omitempty" yaml:"servingNfId" bson:"servingNfId,omitempty"`
	SmStatusNotifyUri string `json:"smStatusNotifyUri,omitempty" yaml:"smStatusNotifyUri" bson:"smStatusNotifyUri,omitempty"`

	UpCnxState models.UpCnxState `json:"upCnxState,omitempty" yaml:"upCnxState" bson:"upCnxState,omitempty"`
	// SelectedPCFProfile models.NfProfile        `json:"selectedPCFProfile,omitempty" yaml:"selectedPCFProfile" bson:"selectedPCFProfile,omitempty"`
	AnType           models.AccessType       `json:"anType" yaml:"anType" bson:"anType"`
	RatType          models.RatType          `json:"ratType,omitempty" yaml:"ratType" bson:"ratType,omitempty"`
	PresenceInLadn   models.PresenceState    `json:"presenceInLadn,omitempty" yaml:"presenceInLadn" bson:"presenceInLadn,omitempty"` // ignore
	HoState          models.HoState          `json:"hoState,omitempty" yaml:"hoState" bson:"hoState,omitempty"`
	DnnConfiguration models.DnnConfiguration `json:"dnnConfiguration,omitempty" yaml:"dnnConfiguration" bson:"dnnConfiguration,omitempty"` // ?

	Snssai         *models.Snssai       `json:"snssai" yaml:"snssai" bson:"snssai"`
	HplmnSnssai    *models.Snssai       `json:"hplmnSnssai,omitempty" yaml:"hplmnSnssai" bson:"hplmnSnssai,omitempty"`
	ServingNetwork *models.PlmnId       `json:"servingNetwork,omitempty" yaml:"servingNetwork" bson:"servingNetwork,omitempty"`
	UeLocation     *models.UserLocation `json:"ueLocation,omitempty" yaml:"ueLocation" bson:"ueLocation,omitempty"`
	AddUeLocation  *models.UserLocation `json:"addUeLocation,omitempty" yaml:"addUeLocation" bson:"addUeLocation,omitempty"` // ignore

	// PDUAddress             net.IP `json:"pduAddress,omitempty" yaml:"pduAddress" bson:"pduAddress,omitempty"`
	PDUAddress *UeIpAddr `json:"pduAddress,omitempty" yaml:"pduAddress" bson:"pduAddress,omitempty"`

	// Client
	// CommunicationClient *Namf_Communication.APIClient `json:"communicationClient,omitempty" yaml:"communicationClient" bson:"communicationClient,omitempty"` // ?

	// encountered a cycle via *context.GTPTunnel
	Tunnel *UPTunnel `json:"-" yaml:"tunnel" bson:"-"`

	BPManager *BPManager `json:"bpManager,omitempty" yaml:"bpManager" bson:"bpManager,omitempty"` // ignore

	DNNInfo *SnssaiSmfDnnInfo `json:"dnnInfo,omitempty" yaml:"dnnInfo" bson:"dnnInfo,omitempty"`

	// PCO Related
	ProtocolConfigurationOptions *ProtocolConfigurationOptions `json:"protocolConfigurationOptions" yaml:"protocolConfigurationOptions" bson:"protocolConfigurationOptions"` // ignore

	SubGsmLog      *zap.SugaredLogger `json:"-" yaml:"subGsmLog" bson:"-,"`     // ignore
	SubPfcpLog     *zap.SugaredLogger `json:"-" yaml:"subPfcpLog" bson:"-"`     // ignore
	SubPduSessLog  *zap.SugaredLogger `json:"-" yaml:"subPduSessLog" bson:"-"`  // ignore
	SubCtxLog      *zap.SugaredLogger `json:"-" yaml:"subCtxLog" bson:"-"`      // ignore
	SubConsumerLog *zap.SugaredLogger `json:"-" yaml:"subConsumerLog" bson:"-"` // ignore
	SubFsmLog      *zap.SugaredLogger `json:"-" yaml:"subFsmLog" bson:"-"`      // ignore
	SubQosLog      *zap.SugaredLogger `json:"-" yaml:"subQosLog" bson:"-"`      // ignore

	// encountered a cycle via *context.SMContext
	// SM Policy related
	// Updates in policy from PCF
	SmPolicyUpdates []*qos.PolicyUpdate `json:"smPolicyUpdates" yaml:"smPolicyUpdates" bson:"smPolicyUpdates"` // ignore
	// Holds Session/PCC Rules and Qos/Cond/Charging Data
	SmPolicyData qos.SmCtxtPolicyData `json:"smPolicyData" yaml:"smPolicyData" bson:"smPolicyData"`
	// unsupported structure - madatory!

	PendingUPF PendingUPF `json:"pendingUPF,omitempty" yaml:"pendingUPF" bson:"pendingUPF,omitempty"` // ignore
	// NodeID(string form) to PFCP Session Context
	PFCPContext map[string]*PFCPSessionContext `json:"-" yaml:"pfcpContext" bson:"-"`
	// lock
	// SMLock sync.Mutex `json:"smLock,omitempty" yaml:"smLock" bson:"smLock,omitempty"` // ignore
	SMLock sync.Mutex `json:"-" yaml:"smLock" bson:"-"` // ignore

	SMContextState                      SMContextState `json:"smContextState" yaml:"smContextState" bson:"smContextState"`
	PDUSessionID                        int32          `json:"pduSessionID" yaml:"pduSessionID" bson:"pduSessionID"`
	OldPduSessionId                     int32          `json:"oldPduSessionId,omitempty" yaml:"oldPduSessionId" bson:"oldPduSessionId,omitempty"`
	SelectedPDUSessionType              uint8          `json:"selectedPDUSessionType,omitempty" yaml:"selectedPDUSessionType" bson:"selectedPDUSessionType,omitempty"`
	UnauthenticatedSupi                 bool           `json:"unauthenticatedSupi,omitempty" yaml:"unauthenticatedSupi" bson:"unauthenticatedSupi,omitempty"`                                                 // ignore
	PDUSessionRelease_DUE_TO_DUP_PDU_ID bool           `json:"pduSessionRelease_DUE_TO_DUP_PDU_ID,omitempty" yaml:"pduSessionRelease_DUE_TO_DUP_PDU_ID" bson:"pduSessionRelease_DUE_TO_DUP_PDU_ID,omitempty"` // ignore
	LocalPurged                         bool           `json:"localPurged,omitempty" yaml:"localPurged" bson:"localPurged,omitempty"`                                                                         // ignore
	// NAS
	Pti                     uint8 `json:"pti,omitempty" yaml:"pti" bson:"pti,omitempty"` // ignore
	EstAcceptCause5gSMValue uint8 `json:"estAcceptCause5gSMValue,omitempty" yaml:"estAcceptCause5gSMValue" bson:"estAcceptCause5gSMValue,omitempty"`
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

	smContext.SMContextState = SmStateInit
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
	smContext.SubConsumerLog = logger.SmfLog.With("uuid", smContext.Ref, "id", smContext.Identifier, "pduid", smContext.PDUSessionID)
	smContext.SubFsmLog = logger.SmfLog.With("uuid", smContext.Ref, "id", smContext.Identifier, "pduid", smContext.PDUSessionID)
	smContext.SubQosLog = logger.SmfLog.With("uuid", smContext.Ref, "id", smContext.Identifier, "pduid", smContext.PDUSessionID)
}

func (smContext *SMContext) ChangeState(nextState SMContextState) {
	smContext.SMContextState = nextState
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

	smContext.ChangeState(SmStateRelease)

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
	smContext.HplmnSnssai = createData.HplmnSnssai
	smContext.ServingNetwork = createData.ServingNetwork
	smContext.AnType = createData.AnType
	smContext.RatType = createData.RatType
	smContext.PresenceInLadn = createData.PresenceInLadn
	smContext.UeLocation = createData.UeLocation
	smContext.UeTimeZone = createData.UeTimeZone
	smContext.AddUeLocation = createData.AddUeLocation
	smContext.OldPduSessionId = createData.OldPduSessionId
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
		case models.PduSessionType_IPV4:
			allowIPv4 = true
		case models.PduSessionType_IPV6:
			allowIPv6 = true
		case models.PduSessionType_IPV4_V6:
			allowIPv4 = true
			allowIPv6 = true
		case models.PduSessionType_ETHERNET:
			allowEthernet = true
		}
	}

	if !allowIPv4 {
		return fmt.Errorf("PduSessionType_IPV4 is not allowed in DNN[%s] configuration", smContext.Dnn)
	}

	smContext.EstAcceptCause5gSMValue = 0
	switch util.PDUSessionTypeToModels(requestedPDUSessionType) {
	case models.PduSessionType_IPV4:
		if allowIPv4 {
			smContext.SelectedPDUSessionType = util.ModelsToPDUSessionType(models.PduSessionType_IPV4)
		} else {
			return fmt.Errorf("PduSessionType_IPV4 is not allowed in DNN[%s] configuration", smContext.Dnn)
		}
	case models.PduSessionType_IPV6:
		if allowIPv6 {
			smContext.SelectedPDUSessionType = util.ModelsToPDUSessionType(models.PduSessionType_IPV6)
		} else {
			return fmt.Errorf("PduSessionType_IPV6 is not allowed in DNN[%s] configuration", smContext.Dnn)
		}
	case models.PduSessionType_IPV4_V6:
		if allowIPv4 && allowIPv6 {
			smContext.SelectedPDUSessionType = util.ModelsToPDUSessionType(models.PduSessionType_IPV4_V6)
		} else if allowIPv4 {
			smContext.SelectedPDUSessionType = util.ModelsToPDUSessionType(models.PduSessionType_IPV4)
			smContext.EstAcceptCause5gSMValue = nasMessage.Cause5GSMPDUSessionTypeIPv4OnlyAllowed
		} else if allowIPv6 {
			smContext.SelectedPDUSessionType = util.ModelsToPDUSessionType(models.PduSessionType_IPV6)
			smContext.EstAcceptCause5gSMValue = nasMessage.Cause5GSMPDUSessionTypeIPv6OnlyAllowed
		} else {
			return fmt.Errorf("PduSessionType_IPV4_V6 is not allowed in DNN[%s] configuration", smContext.Dnn)
		}
	case models.PduSessionType_ETHERNET:
		if allowEthernet {
			smContext.SelectedPDUSessionType = util.ModelsToPDUSessionType(models.PduSessionType_ETHERNET)
		} else {
			return fmt.Errorf("PduSessionType_ETHERNET is not allowed in DNN[%s] configuration", smContext.Dnn)
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

func (smContextState SMContextState) String() string {
	switch smContextState {
	case SmStateInit:
		return "SmStateInit"
	case SmStateActivePending:
		return "SmStateActivePending"
	case SmStateActive:
		return "SmStateActive"
	case SmStateInActivePending:
		return "SmStateInActivePending"
	case SmStateModify:
		return "SmStateModify"
	case SmStatePfcpCreatePending:
		return "SmStatePfcpCreatePending"
	case SmStatePfcpModify:
		return "SmStatePfcpModify"
	case SmStatePfcpRelease:
		return "SmStatePfcpRelease"
	case SmStateN1N2TransferPending:
		return "SmStateN1N2TransferPending"

	default:
		return "Unknown State"
	}
}

func (smContext *SMContext) GeneratePDUSessionEstablishmentReject(status int, problemDetails *models.ProblemDetails, cause uint8) *util.Response {
	var httpResponse *util.Response

	if buf, err := BuildGSMPDUSessionEstablishmentReject(smContext, cause); err != nil {
		httpResponse = &util.Response{
			Header: nil,
			Status: status,
			Body:   models.PostSmContextsErrorResponse{},
		}
	} else {
		httpResponse = &util.Response{
			Header: nil,
			Status: status,
			Body: models.PostSmContextsErrorResponse{
				BinaryDataN1SmMessage: buf,
			},
		}
	}

	return httpResponse
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
