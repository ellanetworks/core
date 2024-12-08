package message

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/openapi/models"
	"github.com/wmnsk/go-pfcp/message"
	"github.com/yeastengine/ella/internal/amf/producer"
	smf_context "github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/smf/logger"
	"github.com/yeastengine/ella/internal/smf/pfcp/udp"
)

var seq uint32

func getSeqNumber() uint32 {
	return atomic.AddUint32(&seq, 1)
}

func init() {
	PfcpTxns = make(map[uint32]*smf_context.NodeID)
}

var (
	PfcpTxns    map[uint32]*smf_context.NodeID
	PfcpTxnLock sync.Mutex
)

func FetchPfcpTxn(seqNo uint32) (upNodeID *smf_context.NodeID) {
	PfcpTxnLock.Lock()
	defer PfcpTxnLock.Unlock()
	if upNodeID = PfcpTxns[seqNo]; upNodeID != nil {
		delete(PfcpTxns, seqNo)
	}
	return upNodeID
}

func InsertPfcpTxn(seqNo uint32, upNodeID *smf_context.NodeID) {
	PfcpTxnLock.Lock()
	defer PfcpTxnLock.Unlock()
	PfcpTxns[seqNo] = upNodeID
}

func SendHeartbeatRequest(upNodeID smf_context.NodeID, upfPort uint16) error {
	msg := BuildPfcpHeartbeatRequest(getSeqNumber(), udp.ServerStartTime)
	addr := &net.UDPAddr{
		IP:   upNodeID.ResolveNodeIdToIp(),
		Port: int(upfPort),
	}
	InsertPfcpTxn(msg.Sequence(), &upNodeID)
	if err := udp.SendPfcp(msg, addr, nil); err != nil {
		FetchPfcpTxn(msg.Sequence())
		return err
	}
	logger.PfcpLog.Infof("sent pfcp heartbeat request to UPF: [%s] (sequence: %d )", upNodeID.ResolveNodeIdToIp().String(), msg.Sequence())
	return nil
}

func SendPfcpAssociationSetupRequest(upNodeID smf_context.NodeID, upfPort uint16) error {
	pfcpMsg := BuildPfcpAssociationSetupRequest(getSeqNumber(), udp.ServerStartTime, smf_context.SMF_Self().CPNodeID.ResolveNodeIdToIp().String())
	addr := &net.UDPAddr{
		IP:   upNodeID.ResolveNodeIdToIp(),
		Port: int(upfPort),
	}
	InsertPfcpTxn(pfcpMsg.Sequence(), &upNodeID)
	err := udp.SendPfcp(pfcpMsg, addr, nil)
	if err != nil {
		return err
	}
	logger.PfcpLog.Infof("sent PFCP Association Request to UPF: %s", addr.String())
	return nil
}

func SendPfcpAssociationSetupResponse(upNodeID smf_context.NodeID, cause uint8, upfPort uint16) error {
	pfcpMsg := BuildPfcpAssociationSetupResponse(cause, udp.ServerStartTime, smf_context.SMF_Self().CPNodeID.ResolveNodeIdToIp().String())
	addr := &net.UDPAddr{
		IP:   upNodeID.ResolveNodeIdToIp(),
		Port: int(upfPort),
	}
	err := udp.SendPfcp(pfcpMsg, addr, nil)
	if err != nil {
		return err
	}
	logger.PfcpLog.Infof("sent PFCP Association Response to NodeID[%s]", upNodeID.ResolveNodeIdToIp().String())
	return nil
}

func SendPfcpAssociationReleaseResponse(upNodeID smf_context.NodeID, cause uint8, upfPort uint16) error {
	pfcpMsg := BuildPfcpAssociationReleaseResponse(cause, smf_context.SMF_Self().CPNodeID.ResolveNodeIdToIp().String())
	addr := &net.UDPAddr{
		IP:   upNodeID.ResolveNodeIdToIp(),
		Port: int(upfPort),
	}
	err := udp.SendPfcp(pfcpMsg, addr, nil)
	if err != nil {
		return err
	}
	logger.PfcpLog.Infof("sent PFCP Association Release Response to NodeID[%s]", upNodeID.ResolveNodeIdToIp().String())
	return nil
}

