package context

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/pfcp/pfcpType"
	"github.com/omec-project/util/util_3gpp"
	"github.com/yeastengine/ella/internal/smf/logger"
	"github.com/yeastengine/ella/internal/smf/qos"
	"github.com/yeastengine/ella/internal/smf/util"
)

// GTPTunnel represents the GTP tunnel information
type GTPTunnel struct {
	SrcEndPoint  *DataPathNode
	DestEndPoint *DataPathNode

	PDR  map[string]*PDR
	TEID uint32
}

type DataPathNode struct {
	UPF *UPF
	// DataPathToAN *DataPathDownLink
	// DataPathToDN map[string]*DataPathUpLink //uuid to DataPathLink

	UpLinkTunnel   *GTPTunnel
	DownLinkTunnel *GTPTunnel
	// for UE Routing Topology
	// for special case:
	// branching & leafnode

	// InUse                bool
	IsBranchingPoint bool
	// DLDataPathLinkForPSA *DataPathUpLink
	// BPUpLinkPDRs         map[string]*DataPathDownLink // uuid to UpLink
}

type DataPath struct {
	// Data Path Double Link List
	FirstDPNode *DataPathNode
	// meta data
	Destination       Destination
	Activated         bool
	IsDefaultPath     bool
	HasBranchingPoint bool
}

type DataPathPool map[int64]*DataPath

type Destination struct {
	DestinationIP   string
	DestinationPort string
	Url             string
}

func NewDataPathNode() *DataPathNode {
	node := &DataPathNode{
		UpLinkTunnel:   &GTPTunnel{PDR: make(map[string]*PDR)},
		DownLinkTunnel: &GTPTunnel{PDR: make(map[string]*PDR)},
	}
	return node
}

func NewDataPath() *DataPath {
	dataPath := &DataPath{
		Destination: Destination{
			DestinationIP:   "",
			DestinationPort: "",
			Url:             "",
		},
	}

	return dataPath
}

func NewDataPathPool() DataPathPool {
	pool := make(map[int64]*DataPath)
	return pool
}

func (node *DataPathNode) AddNext(next *DataPathNode) {
	node.DownLinkTunnel.SrcEndPoint = next
}

func (node *DataPathNode) AddPrev(prev *DataPathNode) {
	node.UpLinkTunnel.SrcEndPoint = prev
}

func (node *DataPathNode) Next() *DataPathNode {
	if node.DownLinkTunnel == nil {
		return nil
	}
	next := node.DownLinkTunnel.SrcEndPoint
	return next
}

func (node *DataPathNode) Prev() *DataPathNode {
	if node.UpLinkTunnel == nil {
		return nil
	}
	prev := node.UpLinkTunnel.SrcEndPoint
	return prev
}

func (node *DataPathNode) ActivateUpLinkTunnel(smContext *SMContext) error {
	var err error
	var pdr *PDR
	var flowQer *QER
	logger.CtxLog.Traceln("In ActivateUpLinkTunnel")
	node.UpLinkTunnel.SrcEndPoint = node.Prev()
	node.UpLinkTunnel.DestEndPoint = node

	destUPF := node.UPF

	// Iterate through PCC Rules to install PDRs
	pccRuleUpdate := smContext.SmPolicyUpdates[0].PccRuleUpdate

	if pccRuleUpdate != nil {
		addRules := pccRuleUpdate.GetAddPccRuleUpdate()

		for name, rule := range addRules {
			if pdr, err = destUPF.BuildCreatePdrFromPccRule(rule); err == nil {
				// Add PCC Rule Qos Data QER
				if flowQer, err = node.CreatePccRuleQer(smContext, rule.RefQosData[0], rule.RefTcData[0]); err == nil {
					pdr.QER = append(pdr.QER, flowQer)
				}
				// Set PDR in Tunnel
				node.UpLinkTunnel.PDR[name] = pdr
			}
		}
	} else {
		// Default PDR
		if pdr, err = destUPF.AddPDR(); err != nil {
			logger.CtxLog.Errorln("In ActivateUpLinkTunnel UPF IP: ", node.UPF.NodeID.ResolveNodeIdToIp().String())
			logger.CtxLog.Errorln("Allocate PDR Error: ", err)
			return fmt.Errorf("add PDR failed: %s", err)
		} else {
			node.UpLinkTunnel.PDR["default"] = pdr
		}
	}

	if err = smContext.PutPDRtoPFCPSession(destUPF.NodeID, node.UpLinkTunnel.PDR); err != nil {
		logger.CtxLog.Errorln("Put PDR Error: ", err)
		return err
	}

	if teid, err := smfContext.DrsmCtxts.TeidPool.AllocateInt32ID(); err != nil {
		logger.CtxLog.Errorf("Generate uplink TEID fail: %s", err)
		return err
	} else {
		node.UpLinkTunnel.TEID = (uint32(teid))
	}

	return nil
}

