// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/omec-project/nas/nasConvert"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/openapi/Namf_Communication"
	"github.com/omec-project/openapi/Nnrf_NFDiscovery"
	"github.com/omec-project/openapi/Npcf_SMPolicyControl"
	"github.com/omec-project/openapi/models"
	nrfCache "github.com/omec-project/openapi/nrfcache"
	"github.com/omec-project/util/httpwrapper"
	"github.com/sirupsen/logrus"
	"github.com/yeastengine/ella/internal/smf/logger"
	"github.com/yeastengine/ella/internal/smf/qos"
	errors "github.com/yeastengine/ella/internal/smf/smferrors"
	"github.com/yeastengine/ella/internal/smf/transaction"
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

var (
	smContextCount  uint64
	smContextActive uint64
)

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

func init() {
}

func incSMContextActive() uint64 {
	atomic.AddUint64(&smContextActive, 1)
	return smContextActive
}

func decSMContextActive() uint64 {
	atomic.AddUint64(&smContextActive, ^uint64(0))
	return smContextActive
}

func GetSMContextCount() uint64 {
	atomic.AddUint64(&smContextCount, 1)
	return smContextCount
}

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

	UpCnxState         models.UpCnxState       `json:"upCnxState,omitempty" yaml:"upCnxState" bson:"upCnxState,omitempty"`
	AMFProfile         models.NfProfile        `json:"amfProfile,omitempty" yaml:"amfProfile" bson:"amfProfile,omitempty"`
	SelectedPCFProfile models.NfProfile        `json:"selectedPCFProfile,omitempty" yaml:"selectedPCFProfile" bson:"selectedPCFProfile,omitempty"`
	AnType             models.AccessType       `json:"anType" yaml:"anType" bson:"anType"`
	RatType            models.RatType          `json:"ratType,omitempty" yaml:"ratType" bson:"ratType,omitempty"`
	PresenceInLadn     models.PresenceState    `json:"presenceInLadn,omitempty" yaml:"presenceInLadn" bson:"presenceInLadn,omitempty"` // ignore
	HoState            models.HoState          `json:"hoState,omitempty" yaml:"hoState" bson:"hoState,omitempty"`
	DnnConfiguration   models.DnnConfiguration `json:"dnnConfiguration,omitempty" yaml:"dnnConfiguration" bson:"dnnConfiguration,omitempty"` // ?

	Snssai         *models.Snssai       `json:"snssai" yaml:"snssai" bson:"snssai"`
	HplmnSnssai    *models.Snssai       `json:"hplmnSnssai,omitempty" yaml:"hplmnSnssai" bson:"hplmnSnssai,omitempty"`
	ServingNetwork *models.PlmnId       `json:"servingNetwork,omitempty" yaml:"servingNetwork" bson:"servingNetwork,omitempty"`
	UeLocation     *models.UserLocation `json:"ueLocation,omitempty" yaml:"ueLocation" bson:"ueLocation,omitempty"`
	AddUeLocation  *models.UserLocation `json:"addUeLocation,omitempty" yaml:"addUeLocation" bson:"addUeLocation,omitempty"` // ignore

	// PDUAddress             net.IP `json:"pduAddress,omitempty" yaml:"pduAddress" bson:"pduAddress,omitempty"`
	PDUAddress *UeIpAddr `json:"pduAddress,omitempty" yaml:"pduAddress" bson:"pduAddress,omitempty"`

	// Client
	SMPolicyClient      *Npcf_SMPolicyControl.APIClient `json:"smPolicyClient,omitempty" yaml:"smPolicyClient" bson:"smPolicyClient,omitempty"`                // ?
	CommunicationClient *Namf_Communication.APIClient   `json:"communicationClient,omitempty" yaml:"communicationClient" bson:"communicationClient,omitempty"` // ?

	// encountered a cycle via *context.GTPTunnel
	Tunnel *UPTunnel `json:"-" yaml:"tunnel" bson:"-"`

	BPManager *BPManager `json:"bpManager,omitempty" yaml:"bpManager" bson:"bpManager,omitempty"` // ignore

	DNNInfo *SnssaiSmfDnnInfo `json:"dnnInfo,omitempty" yaml:"dnnInfo" bson:"dnnInfo,omitempty"`

	// PCO Related
	ProtocolConfigurationOptions *ProtocolConfigurationOptions `json:"protocolConfigurationOptions" yaml:"protocolConfigurationOptions" bson:"protocolConfigurationOptions"` // ignore

	SubGsmLog      *logrus.Entry `json:"-" yaml:"subGsmLog" bson:"-,"`     // ignore
	SubPfcpLog     *logrus.Entry `json:"-" yaml:"subPfcpLog" bson:"-"`     // ignore
	SubPduSessLog  *logrus.Entry `json:"-" yaml:"subPduSessLog" bson:"-"`  // ignore
	SubCtxLog      *logrus.Entry `json:"-" yaml:"subCtxLog" bson:"-"`      // ignore
	SubConsumerLog *logrus.Entry `json:"-" yaml:"subConsumerLog" bson:"-"` // ignore
	SubFsmLog      *logrus.Entry `json:"-" yaml:"subFsmLog" bson:"-"`      // ignore
	SubQosLog      *logrus.Entry `json:"-" yaml:"subQosLog" bson:"-"`      // ignore

	// encountered a cycle via *context.SMContext
	ActiveTxn *transaction.Transaction `json:"-" yaml:"activeTxn" bson:"-,"` // ignore
	// SM Policy related
	// Updates in policy from PCF
	SmPolicyUpdates []*qos.PolicyUpdate `json:"smPolicyUpdates" yaml:"smPolicyUpdates" bson:"smPolicyUpdates"` // ignore
	// Holds Session/PCC Rules and Qos/Cond/Charging Data
	SmPolicyData qos.SmCtxtPolicyData `json:"smPolicyData" yaml:"smPolicyData" bson:"smPolicyData"`
	// unsupported structure - madatory!
	SBIPFCPCommunicationChan chan PFCPSessionResponseStatus `json:"-" yaml:"sbiPFCPCommunicationChan" bson:"-"` // ignore

	PendingUPF PendingUPF `json:"pendingUPF,omitempty" yaml:"pendingUPF" bson:"pendingUPF,omitempty"` // ignore
	// NodeID(string form) to PFCP Session Context
	PFCPContext map[string]*PFCPSessionContext `json:"-" yaml:"pfcpContext" bson:"-"`
	// TxnBus per subscriber
	TxnBus transaction.TxnBus `json:"-" yaml:"txnBus" bson:"-"` // ignore
	// SMTxnBusLock sync.Mutex         `json:"smTxnBusLock,omitempty" yaml:"smTxnBusLock" bson:"smTxnBusLock,omitempty"` // ignore
	SMTxnBusLock sync.Mutex `json:"-" yaml:"smTxnBusLock" bson:"-"` // ignore
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
	smContext.SBIPFCPCommunicationChan = make(chan PFCPSessionResponseStatus, 1)
	smContext.SmPolicyUpdates = make([]*qos.PolicyUpdate, 0)
	smContext.SmPolicyData.Initialize()

	smContext.ProtocolConfigurationOptions = &ProtocolConfigurationOptions{
		DNSIPv4Request: false,
		DNSIPv6Request: false,
	}

	// initialise log tags
	smContext.initLogTags()

	return smContext
}

