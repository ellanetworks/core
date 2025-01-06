// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pfcp

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

	"github.com/ellanetworks/core/internal/logger"
	smf_context "github.com/ellanetworks/core/internal/smf/context"
	upf "github.com/ellanetworks/core/internal/upf/core"
	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
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

func SendPfcpSessionEstablishmentRequest(
	upNodeID smf_context.NodeID,
	ctx *smf_context.SMContext,
	pdrList []*smf_context.PDR,
	farList []*smf_context.FAR,
	barList []*smf_context.BAR,
	qerList []*smf_context.QER,
	upfPort uint16,
) (bool, error) {
	upNodeIDStr := upNodeID.ResolveNodeIdToIp().String()
	pfcpContext, ok := ctx.PFCPContext[upNodeIDStr]
	if !ok {
		return false, fmt.Errorf("PFCP Context not found for NodeID[%v]", upNodeID)
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
		return false, err
	}
	rsp, err := upf.HandlePfcpSessionEstablishmentRequest(pfcpMsg)
	if err != nil {
		return false, fmt.Errorf("failed to handle PFCP Session Establishment Request in upf: %v", err)
	}
	addPduSessionAnchor, err := HandlePfcpSessionEstablishmentRequest(rsp)
	if err != nil {
		return false, fmt.Errorf("failed to handle PFCP Session Establishment Response: %v", err)
	}
	return addPduSessionAnchor, nil
}

func HandlePfcpSessionEstablishmentRequest(msg *message.SessionEstablishmentResponse) (bool, error) {
	var addPduSessionAnchor bool
	SEID := msg.SEID()
	smContext := smf_context.GetSMContextBySEID(SEID)
	if smContext == nil {
		return addPduSessionAnchor, fmt.Errorf("failed to find SMContext for SEID[%d]", SEID)
	}
	smContext.SMLock.Lock()

	if msg.NodeID == nil {
		return addPduSessionAnchor, fmt.Errorf("PFCP Session Establishment Response missing NodeID")
	}
	nodeID, err := msg.NodeID.NodeID()
	if err != nil {
		return addPduSessionAnchor, fmt.Errorf("failed to parse NodeID IE: %+v", err)
	}

	if msg.UPFSEID != nil {
		pfcpSessionCtx := smContext.PFCPContext[nodeID]
		rspUPFseid, err := msg.UPFSEID.FSEID()
		if err != nil {
			return addPduSessionAnchor, fmt.Errorf("failed to parse FSEID IE: %+v", err)
		}
		pfcpSessionCtx.RemoteSEID = rspUPFseid.SEID
		smContext.SubPfcpLog.Infof("in HandlePfcpSessionEstablishmentResponse rsp.UPFSEID.Seid [%v] ", rspUPFseid.SEID)
	}

	// Get N3 interface UPF
	ANUPF := smContext.Tunnel.DataPathPool.GetDefaultPath().FirstDPNode

	// UE IP-Addr(only v4 supported)
	if msg.CreatedPDR != nil {
		ueIPAddress := FindUEIPAddress(msg.CreatedPDR)
		if ueIPAddress != nil {
			smContext.SubPfcpLog.Infof("upf provided ue ip address [%v]", ueIPAddress)
			// Release previous locally allocated UE IP-Addr
			err := smContext.ReleaseUeIpAddr()
			if err != nil {
				logger.SmfLog.Errorf("failed to release UE IP-Addr: %+v", err)
			}

			// Update with one received from UPF
			smContext.PDUAddress.Ip = ueIPAddress
			smContext.PDUAddress.UpfProvided = true
		}

		// Store F-TEID created by UPF
		fteid, err := FindFTEID(msg.CreatedPDR)
		if err != nil {
			return addPduSessionAnchor, fmt.Errorf("failed to parse TEID IE: %+v", err)
		}
		logger.SmfLog.Infof("created PDR FTEID: %+v", fteid)
		ANUPF.UpLinkTunnel.TEID = fteid.TEID
		upf := smf_context.GetUserPlaneInformation().UPF.UPF
		if upf == nil {
			return addPduSessionAnchor, fmt.Errorf("can't find UPF[%s]", nodeID)
		}
		upf.N3Interfaces = make([]smf_context.UPFInterfaceInfo, 0)
		n3Interface := smf_context.UPFInterfaceInfo{}
		n3Interface.IPv4EndPointAddresses = append(n3Interface.IPv4EndPointAddresses, fteid.IPv4Address)
		upf.N3Interfaces = append(upf.N3Interfaces, n3Interface)
	}
	smContext.SMLock.Unlock()

	if msg.NodeID == nil {
		return addPduSessionAnchor, fmt.Errorf("PFCP Session Establishment Response missing NodeID")
	}

	if msg.Cause == nil {
		return addPduSessionAnchor, fmt.Errorf("PFCP Session Establishment Response missing Cause")
	}
	causeValue, err := msg.Cause.Cause()
	if err != nil {
		return addPduSessionAnchor, fmt.Errorf("failed to parse Cause IE: %+v", err)
	}
	if causeValue == ie.CauseRequestAccepted {
		smContext.SBIPFCPCommunicationChan <- smf_context.SessionEstablishSuccess
		smContext.SubPfcpLog.Infof("PFCP Session Establishment accepted")
	} else {
		smContext.SBIPFCPCommunicationChan <- smf_context.SessionEstablishFailed
		smContext.SubPfcpLog.Errorf("PFCP Session Establishment rejected with cause [%v]", causeValue)
	}

	if smf_context.SMF_Self().ULCLSupport && smContext.BPManager != nil {
		if smContext.BPManager.BPStatus == smf_context.AddingPSA {
			smContext.SubPfcpLog.Infoln("keep Adding PSAndULCL")
			addPduSessionAnchor = true
			smContext.BPManager.BPStatus = smf_context.AddingPSA
		}
	}
	return addPduSessionAnchor, nil
}