func (node *DataPathNode) ActivateDownLinkTunnel(smContext *SMContext) error {
	var err error
	var pdr *PDR
	var flowQer *QER
	node.DownLinkTunnel.SrcEndPoint = node.Next()
	node.DownLinkTunnel.DestEndPoint = node

	destUPF := node.UPF
	// Iterate through PCC Rules to install PDRs
	pccRuleUpdate := smContext.SmPolicyUpdates[0].PccRuleUpdate
	if pccRuleUpdate != nil {
		addRules := pccRuleUpdate.GetAddPccRuleUpdate()
		for name, rule := range addRules {
			if pdr, err = destUPF.BuildCreatePdrFromPccRule(rule); err == nil {
				// Add PCC Rule Qos Data QER
				if flowQer, err = node.CreatePccRuleQer(smContext, rule.RefQosData[0], rule.RefTcData[0]); err == nil {
					pdr.QER = append(pdr.QER, flowQer)
				}
				// Set PDR in Tunnel
				node.DownLinkTunnel.PDR[name] = pdr
			}
		}
	} else {
		// Default PDR
		if pdr, err = destUPF.AddPDR(); err != nil {
			logger.CtxLog.Errorln("In ActivateDownLinkTunnel UPF IP: ", node.UPF.NodeID.ResolveNodeIdToIp().String())
			logger.CtxLog.Errorln("Allocate PDR Error: ", err)
			return fmt.Errorf("add PDR failed: %s", err)
		} else {
			node.DownLinkTunnel.PDR["default"] = pdr
		}
	}

	// Put PDRs in PFCP session
	if err = smContext.PutPDRtoPFCPSession(destUPF.NodeID, node.DownLinkTunnel.PDR); err != nil {
		logger.CtxLog.Errorln("Put PDR Error: ", err)
		return err
	}

	// Generate TEID for Tunnel
	if teid, err := smfContext.DrsmCtxts.TeidPool.AllocateInt32ID(); err != nil {
		logger.CtxLog.Errorf("Generate downlink TEID fail: %s", err)
		return err
	} else {
		node.DownLinkTunnel.TEID = (uint32(teid))
	}

	return nil
}

func (node *DataPathNode) DeactivateUpLinkTunnel(smContext *SMContext) {
	for name, pdr := range node.UpLinkTunnel.PDR {
		if pdr != nil {
			logger.CtxLog.Infof("Deactivaed UpLinkTunnel PDR name[%v], id[%v]", name, pdr.PDRID)

			// Remove PDR from PFCP Session
			smContext.RemovePDRfromPFCPSession(node.UPF.NodeID, pdr)

			// Remove of UPF
			err := node.UPF.RemovePDR(pdr)
			if err != nil {
				logger.CtxLog.Warnln("Deactivaed UpLinkTunnel", err)
			}

			if far := pdr.FAR; far != nil {
				err = node.UPF.RemoveFAR(far)
				if err != nil {
					logger.CtxLog.Warnln("Deactivaed UpLinkTunnel", err)
				}

				bar := far.BAR
				if bar != nil {
					err = node.UPF.RemoveBAR(bar)
					if err != nil {
						logger.CtxLog.Warnln("Deactivaed UpLinkTunnel", err)
					}
				}
			}
			if qerList := pdr.QER; qerList != nil {
				for _, qer := range qerList {
					if qer != nil {
						err = node.UPF.RemoveQER(qer)
						if err != nil {
							logger.CtxLog.Warnln("Deactivaed UpLinkTunnel", err)
						}
					}
				}
			}
		}
	}

	teid := node.DownLinkTunnel.TEID
	smfContext.DrsmCtxts.TeidPool.ReleaseInt32ID(int32(teid))
	node.DownLinkTunnel = &GTPTunnel{}
}

