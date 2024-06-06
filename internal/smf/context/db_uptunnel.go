package context

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/omec-project/pfcp/pfcpType"
	"github.com/omec-project/util/idgenerator"
	"github.com/omec-project/util/mongoapi"
	"github.com/yeastengine/ella/internal/smf/logger"
	"go.mongodb.org/mongo-driver/bson"
)

// DataPathPoolInDB type DataPathPoolInDB map[int64]DataPathInDB
type DataPathPoolInDB map[int64]*DataPathInDB

// UPTunnelInDB UPTunnel
type UPTunnelInDB struct {
	PathIDGenerator *idgenerator.IDGenerator
	DataPathPool    DataPathPoolInDB
	ANInformation   struct {
		IPAddress net.IP
		TEID      uint32
	}
}

type DataPathInDB struct {
	// Data Path Double Link List
	FirstDPNode *DataPathNodeInDB
	// meta data
	Destination       Destination
	Activated         bool
	IsDefaultPath     bool
	HasBranchingPoint bool
}

type DataPathNodeInDB struct {
	// UPF *UPF

	ULTunnelInfo *TunnelInfo
	DLTunnelInfo *TunnelInfo

	DataPathNodeUPFNodeID NodeIDInDB
	IsBranchingPoint      bool
}
type TunnelInfo struct {
	PDR                   map[string]*PDR
	DataPathNodeUPFNodeID NodeIDInDB
	TEID                  uint32
}

type NodeIDInDB struct {
	NodeIdValue []byte
	NodeIdType  uint8 // 0x00001111
}

type PFCPSessionContextInDB struct {
	PDRs       map[uint16]*PDR
	LocalSEID  string
	RemoteSEID string
	NodeID     pfcpType.NodeID
}

type PFCPContextInDB map[string]PFCPSessionContextInDB

func GetNodeIDInDB(nodeID pfcpType.NodeID) (nodeIDInDB NodeIDInDB) {
	nodeIDInDB = NodeIDInDB{
		NodeIdType:  nodeID.NodeIdType,
		NodeIdValue: nodeID.NodeIdValue,
	}
	return nodeIDInDB
}

func GetNodeID(nodeIDInDB NodeIDInDB) (nodeID pfcpType.NodeID) {
	nodeID = pfcpType.NodeID{
		NodeIdType:  nodeIDInDB.NodeIdType,
		NodeIdValue: nodeIDInDB.NodeIdValue,
	}
	return nodeID
}

