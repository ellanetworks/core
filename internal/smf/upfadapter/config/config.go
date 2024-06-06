package config

import (
	"bytes"
	"net"
	"os"
	"sync"
	"time"

	"upf-adapter/logger"

	"github.com/omec-project/pfcp"
	"github.com/omec-project/pfcp/pfcpType"
)

type UPFStatus int

const MaxUpfProbeRetryInterval time.Duration = 5 // Seconds

var UpfCfg Config

const (
	NotAssociated          UPFStatus = 0
	AssociatedSettingUp    UPFStatus = 1
	AssociatedSetUpSuccess UPFStatus = 2
)

// UPF structure
type UPNode struct {
	UpfName     string
	NodeID      pfcpType.NodeID
	ANIP        net.IP
	State       UPFStatus
	LastAssoRsp interface{}
	LastHBRsp   interface{}
	UpfLock     sync.RWMutex
}

// All UPF nodes
type Config struct {
	UpfListLock sync.RWMutex
	UPFs        map[string]*UPNode
}

type UdpPodMsgType int

type UdpPodPfcpMsg struct {
	SmfIp    string          `json:"smfIp"`
	UpNodeID pfcpType.NodeID `json:"upNodeID"`
	// message type contains in Msg.Header
	Msg  pfcp.Message `json:"pfcpMsg"`
	Addr *net.UDPAddr `json:"addr"`
}

type PfcpHttpRsp struct {
	Rsp []byte
	Err error
}

type PfcpTxnChan chan PfcpHttpRsp

var (
	UpfTxns      map[uint32]PfcpTxnChan
	UpfTxnsMutex = sync.RWMutex{}
)

var (
	UpfAdapterIp       net.IP
	UpfServerStartTime time.Time
)

func init() {
	podIpStr := os.Getenv("POD_IP")
	podIp := net.ParseIP(podIpStr)
	UpfAdapterIp = podIp.To4()

	UpfCfg = Config{
		UPFs: make(map[string]*UPNode),
	}

	UpfTxns = make(map[uint32]PfcpTxnChan)
}

// BuildPfcpHeartbeatRequest shall trigger hearbeat request to all Attached UPFs
func BuildPfcpHeartbeatRequest() (pfcp.HeartbeatRequest, error) {
	msg := pfcp.HeartbeatRequest{}

	msg.RecoveryTimeStamp = &pfcpType.RecoveryTimeStamp{
		RecoveryTimeStamp: UpfServerStartTime,
	}

	return msg, nil
}

func IsUpfAssociated(nodeId pfcpType.NodeID) bool {
	UpfCfg.UpfListLock.RLock()
	defer UpfCfg.UpfListLock.RUnlock()

	logger.CfgLog.Debugf("associated upfs: [ %v]", UpfCfg.UPFs)

	if upf := UpfCfg.UPFs[string(nodeId.NodeIdValue)]; upf != nil {
		if upf.State == AssociatedSetUpSuccess {
			logger.CfgLog.Debugf("upf:[%v] associated", string(nodeId.NodeIdValue))
			return true
		}
		logger.CfgLog.Debugf("upf:[%v] not associated", string(nodeId.NodeIdValue))
		return false
	}

	logger.CfgLog.Debugf("upf:[%v] not configured yet", string(nodeId.NodeIdValue))
	return false
}

func GetUpfFromNodeId(nodeId *pfcpType.NodeID) *UPNode {
	UpfCfg.UpfListLock.RLock()
	defer UpfCfg.UpfListLock.RUnlock()

	logger.CfgLog.Debugf("getting upf from node id [%v] ", nodeId)
	logger.CfgLog.Debugf("content of upf config [%v] ", UpfCfg.UPFs)

	for _, upf := range UpfCfg.UPFs {
		if nodeId.NodeIdType == pfcpType.NodeIdTypeIpv4Address {
			if bytes.Equal(upf.ANIP.To4(), nodeId.NodeIdValue) {
				logger.CfgLog.Debugf("getting upf from node id, ip-addr [%v, %v] successful", nodeId, upf.ANIP.To4())
				return upf
			}
		} else if nodeId.NodeIdType == pfcpType.NodeIdTypeFqdn &&
			upf.NodeID.NodeIdType == pfcpType.NodeIdTypeFqdn {
			if bytes.Equal(nodeId.NodeIdValue, upf.NodeID.NodeIdValue) {
				logger.CfgLog.Debugf("getting upf from node id, fqdn [%v, %v] successful", nodeId, nodeId.NodeIdValue)
				return upf
			}
		}
	}
	logger.CfgLog.Errorf("getting upf from node id [%v] failure", nodeId)
	return nil
}