func (node *DataPathNode) DeactivateDownLinkTunnel(smContext *SMContext) {
	for name, pdr := range node.DownLinkTunnel.PDR {
		if pdr != nil {
			logger.CtxLog.Infof("Deactivaed DownLinkTunnel PDR name[%v], id[%v]", name, pdr.PDRID)

			// Remove PDR from PFCP Session
			smContext.RemovePDRfromPFCPSession(node.UPF.NodeID, pdr)

			// Remove from UPF
			err := node.UPF.RemovePDR(pdr)
			if err != nil {
				logger.CtxLog.Warnln("Deactivaed DownLinkTunnel", err)
			}

			if far := pdr.FAR; far != nil {
				err = node.UPF.RemoveFAR(far)
				if err != nil {
					logger.CtxLog.Warnln("Deactivaed DownLinkTunnel", err)
				}

				bar := far.BAR
				if bar != nil {
					err = node.UPF.RemoveBAR(bar)
					if err != nil {
						logger.CtxLog.Warnln("Deactivaed DownLinkTunnel", err)
					}
				}
			}
			if qerList := pdr.QER; qerList != nil {
				for _, qer := range qerList {
					if qer != nil {
						err = node.UPF.RemoveQER(qer)
						if err != nil {
							logger.CtxLog.Warnln("Deactivaed UpLinkTunnel", err)
						}
					}
				}
			}
		}
	}

	teid := node.DownLinkTunnel.TEID
	smfContext.DrsmCtxts.TeidPool.ReleaseInt32ID(int32(teid))
	node.DownLinkTunnel = &GTPTunnel{}
}

func (node *DataPathNode) GetUPFID() (id string, err error) {
	node_ip := node.GetNodeIP()
	var exist bool

	if id, exist = smfContext.UserPlaneInformation.UPFsIPtoID[node_ip]; !exist {
		AllocateUPFID()
		if id, exist = smfContext.UserPlaneInformation.UPFsIPtoID[node_ip]; !exist {
			err = fmt.Errorf("UPNode IP %s doesn't exist in smfcfg.yaml", node_ip)
			return "", err
		}
	}

	return id, nil
}

func (node *DataPathNode) GetNodeIP() (ip string) {
	ip = node.UPF.NodeID.ResolveNodeIdToIp().String()
	return
}

func (node *DataPathNode) IsANUPF() bool {
	if node.Prev() == nil {
		return true
	} else {
		return false
	}
}

func (node *DataPathNode) IsAnchorUPF() bool {
	if node.Next() == nil {
		return true
	} else {
		return false
	}
}

func (dataPathPool DataPathPool) GetDefaultPath() (dataPath *DataPath) {
	for _, path := range dataPathPool {
		if path.IsDefaultPath {
			dataPath = path
			return
		}
	}
	return
}

func (dataPath *DataPath) String() string {
	firstDPNode := dataPath.FirstDPNode

	var str string

	str += "DataPath Meta Information\n"
	str += "Activated: " + strconv.FormatBool(dataPath.Activated) + "\n"
	str += "IsDefault Path: " + strconv.FormatBool(dataPath.IsDefaultPath) + "\n"
	str += "Has Braching Point: " + strconv.FormatBool(dataPath.HasBranchingPoint) + "\n"
	str += "Destination IP: " + dataPath.Destination.DestinationIP + "\n"
	str += "Destination Port: " + dataPath.Destination.DestinationPort + "\n"

	str += "DataPath Routing Information\n"
	index := 1
	for curDPNode := firstDPNode; curDPNode != nil; curDPNode = curDPNode.Next() {
		str += strconv.Itoa(index) + "th Node in the Path\n"
		str += "Current UPF IP: " + curDPNode.GetNodeIP() + "\n"
		if curDPNode.Prev() != nil {
			str += "Previous UPF IP: " + curDPNode.Prev().GetNodeIP() + "\n"
		} else {
			str += "Previous UPF IP: None\n"
		}

		if curDPNode.Next() != nil {
			str += "Next UPF IP: " + curDPNode.Next().GetNodeIP() + "\n"
		} else {
			str += "Next UPF IP: None\n"
		}

		index++
	}

	return str
}