func (smContext *SMContext) initLogTags() {
	subField := logrus.Fields{
		"uuid": smContext.Ref,
		"id":   smContext.Identifier, "pduid": smContext.PDUSessionID,
	}

	smContext.SubPfcpLog = logger.PfcpLog.WithFields(subField)
	smContext.SubCtxLog = logger.CtxLog.WithFields(subField)
	smContext.SubPduSessLog = logger.PduSessLog.WithFields(subField)
	smContext.SubGsmLog = logger.GsmLog.WithFields(subField)
	smContext.SubConsumerLog = logger.ConsumerLog.WithFields(subField)
	smContext.SubFsmLog = logger.FsmLog.WithFields(subField)
	smContext.SubQosLog = logger.QosLog.WithFields(subField)
}

func (smContext *SMContext) ChangeState(nextState SMContextState) {
	// Update Subscriber profile Metrics
	if nextState == SmStateActive || smContext.SMContextState == SmStateActive {
		var upf string
		if smContext.Tunnel != nil {
			// Set UPF FQDN name if provided else IP-address
			if smContext.Tunnel.DataPathPool[1].FirstDPNode.UPF.NodeID.NodeIdType == NodeIdTypeFqdn {
				upf = string(smContext.Tunnel.DataPathPool[1].FirstDPNode.UPF.NodeID.NodeIdValue)
				upf = strings.Split(upf, ".")[0]
			} else {
				upf = smContext.Tunnel.DataPathPool[1].FirstDPNode.UPF.GetUPFIP()
			}
		}

		// enterprise name
		if smfContext.EnterpriseList != nil {
			entMap := *smfContext.EnterpriseList
			smContext.SubCtxLog.Debugf("context state change, Enterprises configured = [%v], subscriber slice sst [%v], sd [%v]",
				entMap, smContext.Snssai.Sst, smContext.Snssai.Sd)
		} else {
			smContext.SubCtxLog.Debug("context state change, enterprise info not available")
		}

	}

	smContext.SubCtxLog.Infof("context state change, current state[%v] next state[%v]",
		smContext.SMContextState.String(), nextState.String())
	smContext.SMContextState = nextState
}