func HandlePfcpSessionModificationResponse(msg *message.SessionModificationResponse) (bool, error) {
	var addPduSessionAnchor bool
	SEID := msg.SEID()

	smContext := smf_context.GetSMContextBySEID(SEID)

	if smf_context.SMF_Self().ULCLSupport && smContext.BPManager != nil {
		if smContext.BPManager.BPStatus == smf_context.AddingPSA {
			addPduSessionAnchor = true
		}
	}

	if msg.Cause == nil {
		return addPduSessionAnchor, fmt.Errorf("PFCP Session Modification Response missing Cause")
	}

	causeValue, err := msg.Cause.Cause()
	if err != nil {
		return addPduSessionAnchor, fmt.Errorf("failed to parse Cause IE: %+v", err)
	}

	if causeValue == ie.CauseRequestAccepted {
		smContext.SubPduSessLog.Infoln("PFCP Modification Response Accept")
		if smContext.SMContextState == smf_context.SmStatePfcpModify {
			upfNodeID := smContext.GetNodeIDByLocalSEID(SEID)
			upfIP := upfNodeID.ResolveNodeIdToIp().String()
			delete(smContext.PendingUPF, upfIP)
			smContext.SubPduSessLog.Debugf("Delete pending pfcp response: UPF IP [%s]\n", upfIP)

			if smContext.PendingUPF.IsEmpty() {
				smContext.SBIPFCPCommunicationChan <- smf_context.SessionUpdateSuccess
			}

			if smf_context.SMF_Self().ULCLSupport && smContext.BPManager != nil {
				if smContext.BPManager.BPStatus == smf_context.UnInitialized {
					smContext.BPManager.BPStatus = smf_context.AddingPSA
					addPduSessionAnchor = true
				}
			}
		}

		smContext.SubPfcpLog.Infof("PFCP Session Modification Success[%d]\n", SEID)
	} else {
		smContext.SubPfcpLog.Infof("PFCP Session Modification Failed[%d]\n", SEID)
		if smContext.SMContextState == smf_context.SmStatePfcpModify {
			smContext.SBIPFCPCommunicationChan <- smf_context.SessionUpdateFailed
		}
	}

	smContext.SubCtxLog.Debugln("PFCP Session Context")
	for _, ctx := range smContext.PFCPContext {
		smContext.SubCtxLog.Debugln(ctx.String())
	}
	return addPduSessionAnchor, nil
}