func (dataPath *DataPath) validateDataPathUpfStatus() error {
	firstDPNode := dataPath.FirstDPNode
	for curDataPathNode := firstDPNode; curDataPathNode != nil; curDataPathNode = curDataPathNode.Next() {
		logger.PduSessLog.Infof("Nodes in Data Path [%v] and status [%v]",
			curDataPathNode.UPF.NodeID.ResolveNodeIdToIp().String(), curDataPathNode.UPF.UPFStatus.String())
		if curDataPathNode.UPF.UPFStatus != AssociatedSetUpSuccess {
			logger.PduSessLog.Errorf("UPF [%v] in DataPath not associated",
				curDataPathNode.UPF.NodeID.ResolveNodeIdToIp().String())
			return errors.New("UPF not associated in DataPath")
		}
	}
	return nil
}

func (dataPath *DataPath) ActivateUlDlTunnel(smContext *SMContext) error {
	firstDPNode := dataPath.FirstDPNode
	logger.PduSessLog.Traceln("In ActivateTunnelAndPDR")
	logger.PduSessLog.Traceln(dataPath.String())
	// Activate Tunnels
	for curDataPathNode := firstDPNode; curDataPathNode != nil; curDataPathNode = curDataPathNode.Next() {
		logger.PduSessLog.Traceln("Current DP Node IP: ", curDataPathNode.UPF.NodeID.ResolveNodeIdToIp().String())
		if err := curDataPathNode.ActivateUpLinkTunnel(smContext); err != nil {
			logger.CtxLog.Warnln(err)
			return err
		}
		if err := curDataPathNode.ActivateDownLinkTunnel(smContext); err != nil {
			logger.CtxLog.Warnln(err)
			return err
		}
	}
	return nil
}

func (dpNode *DataPathNode) CreatePccRuleQer(smContext *SMContext, qosData string, tcData string) (*QER, error) {
	smPolicyDec := smContext.SmPolicyUpdates[0].SmPolicyDecision
	refQos := qos.GetQoSDataFromPolicyDecision(smPolicyDec, qosData)
	tc := qos.GetTcDataFromPolicyDecision(smPolicyDec, tcData)

	// Get Flow Status
	gateStatus := pfcpType.GateOpen
	if tc != nil && tc.FlowStatus == models.FlowStatus_DISABLED {
		gateStatus = pfcpType.GateClose
	}

	var flowQER *QER

	if newQER, err := dpNode.UPF.AddQER(); err != nil {
		logger.PduSessLog.Errorln("new QER failed")
		return nil, err
	} else {
		newQER.QFI.QFI = qos.GetQosFlowIdFromQosId(refQos.QosId)

		// Flow Status
		newQER.GateStatus = &pfcpType.GateStatus{
			ULGate: gateStatus,
			DLGate: gateStatus,
		}

		// Rates
		newQER.MBR = &pfcpType.MBR{
			ULMBR: util.BitRateTokbps(refQos.MaxbrUl),
			DLMBR: util.BitRateTokbps(refQos.MaxbrDl),
		}

		flowQER = newQER
	}

	return flowQER, nil
}

func (dpNode *DataPathNode) CreateSessRuleQer(smContext *SMContext) (*QER, error) {
	var flowQER *QER

	sessionRule := smContext.SelectedSessionRule()

	// Get Default Qos-Data for the session
	smPolicyDec := smContext.SmPolicyUpdates[0].SmPolicyDecision

	defQosData := qos.GetDefaultQoSDataFromPolicyDecision(smPolicyDec)
	if newQER, err := dpNode.UPF.AddQER(); err != nil {
		logger.PduSessLog.Errorln("new QER failed")
		return nil, err
	} else {
		newQER.QFI.QFI = qos.GetQosFlowIdFromQosId(defQosData.QosId)
		newQER.GateStatus = &pfcpType.GateStatus{
			ULGate: pfcpType.GateOpen,
			DLGate: pfcpType.GateOpen,
		}
		newQER.MBR = &pfcpType.MBR{
			ULMBR: util.BitRateTokbps(sessionRule.AuthSessAmbr.Uplink),
			DLMBR: util.BitRateTokbps(sessionRule.AuthSessAmbr.Downlink),
		}

		flowQER = newQER
	}

	return flowQER, nil
}