func SendPfcpSessionEstablishmentRequest(
	upNodeID smf_context.NodeID,
	ctx *smf_context.SMContext,
	pdrList []*smf_context.PDR,
	farList []*smf_context.FAR,
	barList []*smf_context.BAR,
	qerList []*smf_context.QER,
	upfPort uint16,
) error {
	upNodeIDStr := upNodeID.ResolveNodeIdToIp().String()
	pfcpContext, ok := ctx.PFCPContext[upNodeIDStr]
	if !ok {
		return fmt.Errorf("PFCP Context not found for NodeID[%v]", upNodeID)
	}

	nodeIDIPAddress := smf_context.SMF_Self().CPNodeID.ResolveNodeIdToIp()

	pfcpMsg, err := BuildPfcpSessionEstablishmentRequest(
		getSeqNumber(),
		nodeIDIPAddress.String(),
		nodeIDIPAddress,
		pfcpContext.LocalSEID,
		pdrList,
		farList,
		qerList,
	)
	if err != nil {
		return err
	}
	logger.PfcpLog.Debugf("in SendPfcpSessionEstablishmentRequest pfcpMsg.CPFSEID.Seid %v\n", pfcpMsg.SEID())
	ip := upNodeID.ResolveNodeIdToIp()

	upaddr := &net.UDPAddr{
		IP:   ip,
		Port: int(upfPort),
	}
	ctx.SubPduSessLog.Debugln("[SMF] Send SendPfcpSessionEstablishmentRequest")
	ctx.SubPduSessLog.Debugln("Send to addr ", upaddr.String())
	logger.PfcpLog.Infof("in SendPfcpSessionEstablishmentRequest fseid %v\n", pfcpMsg.SEID())

	InsertPfcpTxn(pfcpMsg.Sequence(), &upNodeID)
	eventData := udp.PfcpEventData{LSEID: ctx.PFCPContext[ip.String()].LocalSEID, ErrHandler: HandlePfcpSendError}
	err = udp.SendPfcp(pfcpMsg, upaddr, eventData)
	if err != nil {
		return err
	}
	ctx.SubPfcpLog.Infof("Sent PFCP Session Establish Request to NodeID[%s]", ip.String())
	return nil
}

func SendPfcpSessionModificationRequest(
	upNodeID smf_context.NodeID,
	ctx *smf_context.SMContext,
	pdrList []*smf_context.PDR,
	farList []*smf_context.FAR,
	barList []*smf_context.BAR,
	qerList []*smf_context.QER,
	upfPort uint16,
) error {
	seqNum := getSeqNumber()
	upNodeIDStr := upNodeID.ResolveNodeIdToIp().String()
	pfcpContext, ok := ctx.PFCPContext[upNodeIDStr]
	if !ok {
		return fmt.Errorf("PFCP Context not found for NodeID[%s]", upNodeIDStr)
	}
	pfcpMsg, err := BuildPfcpSessionModificationRequest(seqNum, pfcpContext.LocalSEID, pfcpContext.RemoteSEID, smf_context.SMF_Self().CPNodeID.ResolveNodeIdToIp(), pdrList, farList, qerList)
	if err != nil {
		return err
	}
	nodeIDtoIP := upNodeID.ResolveNodeIdToIp().String()
	upaddr := &net.UDPAddr{
		IP:   upNodeID.ResolveNodeIdToIp(),
		Port: int(upfPort),
	}

	InsertPfcpTxn(pfcpMsg.Sequence(), &upNodeID)
	eventData := udp.PfcpEventData{LSEID: ctx.PFCPContext[nodeIDtoIP].LocalSEID, ErrHandler: HandlePfcpSendError}
	err = udp.SendPfcp(pfcpMsg, upaddr, eventData)
	if err != nil {
		logger.PfcpLog.Errorf("send pfcp session modify msg to upf error [%v] ", err.Error())
	}
	ctx.SubPfcpLog.Infof("Sent PFCP Session Modify Request to NodeID[%s]", upNodeID.ResolveNodeIdToIp().String())
	return nil
}