// *** add unit test ***//
func GetSMContext(ref string) (smContext *SMContext) {
	if value, ok := smContextPool.Load(ref); ok {
		smContext = value.(*SMContext)
	}

	return
}

// *** add unit test ***//
func RemoveSMContext(ref string) {
	var smContext *SMContext
	if value, ok := smContextPool.Load(ref); ok {
		smContext = value.(*SMContext)
	}

	smContext.SubCtxLog.Infof("RemoveSMContext, SM context released ")
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
}

// *** add unit test ***//
func GetSMContextBySEID(SEID uint64) (smContext *SMContext) {
	if value, ok := seidSMContextMap.Load(SEID); ok {
		smContext = value.(*SMContext)
	}
	return
}

func (smContext *SMContext) ReleaseUeIpAddr() error {
	if ip := smContext.PDUAddress.Ip; ip != nil && !smContext.PDUAddress.UpfProvided {
		smContext.SubPduSessLog.Infof("Release IP[%s]", smContext.PDUAddress.Ip.String())
		smContext.DNNInfo.UeIPAllocator.Release(smContext.Supi, ip)
		smContext.PDUAddress.Ip = net.IPv4(0, 0, 0, 0)
	}
	return nil
}

// *** add unit test ***//
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

// PCFSelection will select PCF for this SM Context
func (smContext *SMContext) PCFSelection() error {
	// Send NFDiscovery for find PCF
	localVarOptionals := Nnrf_NFDiscovery.SearchNFInstancesParamOpts{}

	var rep models.SearchResult
	var res *http.Response
	var err error

	if SMF_Self().EnableNrfCaching {
		rep, err = nrfCache.SearchNFInstances(SMF_Self().NrfUri, models.NfType_PCF, models.NfType_SMF, &localVarOptionals)
		if err != nil {
			return err
		}
	} else {
		rep, res, err = SMF_Self().
			NFDiscoveryClient.
			NFInstancesStoreApi.
			SearchNFInstances(context.TODO(), models.NfType_PCF, models.NfType_SMF, &localVarOptionals)
		if err != nil {
			return err
		}
		defer func() {
			if rspCloseErr := res.Body.Close(); rspCloseErr != nil {
				logger.PduSessLog.Errorf("SmfEventExposureNotification response body cannot close: %+v", rspCloseErr)
			}
		}()

		if res != nil {
			if status := res.StatusCode; status != http.StatusOK {
				logger.CtxLog.Warningf("NFDiscovery PCF return status: %d\n", status)
			}
		}

	}

	smContext.SelectedPCFProfile = rep.NfInstances[0]

	// Create SMPolicyControl Client for this SM Context
	for _, service := range *smContext.SelectedPCFProfile.NfServices {
		if service.ServiceName == models.ServiceName_NPCF_SMPOLICYCONTROL {
			SmPolicyControlConf := Npcf_SMPolicyControl.NewConfiguration()
			SmPolicyControlConf.SetBasePath(service.ApiPrefix)
			smContext.SMPolicyClient = Npcf_SMPolicyControl.NewAPIClient(SmPolicyControlConf)
		}
	}

	return nil
}

func (smContext *SMContext) GetNodeIDByLocalSEID(seid uint64) (nodeID NodeID) {
	for _, pfcpCtx := range smContext.PFCPContext {
		if pfcpCtx.LocalSEID == seid {
			nodeID = pfcpCtx.NodeID
		}
	}

	return
}

func (smContext *SMContext) AllocateLocalSEIDForDataPath(dataPath *DataPath) {
	logger.PduSessLog.Traceln("In AllocateLocalSEIDForDataPath")
	for curDataPathNode := dataPath.FirstDPNode; curDataPathNode != nil; curDataPathNode = curDataPathNode.Next() {
		NodeIDtoIP := curDataPathNode.UPF.NodeID.ResolveNodeIdToIp().String()
		logger.PduSessLog.Traceln("NodeIDtoIP: ", NodeIDtoIP)
		if _, exist := smContext.PFCPContext[NodeIDtoIP]; !exist {
			allocatedSEID, err := AllocateLocalSEID()
			if err != nil {
				logger.PduSessLog.Errorf("allocateLocalSEID failed, %v", err)
			}
			smContext.PFCPContext[NodeIDtoIP] = &PFCPSessionContext{
				PDRs:      make(map[uint16]*PDR),
				NodeID:    curDataPathNode.UPF.NodeID,
				LocalSEID: allocatedSEID,
			}

			seidSMContextMap.Store(allocatedSEID, smContext)

		}
	}
}