// ActivateUpLinkPdr
func (dpNode *DataPathNode) ActivateUpLinkPdr(smContext *SMContext, defQER *QER, defPrecedence uint32) error {
	ueIpAddr := pfcpType.UEIPAddress{}
	if dpNode.UPF.IsUpfSupportUeIpAddrAlloc() {
		ueIpAddr.CHV4 = true
	} else {
		ueIpAddr.V4 = true
		ueIpAddr.Ipv4Address = smContext.PDUAddress.Ip.To4()
	}

	curULTunnel := dpNode.UpLinkTunnel
	for name, ULPDR := range curULTunnel.PDR {
		ULDestUPF := curULTunnel.DestEndPoint.UPF
		ULPDR.QER = append(ULPDR.QER, defQER)

		// Set Default precedence
		if ULPDR.Precedence == 0 {
			ULPDR.Precedence = defPrecedence
		}

		var iface *UPFInterfaceInfo
		if dpNode.IsANUPF() {
			iface = ULDestUPF.GetInterface(models.UpInterfaceType_N3, smContext.Dnn)
		} else {
			iface = ULDestUPF.GetInterface(models.UpInterfaceType_N9, smContext.Dnn)
		}

		if upIP, err := iface.IP(smContext.SelectedPDUSessionType); err != nil {
			logger.CtxLog.Errorf("activate UpLink PDR[%v] failed %v ", name, err)
			return err
		} else {
			ULPDR.PDI.SourceInterface = pfcpType.SourceInterface{InterfaceValue: pfcpType.SourceInterfaceAccess}
			ULPDR.PDI.LocalFTeid = &pfcpType.FTEID{
				V4:          true,
				Ipv4Address: upIP,
				Teid:        curULTunnel.TEID,
			}

			ULPDR.PDI.UEIPAddress = &ueIpAddr

			ULPDR.PDI.NetworkInstance = util_3gpp.Dnn(smContext.Dnn)
		}

		ULPDR.OuterHeaderRemoval = &pfcpType.OuterHeaderRemoval{
			OuterHeaderRemovalDescription: pfcpType.OuterHeaderRemovalGtpUUdpIpv4,
		}

		ULFAR := ULPDR.FAR
		ULFAR.ApplyAction = pfcpType.ApplyAction{
			Buff: false,
			Drop: false,
			Dupl: false,
			Forw: true,
			Nocp: false,
		}
		ULFAR.ForwardingParameters = &ForwardingParameters{
			DestinationInterface: pfcpType.DestinationInterface{
				InterfaceValue: pfcpType.DestinationInterfaceCore,
			},
			NetworkInstance: []byte(smContext.Dnn),
		}

		if dpNode.IsAnchorUPF() {
			ULFAR.ForwardingParameters.
				DestinationInterface.InterfaceValue = pfcpType.DestinationInterfaceSgiLanN6Lan
		}

		if nextULDest := dpNode.Next(); nextULDest != nil {
			nextULTunnel := nextULDest.UpLinkTunnel
			iface = nextULTunnel.DestEndPoint.UPF.GetInterface(models.UpInterfaceType_N9, smContext.Dnn)

			if upIP, err := iface.IP(smContext.SelectedPDUSessionType); err != nil {
				logger.CtxLog.Errorf("activate UpLink PDR[%v] failed %v ", name, err)
				return err
			} else {
				ULFAR.ForwardingParameters.OuterHeaderCreation = &pfcpType.OuterHeaderCreation{
					OuterHeaderCreationDescription: pfcpType.OuterHeaderCreationGtpUUdpIpv4,
					Ipv4Address:                    upIP,
					Teid:                           nextULTunnel.TEID,
				}
			}
		}
		logger.CtxLog.Infof("activate UpLink PDR[%v]:[%v] ", name, ULPDR)
	}
	return nil
}