func SendPfcpSessionModificationRequest(
	upNodeID smf_context.NodeID,
	ctx *smf_context.SMContext,
	pdrList []*smf_context.PDR,
	farList []*smf_context.FAR,
	barList []*smf_context.BAR,
	qerList []*smf_context.QER,
	upfPort uint16,
) (bool, error) {
	seqNum := getSeqNumber()
	upNodeIDStr := upNodeID.ResolveNodeIdToIp().String()
	pfcpContext, ok := ctx.PFCPContext[upNodeIDStr]
	if !ok {
		return false, fmt.Errorf("PFCP Context not found for NodeID[%s]", upNodeIDStr)
	}
	pfcpMsg, err := BuildPfcpSessionModificationRequest(seqNum, pfcpContext.LocalSEID, pfcpContext.RemoteSEID, smf_context.SMF_Self().CPNodeID.ResolveNodeIdToIp(), pdrList, farList, qerList)
	if err != nil {
		return false, fmt.Errorf("failed to build PFCP Session Modification Request: %v", err)
	}
	rsp, err := upf.HandlePfcpSessionModificationRequest(pfcpMsg)
	if err != nil {
		return false, fmt.Errorf("failed to handle PFCP Session Establishment Request in upf: %v", err)
	}
	addPduSessionAnchor, err := HandlePfcpSessionModificationResponse(rsp)
	if err != nil {
		return false, fmt.Errorf("failed to handle PFCP Session Establishment Response: %v", err)
	}
	return addPduSessionAnchor, nil
}

func HandlePfcpSessionDeletionResponse(msg *message.SessionDeletionResponse) error {
	SEID := msg.SEID()
	smContext := smf_context.GetSMContextBySEID(SEID)

	if smContext == nil {
		return fmt.Errorf("SMContext not found for SEID[%d]", SEID)
	}

	if msg.Cause == nil {
		return fmt.Errorf("PFCP Session Deletion Response missing Cause")
	}

	causeValue, err := msg.Cause.Cause()
	if err != nil {
		return fmt.Errorf("failed to parse Cause IE: %+v", err)
	}

	if causeValue == ie.CauseRequestAccepted {
		if smContext.SMContextState == smf_context.SmStatePfcpRelease {
			upfNodeID := smContext.GetNodeIDByLocalSEID(SEID)
			upfIP := upfNodeID.ResolveNodeIdToIp().String()
			delete(smContext.PendingUPF, upfIP)
			smContext.SubPduSessLog.Debugf("Delete pending pfcp response: UPF IP [%s]\n", upfIP)

			if smContext.PendingUPF.IsEmpty() && !smContext.LocalPurged {
				smContext.SBIPFCPCommunicationChan <- smf_context.SessionReleaseSuccess
			}
		}
		smContext.SubPfcpLog.Infof("PFCP Session Deletion Success[%d]\n", SEID)
	} else {
		if smContext.SMContextState == smf_context.SmStatePfcpRelease && !smContext.LocalPurged {
			smContext.SBIPFCPCommunicationChan <- smf_context.SessionReleaseSuccess
		}
		smContext.SubPfcpLog.Infof("PFCP Session Deletion Failed[%d]\n", SEID)
	}

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

	rsp, err := upf.HandlePfcpSessionDeletionRequest(pfcpMsg)
	if err != nil {
		return fmt.Errorf("failed to handle PFCP Session Establishment Request in upf: %v", err)
	}
	err = HandlePfcpSessionDeletionResponse(rsp)
	if err != nil {
		return fmt.Errorf("failed to handle PFCP Session Establishment Response: %v", err)
	}
	return nil
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
		logger.SmfLog.Errorf("marshal failed: %v", err)
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
		logger.SmfLog.Errorf("json marshal failed: %v", err)
		return nil, err
	}

	logger.SmfLog.Debugf("json encoded udpPodMsg [%s] ", udpPodMsgJson)
	// change the IP here
	logger.SmfLog.Debugf("send to :%s\n", url)

	bodyReader := bytes.NewReader(udpPodMsgJson)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bodyReader)
	if err != nil {
		logger.SmfLog.Errorf("client: could not create request: %s\n", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := http.Client{
		Timeout: 30 * time.Second,
	}
	// waiting for http response
	rsp, err := client.Do(req)
	if err != nil {
		logger.SmfLog.Errorf("client: error making http request: %s\n", err)
		return nil, err
	}

	return rsp, nil
}