func SendPfcpSessionDeletionRequest(upNodeID smf_context.NodeID, ctx *smf_context.SMContext, upfPort uint16) error {
	seqNum := getSeqNumber()
	upNodeIDStr := upNodeID.ResolveNodeIdToIp().String()
	pfcpContext, ok := ctx.PFCPContext[upNodeIDStr]
	if !ok {
		return fmt.Errorf("PFCP Context not found for NodeID[%s]", upNodeIDStr)
	}
	pfcpMsg := BuildPfcpSessionDeletionRequest(seqNum, pfcpContext.LocalSEID, pfcpContext.RemoteSEID, smf_context.SMF_Self().CPNodeID.ResolveNodeIdToIp())

	upaddr := &net.UDPAddr{
		IP:   upNodeID.ResolveNodeIdToIp(),
		Port: int(upfPort),
	}

	InsertPfcpTxn(pfcpMsg.Sequence(), &upNodeID)
	eventData := udp.PfcpEventData{LSEID: pfcpContext.LocalSEID, ErrHandler: HandlePfcpSendError}
	err := udp.SendPfcp(pfcpMsg, upaddr, eventData)
	if err != nil {
		return err
	}

	ctx.SubPfcpLog.Infof("Sent PFCP Session Delete Request to NodeID[%s]", upNodeID.ResolveNodeIdToIp().String())
	return nil
}

func SendPfcpSessionReportResponse(addr *net.UDPAddr, cause uint8, pfcpSRflag smf_context.PFCPSRRspFlags, seqFromUPF uint32, SEID uint64) error {
	pfcpMsg := BuildPfcpSessionReportResponse(cause, pfcpSRflag.Drobu, seqFromUPF, SEID)
	err := udp.SendPfcp(pfcpMsg, addr, nil)
	if err != nil {
		return err
	}
	logger.PfcpLog.Infof("Sent PFCP Session Report Response Seq[%d] to NodeID[%s]", seqFromUPF, addr.IP.String())
	return nil
}

func SendHeartbeatResponse(addr *net.UDPAddr, sequenceNumber uint32) error {
	pfcpMsg := BuildPfcpHeartbeatResponse(sequenceNumber, udp.ServerStartTime)
	err := udp.SendPfcp(pfcpMsg, addr, nil)
	if err != nil {
		return err
	}
	logger.PfcpLog.Infof("Sent PFCP Heartbeat Response Seq[%d] to NodeID[%s]", sequenceNumber, addr.IP.String())
	return nil
}

func HandlePfcpSendError(msg message.Message, pfcpErr error) {
	logger.PfcpLog.Errorf("send of PFCP msg [%v] failed, %v",
		msg.MessageTypeName(), pfcpErr.Error())

	switch msg.MessageType() {
	case message.MsgTypeSessionEstablishmentRequest:
		handleSendPfcpSessEstReqError(msg, pfcpErr)
	case message.MsgTypeSessionModificationRequest:
		handleSendPfcpSessModReqError(msg, pfcpErr)
	case message.MsgTypeSessionDeletionRequest:
		handleSendPfcpSessRelReqError(msg, pfcpErr)
	default:
		logger.PfcpLog.Errorf("Unable to send PFCP packet type [%v] and content [%v]",
			msg.MessageTypeName(), msg)
	}
}

func handleSendPfcpSessEstReqError(msg message.Message, pfcpErr error) {
	// Lets decode the PDU request
	pfcpEstReq, ok := msg.(*message.SessionEstablishmentRequest)
	if !ok {
		logger.PfcpLog.Errorf("Unable to decode PFCP Session Establishment Request")
		return
	}

	SEID := pfcpEstReq.SEID()
	smContext := smf_context.GetSMContextBySEID(SEID)
	if smContext == nil {
		logger.PfcpLog.Errorf("SMContext not found for SEID[%v]", SEID)
		return
	}
	smContext.SubPfcpLog.Errorf("PFCP Session Establishment send failure, %v", pfcpErr.Error())
	// N1N2 Request towards AMF
	n1n2Request := models.N1N2MessageTransferRequest{}

	// N1 Container Info
	n1MsgContainer := models.N1MessageContainer{
		N1MessageClass:   "SM",
		N1MessageContent: &models.RefToBinaryData{ContentId: "GSM_NAS"},
	}

	// N1N2 Json Data
	n1n2Request.JsonData = &models.N1N2MessageTransferReqData{PduSessionId: smContext.PDUSessionID}

	if smNasBuf, err := smf_context.BuildGSMPDUSessionEstablishmentReject(smContext,
		nasMessage.Cause5GSMRequestRejectedUnspecified); err != nil {
		smContext.SubPduSessLog.Errorf("Build GSM PDUSessionEstablishmentReject failed: %s", err)
	} else {
		n1n2Request.BinaryDataN1Message = smNasBuf
		n1n2Request.JsonData.N1MessageContainer = &n1MsgContainer
	}

	// Send N1N2 Reject request
	// communicationClient := Namf_Communication.NewAPIClient(communicationConf)
	rspData, err := producer.CreateN1N2MessageTransfer(smContext.Supi, n1n2Request, "")
	// rspData, _, err := communicationClient.
	// 	N1N2MessageCollectionDocumentApi.
	// 	N1N2MessageTransfer(context.Background(), smContext.Supi, n1n2Request)
	smContext.ChangeState(smf_context.SmStateInit)
	smContext.SubCtxLog.Debugln("SMContextState Change State: ", smContext.SMContextState.String())
	if err != nil {
		smContext.SubPfcpLog.Warnf("Send N1N2Transfer failed")
	}
	if rspData.Cause == models.N1N2MessageTransferCause_N1_MSG_NOT_TRANSFERRED {
		smContext.SubPfcpLog.Warnf("%v", rspData.Cause)
	}
	smContext.SubPfcpLog.Errorf("PFCP send N1N2Transfer Reject initiated for id[%v], pduSessId[%v]", smContext.Identifier, smContext.PDUSessionID)

	// clear subscriber
	smf_context.RemoveSMContext(smContext.Ref)
}