func (dpNode *DataPathNode) ActivateDlLinkPdr(smContext *SMContext, defQER *QER, defPrecedence uint32, dataPath *DataPath) error {
	var iface *UPFInterfaceInfo
	curDLTunnel := dpNode.DownLinkTunnel

	// UPF provided UE ip-addr
	ueIpAddr := pfcpType.UEIPAddress{}
	if dpNode.UPF.IsUpfSupportUeIpAddrAlloc() {
		ueIpAddr.CHV4 = true
	} else {
		ueIpAddr.V4 = true
		ueIpAddr.Ipv4Address = smContext.PDUAddress.Ip.To4()
	}

	for name, DLPDR := range curDLTunnel.PDR {
		logger.CtxLog.Infof("activate Downlink PDR[%v]:[%v] ", name, DLPDR)
		DLDestUPF := curDLTunnel.DestEndPoint.UPF
		DLPDR.QER = append(DLPDR.QER, defQER)

		if DLPDR.Precedence == 0 {
			DLPDR.Precedence = defPrecedence
		}

		if dpNode.IsAnchorUPF() {
			DLPDR.PDI.UEIPAddress = &ueIpAddr
		} else {
			DLPDR.OuterHeaderRemoval = &pfcpType.OuterHeaderRemoval{
				OuterHeaderRemovalDescription: pfcpType.OuterHeaderRemovalGtpUUdpIpv4,
			}

			iface = DLDestUPF.GetInterface(models.UpInterfaceType_N9, smContext.Dnn)
			if upIP, err := iface.IP(smContext.SelectedPDUSessionType); err != nil {
				logger.CtxLog.Errorf("activate Downlink PDR[%v] failed %v ", name, err)
				return err
			} else {
				DLPDR.PDI.SourceInterface = pfcpType.SourceInterface{InterfaceValue: pfcpType.SourceInterfaceCore}
				DLPDR.PDI.LocalFTeid = &pfcpType.FTEID{
					V4:          true,
					Ipv4Address: upIP,
					Teid:        curDLTunnel.TEID,
				}

				DLPDR.PDI.UEIPAddress = &ueIpAddr
			}
		}

		DLFAR := DLPDR.FAR

		logger.PduSessLog.Traceln("Current DP Node IP: ", dpNode.UPF.NodeID.ResolveNodeIdToIp().String())
		logger.PduSessLog.Traceln("Before DLPDR OuterHeaderCreation")
		if nextDLDest := dpNode.Prev(); nextDLDest != nil {
			logger.PduSessLog.Traceln("In DLPDR OuterHeaderCreation")
			nextDLTunnel := nextDLDest.DownLinkTunnel

			DLFAR.ApplyAction = pfcpType.ApplyAction{
				Buff: true,
				Drop: false,
				Dupl: false,
				Forw: false,
				Nocp: true,
			}

			iface = nextDLDest.UPF.GetInterface(models.UpInterfaceType_N9, smContext.Dnn)

			if upIP, err := iface.IP(smContext.SelectedPDUSessionType); err != nil {
				logger.CtxLog.Errorf("activate Downlink PDR[%v] failed %v ", name, err)
				return err
			} else {
				DLFAR.ForwardingParameters = &ForwardingParameters{
					DestinationInterface: pfcpType.DestinationInterface{InterfaceValue: pfcpType.DestinationInterfaceAccess},
					OuterHeaderCreation: &pfcpType.OuterHeaderCreation{
						OuterHeaderCreationDescription: pfcpType.OuterHeaderCreationGtpUUdpIpv4,
						Ipv4Address:                    upIP,
						Teid:                           nextDLTunnel.TEID,
					},
				}
			}
		} else {
			if anIP := smContext.Tunnel.ANInformation.IPAddress; anIP != nil {
				ANUPF := dataPath.FirstDPNode
				DefaultDLPDR := ANUPF.DownLinkTunnel.PDR["default"] // TODO: Iterate over all PDRs
				DLFAR := DefaultDLPDR.FAR
				DLFAR.ForwardingParameters = new(ForwardingParameters)
				DLFAR.ForwardingParameters.DestinationInterface.InterfaceValue = pfcpType.DestinationInterfaceAccess
				DLFAR.ForwardingParameters.NetworkInstance = []byte(smContext.Dnn)
				DLFAR.ForwardingParameters.OuterHeaderCreation = new(pfcpType.OuterHeaderCreation)

				dlOuterHeaderCreation := DLFAR.ForwardingParameters.OuterHeaderCreation
				dlOuterHeaderCreation.OuterHeaderCreationDescription = pfcpType.OuterHeaderCreationGtpUUdpIpv4
				dlOuterHeaderCreation.Teid = smContext.Tunnel.ANInformation.TEID
				dlOuterHeaderCreation.Ipv4Address = smContext.Tunnel.ANInformation.IPAddress.To4()
			}
		}
		logger.CtxLog.Infof("activate Downlink PDR[%v]:[%v] ", name, DLPDR)
	}
	return nil
}