func InsertUpfNode(nodeId pfcpType.NodeID) {
	UpfCfg.UpfListLock.Lock()
	defer UpfCfg.UpfListLock.Unlock()

	// if UPF is already not added
	if _, ok := UpfCfg.UPFs[string(nodeId.NodeIdValue)]; !ok {

		upf := UPNode{
			UpfName: string(nodeId.NodeIdValue),
			State:   NotAssociated,
			NodeID:  nodeId,
			ANIP:    nodeId.ResolveNodeIdToIp(),
		}
		UpfCfg.UPFs[string(nodeId.NodeIdValue)] = &upf
		logger.CfgLog.Infof("inserting upf node [%v] ", string(nodeId.NodeIdValue))
	}
}

func ActivateUpfNode(nodeId *pfcpType.NodeID) *UPNode {
	logger.CfgLog.Infof("activating upf node [%v]", nodeId)
	if upf := GetUpfFromNodeId(nodeId); upf != nil {
		UpfCfg.UpfListLock.Lock()
		upf.State = AssociatedSetUpSuccess
		UpfCfg.UpfListLock.Unlock()
		return upf
	}
	logger.CfgLog.Errorf("upf node [%v] not found ", nodeId)
	return nil
}

func RemoveUpfNode(upfName string) {
	UpfCfg.UpfListLock.Lock()
	defer UpfCfg.UpfListLock.Unlock()

	if upf, ok := UpfCfg.UPFs[upfName]; ok {
		delete(UpfCfg.UPFs, upf.UpfName)
		logger.CfgLog.Infof("deleting upf node [%v] ", upf.UpfName)
	}
}

func InsertUpfPfcpTxn(seq uint32, pfcpTxnChan PfcpTxnChan) {
	logger.CfgLog.Debugf(" inserting transaction with sequence number [%v]", seq)
	UpfTxnsMutex.Lock()
	UpfTxns[seq] = pfcpTxnChan
	UpfTxnsMutex.Unlock()
}

func GetUpfPfcpTxn(seq uint32) PfcpTxnChan {
	UpfTxnsMutex.Lock()
	defer UpfTxnsMutex.Unlock()
	pfcpTxnChan := UpfTxns[seq]
	if pfcpTxnChan != nil {
		delete(UpfTxns, seq)
		logger.CfgLog.Debugf("fetch transaction with sequence number [%v] successful", seq)
		return pfcpTxnChan
	}
	logger.CfgLog.Errorf("fetch transaction with sequence number [%v] failure", seq)

	return nil
}

func (upf *UPNode) PreservePfcpAssociationRsp(pfcpRspBody pfcp.PFCPAssociationSetupResponse) {
	// find the UPF
	logger.CfgLog.Debugf("storing pfcp association response for upf [%v] ", upf)
	upf.UpfLock.Lock()
	defer upf.UpfLock.Unlock()
	upf.LastAssoRsp = pfcpRspBody
}

func (upf *UPNode) PreservePfcpHeartBeatRsp(pfcpRspBody pfcp.HeartbeatResponse) {
	// find the UPF
	logger.CfgLog.Debugf("storing pfcp heartbeat response for upf [%v] ", upf)
	upf.UpfLock.Lock()
	defer upf.UpfLock.Unlock()
	upf.LastHBRsp = pfcpRspBody
}

type UdpPodPfcpRspMsg struct {
	// message type contains in Msg.Header
	Msg pfcp.Message `json:"msg"`
}