func handleSendPfcpSessRelReqError(msg message.Message, pfcpErr error) {
	// Lets decode the PDU request
	pfcpRelReq, ok := msg.(*message.SessionDeletionRequest)
	if !ok {
		logger.PfcpLog.Errorf("Unable to decode PFCP Session Deletion Request")
		return
	}

	SEID := pfcpRelReq.SEID()
	smContext := smf_context.GetSMContextBySEID(SEID)
	if smContext != nil {
		smContext.SubPfcpLog.Errorf("PFCP Session Delete send failure, %v", pfcpErr.Error())
		// Always send success
		smContext.SBIPFCPCommunicationChan <- smf_context.SessionReleaseSuccess
	}
}

func handleSendPfcpSessModReqError(msg message.Message, pfcpErr error) {
	// Lets decode the PDU request
	pfcpModReq, ok := msg.(*message.SessionModificationRequest)
	if !ok {
		logger.PfcpLog.Errorf("Unable to decode PFCP Session Modification Request")
		return
	}

	SEID := pfcpModReq.SEID()
	smContext := smf_context.GetSMContextBySEID(SEID)
	if smContext == nil {
		logger.PfcpLog.Errorf("SMContext not found for SEID[%v]", SEID)
		return
	}
	smContext.SubPfcpLog.Errorf("PFCP Session Modification send failure, %v", pfcpErr.Error())

	smContext.SBIPFCPCommunicationChan <- smf_context.SessionUpdateTimeout
}

type adapterMessage struct {
	Body []byte `json:"body"`
}

type UdpPodPfcpMsg struct {
	// message type contains in Msg.Header
	Msg      adapterMessage     `json:"pfcpMsg"`
	Addr     *net.UDPAddr       `json:"addr"`
	SmfIp    string             `json:"smfIp"`
	UpNodeID smf_context.NodeID `json:"upNodeID"`
}

func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

// SendPfcpMsgToAdapter send pfcp msg to upf-adapter in http/json encoded format
func SendPfcpMsgToAdapter(upNodeID smf_context.NodeID, msg message.Message, addr *net.UDPAddr, eventData interface{}, url string) (*http.Response, error) {
	// get IP
	ip_str := GetLocalIP()

	buf := make([]byte, msg.MarshalLen())
	err := msg.MarshalTo(buf)
	if err != nil {
		logger.PfcpLog.Errorf("marshal failed: %v", err)
		return nil, err
	}

	udpPodMsg := &UdpPodPfcpMsg{
		UpNodeID: upNodeID,
		SmfIp:    ip_str,
		Msg:      adapterMessage{Body: buf},
		Addr:     addr,
	}

	udpPodMsgJson, err := json.Marshal(udpPodMsg)
	if err != nil {
		logger.PfcpLog.Errorf("json marshal failed: %v", err)
		return nil, err
	}

	logger.PfcpLog.Debugf("json encoded udpPodMsg [%s] ", udpPodMsgJson)
	// change the IP here
	logger.PfcpLog.Debugf("send to :%s\n", url)

	bodyReader := bytes.NewReader(udpPodMsgJson)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bodyReader)
	if err != nil {
		logger.PfcpLog.Errorf("client: could not create request: %s\n", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := http.Client{
		Timeout: 30 * time.Second,
	}
	// waiting for http response
	rsp, err := client.Do(req)
	if err != nil {
		logger.PfcpLog.Errorf("client: error making http request: %s\n", err)
		return nil, err
	}

	return rsp, nil
}