// ActivateTunnelAndPDR
func (dataPath *DataPath) ActivateTunnelAndPDR(smContext *SMContext, precedence uint32) error {
	// Check if UPF association is good
	if err := dataPath.validateDataPathUpfStatus(); err != nil {
		logger.PduSessLog.Error("One or more UPF in DataPath not associated")
		return err
	}

	// Allocate Local SEIDs
	smContext.AllocateLocalSEIDForDataPath(dataPath)

	// Allocate UL/DL PDRs for the Tunnels
	if err := dataPath.ActivateUlDlTunnel(smContext); err != nil {
		logger.PduSessLog.Errorf("Activate UL/DL Tunnel error %v", err.Error())
		return err
	}

	// Activate PDR
	for curDataPathNode := dataPath.FirstDPNode; curDataPathNode != nil; curDataPathNode = curDataPathNode.Next() {
		// Add flow QER
		defQER, err := curDataPathNode.CreateSessRuleQer(smContext)
		if err != nil {
			return err
		}

		logger.CtxLog.Traceln("Calculate ", curDataPathNode.UPF.PFCPAddr().String())

		// Setup UpLink PDR
		if curDataPathNode.UpLinkTunnel != nil {
			if err := curDataPathNode.ActivateUpLinkPdr(smContext, defQER, precedence); err != nil {
				logger.CtxLog.Errorf("Activate UpLink PDR error %v", err.Error())
			}
		}

		// Setup DownLink PDR
		if curDataPathNode.DownLinkTunnel != nil {
			if err := curDataPathNode.ActivateDlLinkPdr(smContext, defQER, precedence, dataPath); err != nil {
				logger.CtxLog.Errorf("Activate DlLink PDR error %v", err.Error())
			}
		}

		ueIpAddr := pfcpType.UEIPAddress{}
		if curDataPathNode.UPF.IsUpfSupportUeIpAddrAlloc() {
			ueIpAddr.CHV4 = true
		} else {
			ueIpAddr.V4 = true
			ueIpAddr.Ipv4Address = smContext.PDUAddress.Ip.To4()
		}

		if curDataPathNode.DownLinkTunnel != nil {
			if curDataPathNode.DownLinkTunnel.SrcEndPoint == nil {
				for _, DNDLPDR := range curDataPathNode.DownLinkTunnel.PDR {
					DNDLPDR.PDI.SourceInterface = pfcpType.SourceInterface{InterfaceValue: pfcpType.SourceInterfaceCore}
					DNDLPDR.PDI.NetworkInstance = util_3gpp.Dnn(smContext.Dnn)
					DNDLPDR.PDI.UEIPAddress = &ueIpAddr
				}
			}
		}
	}

	dataPath.Activated = true
	return nil
}

func (dataPath *DataPath) DeactivateTunnelAndPDR(smContext *SMContext) {
	firstDPNode := dataPath.FirstDPNode

	// Deactivate Tunnels
	for curDataPathNode := firstDPNode; curDataPathNode != nil; curDataPathNode = curDataPathNode.Next() {
		curDataPathNode.DeactivateUpLinkTunnel(smContext)
		curDataPathNode.DeactivateDownLinkTunnel(smContext)
	}

	dataPath.Activated = false
}