func testEq(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	if (len(a) == len(b)) && (len(a) == 0) {
		return true
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func RecoverTunnel(tunnelInfo *TunnelInfo) (tunnel *GTPTunnel) {
	if tunnelInfo != nil {
		tunnel = &GTPTunnel{
			TEID: tunnelInfo.TEID,
			PDR:  tunnelInfo.PDR,
		}
		var nilValNode *DataPathNode = nil
		empty_nodeID := &NodeIDInDB{}

		if (tunnelInfo.DataPathNodeUPFNodeID.NodeIdType == empty_nodeID.NodeIdType) && (testEq(tunnelInfo.DataPathNodeUPFNodeID.NodeIdValue, empty_nodeID.NodeIdValue)) {
			endPoint := nilValNode
			tunnel.SrcEndPoint = endPoint
		} else {
			endPoint := RecoverFirstDPNode(tunnelInfo.DataPathNodeUPFNodeID)
			tunnel.SrcEndPoint = endPoint
		}
	}
	// TBA: recover dst endPoint
	return tunnel
}

func RecoverFirstDPNode(nodeIDInDB NodeIDInDB) (dataPathNode *DataPathNode) {
	fmt.Println("in RecoverFirstDPNode")
	nodeInDB := GetNodeInDBFromDB(nodeIDInDB)
	dataPathNode = &DataPathNode{
		IsBranchingPoint: nodeInDB.IsBranchingPoint,
		UPF:              RetrieveUPFNodeByNodeID(GetNodeID(nodeInDB.DataPathNodeUPFNodeID)),
		// UPF: RetrieveUPFNodeByNodeID(GetNodeID(nodeIDInDB)),
	}
	var nilVal *TunnelInfo = nil
	if nodeInDB.ULTunnelInfo != nilVal {
		dataPathNode.UpLinkTunnel = RecoverTunnel(nodeInDB.ULTunnelInfo)
	}
	if nodeInDB.DLTunnelInfo != nilVal {
		dataPathNode.DownLinkTunnel = RecoverTunnel(nodeInDB.DLTunnelInfo)
	}
	fmt.Println("RecoverFirstDPNode - dataPathNode", dataPathNode)
	if nodeInDB.ULTunnelInfo != nilVal {
		fmt.Println("nodeInDB.ULTunnelInfo != nilVal")
		dataPathNode.UpLinkTunnel.DestEndPoint = dataPathNode
	}
	if nodeInDB.DLTunnelInfo != nilVal {
		fmt.Println("nodeInDB.DLTunnelInfo != nilVal")
		dataPathNode.DownLinkTunnel.DestEndPoint = dataPathNode
	}
	return dataPathNode
}

func ToBsonMNodeInDB(data *DataPathNodeInDB) (ret bson.M) {
	// Marshal data into json format
	tmp, err := json.Marshal(data)
	if err != nil {
		logger.DataRepoLog.Errorf("ToBsonMNodeInDB marshall error: %v", err)
	}

	// unmarshal data into bson format
	err = json.Unmarshal(tmp, &ret)
	if err != nil {
		logger.DataRepoLog.Errorf("ToBsonMNodeInDB unmarshall error: %v", err)
	}

	return
}

func StoreNodeInDB(nodeInDB *DataPathNodeInDB) {
	itemBsonA := ToBsonMNodeInDB(nodeInDB)
	filter := bson.M{"nodeIDInDB": nodeInDB.DataPathNodeUPFNodeID}
	logger.DataRepoLog.Infof("filter: %+v", filter)

	_, postErr := mongoapi.CommonDBClient.RestfulAPIPost(NodeInDBCol, filter, itemBsonA)
	if postErr != nil {
		logger.DataRepoLog.Warnln(postErr)
	}
}

func GetNodeInDBFromDB(nodeIDInDB NodeIDInDB) (dataPathNodeInDB *DataPathNodeInDB) {
	filter := bson.M{}
	filter["nodeIDInDB"] = nodeIDInDB

	result, getOneErr := mongoapi.CommonDBClient.RestfulAPIGetOne(NodeInDBCol, filter)
	if getOneErr != nil {
		logger.DataRepoLog.Warnln(getOneErr)
	}

	dataPathNodeInDB = new(DataPathNodeInDB)
	fmt.Println("GetNodeInDBFromDB, smf state json : ", result)
	fmt.Println("GetNodeInDBFromDB, smf dataPathNodeInDB : ", dataPathNodeInDB)

	err := json.Unmarshal(mapToByte(result), dataPathNodeInDB)
	if err != nil {
		logger.DataRepoLog.Errorf("GetNodeInDBFromDB unmarshall error: %v", err)
		return nil
	}
	return dataPathNodeInDB
}

func RecoverDataPathNode(dataPathNodeInDB *DataPathNodeInDB) (dataPathNode *DataPathNode) {
	var nilValDpn *DataPathNodeInDB = nil
	var nilVarTunnelInfo *TunnelInfo = nil
	if dataPathNodeInDB != nilValDpn {
		dataPathNode := &DataPathNode{
			UPF:              RetrieveUPFNodeByNodeID(GetNodeID(dataPathNodeInDB.DataPathNodeUPFNodeID)),
			IsBranchingPoint: dataPathNodeInDB.IsBranchingPoint,
		}

		upLinkTunnel := new(GTPTunnel)
		downLinkTunnel := new(GTPTunnel)

		uLTunnelInfo := dataPathNodeInDB.ULTunnelInfo
		dLTunnelInfo := dataPathNodeInDB.DLTunnelInfo

		if uLTunnelInfo != nilVarTunnelInfo {
			upLinkTunnel = RecoverTunnel(dataPathNodeInDB.ULTunnelInfo)
			dataPathNode.UpLinkTunnel = upLinkTunnel
			dataPathNode.UpLinkTunnel.DestEndPoint = dataPathNode
		}

		if dLTunnelInfo != nilVarTunnelInfo {
			downLinkTunnel = RecoverTunnel(dataPathNodeInDB.DLTunnelInfo)
			dataPathNode.DownLinkTunnel = downLinkTunnel
			dataPathNode.DownLinkTunnel.DestEndPoint = dataPathNode
		}

		dataPathNode.UpLinkTunnel = upLinkTunnel
		dataPathNode.DownLinkTunnel = downLinkTunnel

		return dataPathNode
	}

	return nil
}

func StoreDataPathNode(dataPathNode *DataPathNode) (dataPathNodeInDB *DataPathNodeInDB) {
	var nilValDpn *DataPathNode = nil
	var nilValTunnel *GTPTunnel = nil
	if dataPathNode != nilValDpn {
		dataPathNodeInDB := &DataPathNodeInDB{
			DataPathNodeUPFNodeID: GetNodeIDInDB(dataPathNode.UPF.NodeID),
			IsBranchingPoint:      dataPathNode.IsBranchingPoint,
		}

		uLTunnelInfo := new(TunnelInfo)
		dLTunnelInfo := new(TunnelInfo)

		upLinkTunnel := dataPathNode.UpLinkTunnel
		downLinkTunnel := dataPathNode.DownLinkTunnel
		if upLinkTunnel != nilValTunnel {
			// store uLTunnelInfo
			uLTunnelInfo.TEID = upLinkTunnel.TEID
			uLTunnelInfo.PDR = upLinkTunnel.PDR

			// upLinkTunnelDEP := upLinkTunnel.DestEndPoint
			upLinkTunnelSEP := upLinkTunnel.SrcEndPoint
			if upLinkTunnelSEP != nilValDpn {
				uLTunnelInfo.DataPathNodeUPFNodeID = GetNodeIDInDB(upLinkTunnelSEP.UPF.NodeID)
			}
			dataPathNodeInDB.ULTunnelInfo = uLTunnelInfo
		}

		if downLinkTunnel != nilValTunnel {
			dLTunnelInfo.TEID = downLinkTunnel.TEID
			dLTunnelInfo.PDR = downLinkTunnel.PDR

			dlLinkTunnelSEP := downLinkTunnel.SrcEndPoint
			if dlLinkTunnelSEP != nilValDpn {
				dLTunnelInfo.DataPathNodeUPFNodeID = GetNodeIDInDB(dlLinkTunnelSEP.UPF.NodeID)
			}
			dataPathNodeInDB.DLTunnelInfo = dLTunnelInfo
		}
		StoreNodeInDB(dataPathNodeInDB)
		return dataPathNodeInDB
	}
	return nil
}