func (smContext *SMContext) PutPDRtoPFCPSession(nodeID NodeID, pdrList map[string]*PDR) error {
	// TODO: Iterate over PDRS
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

	supportedPDUSessionType := SMF_Self().SupportedPDUSessionType
	switch supportedPDUSessionType {
	case "IPv4":
		if !allowIPv4 {
			return fmt.Errorf("no SupportedPDUSessionType[%q] in DNN[%s] configuration", supportedPDUSessionType, smContext.Dnn)
		}
	case "IPv6":
		if !allowIPv6 {
			return fmt.Errorf("no SupportedPDUSessionType[%q] in DNN[%s] configuration", supportedPDUSessionType, smContext.Dnn)
		}
	case "IPv4v6":
		if !allowIPv4 && !allowIPv6 {
			return fmt.Errorf("no SupportedPDUSessionType[%q] in DNN[%s] configuration", supportedPDUSessionType, smContext.Dnn)
		}
	case "Ethernet":
		if !allowEthernet {
			return fmt.Errorf("no SupportedPDUSessionType[%q] in DNN[%s] configuration", supportedPDUSessionType, smContext.Dnn)
		}
	}

	smContext.EstAcceptCause5gSMValue = 0
	switch nasConvert.PDUSessionTypeToModels(requestedPDUSessionType) {
	case models.PduSessionType_IPV4:
		if allowIPv4 {
			smContext.SelectedPDUSessionType = nasConvert.ModelsToPDUSessionType(models.PduSessionType_IPV4)
		} else {
			return fmt.Errorf("PduSessionType_IPV4 is not allowed in DNN[%s] configuration", smContext.Dnn)
		}
	case models.PduSessionType_IPV6:
		if allowIPv6 {
			smContext.SelectedPDUSessionType = nasConvert.ModelsToPDUSessionType(models.PduSessionType_IPV6)
		} else {
			return fmt.Errorf("PduSessionType_IPV6 is not allowed in DNN[%s] configuration", smContext.Dnn)
		}
	case models.PduSessionType_IPV4_V6:
		if allowIPv4 && allowIPv6 {
			smContext.SelectedPDUSessionType = nasConvert.ModelsToPDUSessionType(models.PduSessionType_IPV4_V6)
		} else if allowIPv4 {
			smContext.SelectedPDUSessionType = nasConvert.ModelsToPDUSessionType(models.PduSessionType_IPV4)
			smContext.EstAcceptCause5gSMValue = nasMessage.Cause5GSMPDUSessionTypeIPv4OnlyAllowed
		} else if allowIPv6 {
			smContext.SelectedPDUSessionType = nasConvert.ModelsToPDUSessionType(models.PduSessionType_IPV6)
			smContext.EstAcceptCause5gSMValue = nasMessage.Cause5GSMPDUSessionTypeIPv6OnlyAllowed
		} else {
			return fmt.Errorf("PduSessionType_IPV4_V6 is not allowed in DNN[%s] configuration", smContext.Dnn)
		}
	case models.PduSessionType_ETHERNET:
		if allowEthernet {
			smContext.SelectedPDUSessionType = nasConvert.ModelsToPDUSessionType(models.PduSessionType_ETHERNET)
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

func (smContext *SMContext) GeneratePDUSessionEstablishmentReject(cause string) *httpwrapper.Response {
	var httpResponse *httpwrapper.Response

	if buf, err := BuildGSMPDUSessionEstablishmentReject(
		smContext,
		errors.ErrorCause[cause]); err != nil {
		httpResponse = &httpwrapper.Response{
			Header: nil,
			Status: int(errors.ErrorType[cause].Status),
			Body: models.PostSmContextsErrorResponse{
				JsonData: &models.SmContextCreateError{
					Error:   errors.ErrorType[cause],
					N1SmMsg: &models.RefToBinaryData{ContentId: "n1SmMsg"},
				},
			},
		}
	} else {
		httpResponse = &httpwrapper.Response{
			Header: nil,
			Status: int(errors.ErrorType[cause].Status),
			Body: models.PostSmContextsErrorResponse{
				JsonData: &models.SmContextCreateError{
					Error:   errors.ErrorType[cause],
					N1SmMsg: &models.RefToBinaryData{ContentId: "n1SmMsg"},
				},
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
			logger.CtxLog.Errorf("failed to commit SM Policy Decision, %v", err)
		}
	}

	// Release 0th index update
	if len(smContext.SmPolicyUpdates) >= 1 {
		smContext.SmPolicyUpdates = smContext.SmPolicyUpdates[1:]
	}

	// Notify PCF of failure ?
	// TODO
	return nil
}
