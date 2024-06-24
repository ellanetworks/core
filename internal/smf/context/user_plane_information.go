package context

import (
	"bytes"
	"fmt"
	"net"
	"reflect"

	"github.com/omec-project/pfcp/pfcpType"
	"github.com/yeastengine/ella/internal/smf/factory"
	"github.com/yeastengine/ella/internal/smf/logger"
)

// UserPlaneInformation store userplane topology
type UserPlaneInformation struct {
	UPNodes              map[string]*UPNode
	UPFs                 map[string]*UPNode
	AccessNetwork        map[string]*UPNode
	UPFIPToName          map[string]string
	UPFsID               map[string]string    // name to id
	UPFsIPtoID           map[string]string    // ip->id table, for speed optimization
	DefaultUserPlanePath map[string][]*UPNode // DNN to Default Path
}

type UPNodeType string

const (
	UPNODE_UPF UPNodeType = "UPF"
	UPNODE_AN  UPNodeType = "AN"
)

// UPNode represent the user plane node topology
type UPNode struct {
	UPF    *UPF
	Type   UPNodeType
	NodeID pfcpType.NodeID
	ANIP   net.IP
	Dnn    string
	Links  []*UPNode
	Port   uint16
}

// UPPath represent User Plane Sequence of this path
type UPPath []*UPNode

func AllocateUPFID() {
	UPFsID := smfContext.UserPlaneInformation.UPFsID
	UPFsIPtoID := smfContext.UserPlaneInformation.UPFsIPtoID

	for upfName, upfNode := range smfContext.UserPlaneInformation.UPFs {
		upfid := upfNode.UPF.UUID()
		upfip := upfNode.NodeID.ResolveNodeIdToIp().String()

		UPFsID[upfName] = upfid
		UPFsIPtoID[upfip] = upfid
	}
}

// NewUserPlaneInformation process the configuration then returns a new instance of UserPlaneInformation
func NewUserPlaneInformation(upTopology *factory.UserPlaneInformation) *UserPlaneInformation {
	userplaneInformation := &UserPlaneInformation{
		UPNodes:              make(map[string]*UPNode),
		UPFs:                 make(map[string]*UPNode),
		AccessNetwork:        make(map[string]*UPNode),
		UPFIPToName:          make(map[string]string),
		UPFsID:               make(map[string]string),
		UPFsIPtoID:           make(map[string]string),
		DefaultUserPlanePath: make(map[string][]*UPNode),
	}

	// Load UP Nodes to SMF
	for name, node := range upTopology.UPNodes {
		userplaneInformation.InsertSmfUserPlaneNode(name, &node)
	}

	// Load UP Node Link config to SMF
	for _, link := range upTopology.Links {
		userplaneInformation.InsertUPNodeLinks(&link)
	}
	return userplaneInformation
}

func (upi *UserPlaneInformation) GetUPFNameByIp(ip string) string {
	return upi.UPFIPToName[ip]
}

func (upi *UserPlaneInformation) GetUPFNodeIDByName(name string) pfcpType.NodeID {
	return upi.UPFs[name].NodeID
}

func (upi *UserPlaneInformation) GetUPFNodeByIP(ip string) *UPNode {
	upfName := upi.GetUPFNameByIp(ip)
	return upi.UPFs[upfName]
}

func (upi *UserPlaneInformation) GetUPFIDByIP(ip string) string {
	return upi.UPFsIPtoID[ip]
}

func (upi *UserPlaneInformation) ResetDefaultUserPlanePath() {
	logger.UPNodeLog.Infof("resetting the default user plane paths [%v]", upi.DefaultUserPlanePath)
	upi.DefaultUserPlanePath = make(map[string][]*UPNode)
}

func (upi *UserPlaneInformation) GetDefaultUserPlanePathByDNN(selection *UPFSelectionParams) (path UPPath) {
	path, pathExist := upi.DefaultUserPlanePath[selection.String()]
	logger.CtxLog.Traceln("In GetDefaultUserPlanePathByDNN")
	logger.CtxLog.Traceln("selection: ", selection.String())
	if pathExist {
		return
	} else {
		pathExist = upi.GenerateDefaultPath(selection)
		if pathExist {
			return upi.DefaultUserPlanePath[selection.String()]
		}
	}
	return nil
}

func (upi *UserPlaneInformation) ExistDefaultPath(dnn string) bool {
	_, exist := upi.DefaultUserPlanePath[dnn]
	return exist
}

func GenerateDataPath(upPath UPPath, smContext *SMContext) *DataPath {
	if len(upPath) < 1 {
		logger.CtxLog.Errorf("Invalid data path")
		return nil
	}
	lowerBound := 0
	upperBound := len(upPath) - 1
	var root *DataPathNode
	var curDataPathNode *DataPathNode
	var prevDataPathNode *DataPathNode

	for idx, upNode := range upPath {
		curDataPathNode = NewDataPathNode()
		curDataPathNode.UPF = upNode.UPF

		if idx == lowerBound {
			root = curDataPathNode
			root.AddPrev(nil)
		}
		if idx == upperBound {
			curDataPathNode.AddNext(nil)
		}
		if prevDataPathNode != nil {
			prevDataPathNode.AddNext(curDataPathNode)
			curDataPathNode.AddPrev(prevDataPathNode)
		}
		prevDataPathNode = curDataPathNode
	}

	dataPath := &DataPath{
		Destination: Destination{
			DestinationIP:   "",
			DestinationPort: "",
			Url:             "",
		},
		FirstDPNode: root,
	}
	return dataPath
}

func (upi *UserPlaneInformation) GenerateDefaultPath(selection *UPFSelectionParams) (pathExist bool) {
	var source *UPNode
	var destinations []*UPNode

	for len(upi.AccessNetwork) == 0 {
		logger.CtxLog.Errorf("There is no AN Node in config file!")
		return false
	}

	destinations = upi.selectMatchUPF(selection)

	if len(destinations) == 0 {
		logger.CtxLog.Errorf("Can't find UPF with DNN[%s] S-NSSAI[sst: %d sd: %s] DNAI[%s]\n", selection.Dnn,
			selection.SNssai.Sst, selection.SNssai.Sd, selection.Dnai)
		return false
	} else {
		logger.CtxLog.Debugf("Found UPF with DNN[%s] S-NSSAI[sst: %d sd: %s] DNAI[%s]\n", selection.Dnn,
			selection.SNssai.Sst, selection.SNssai.Sd, selection.Dnai)
	}

	// Run DFS
	visited := make(map[*UPNode]bool)

	for _, upNode := range upi.UPNodes {
		visited[upNode] = false
	}

	for anName, node := range upi.AccessNetwork {
		if node.Type == UPNODE_AN {
			source = node
			var path []*UPNode
			path, pathExist = getPathBetween(source, destinations[0], visited, selection)

			if pathExist {
				if path[0].Type == UPNODE_AN {
					path = path[1:]
				}
				upi.DefaultUserPlanePath[selection.String()] = path
				break
			} else {
				logger.CtxLog.Debugf("No path between an-node[%v] and upf[%v] ", anName, string(destinations[0].NodeID.NodeIdValue))
				continue
			}
		}
	}

	return pathExist
}

func (upi *UserPlaneInformation) selectMatchUPF(selection *UPFSelectionParams) []*UPNode {
	upList := make([]*UPNode, 0)

	for _, upNode := range upi.UPFs {
		for _, snssaiInfo := range upNode.UPF.SNssaiInfos {
			currentSnssai := &snssaiInfo.SNssai
			targetSnssai := selection.SNssai

			if currentSnssai.Equal(targetSnssai) {
				for _, dnnInfo := range snssaiInfo.DnnList {
					if dnnInfo.Dnn == selection.Dnn && dnnInfo.ContainsDNAI(selection.Dnai) {
						upList = append(upList, upNode)
						break
					}
				}
			}
		}
	}
	return upList
}

func getPathBetween(cur *UPNode, dest *UPNode, visited map[*UPNode]bool,
	selection *UPFSelectionParams,
) (path []*UPNode, pathExist bool) {
	visited[cur] = true

	if reflect.DeepEqual(*cur, *dest) {
		path = make([]*UPNode, 0)
		path = append(path, cur)
		pathExist = true
		return path, pathExist
	}

	selectedSNssai := selection.SNssai

	for _, nodes := range cur.Links {
		if !visited[nodes] {
			if !nodes.UPF.isSupportSnssai(selectedSNssai) {
				visited[nodes] = true
				continue
			}

			path_tail, path_exist := getPathBetween(nodes, dest, visited, selection)

			if path_exist {
				path = make([]*UPNode, 0)
				path = append(path, cur)

				path = append(path, path_tail...)
				pathExist = true

				return path, pathExist
			}
		}
	}

	return nil, false
}

// insert new UPF (only N3)
func (upi *UserPlaneInformation) InsertSmfUserPlaneNode(name string, node *factory.UPNode) error {
	logger.UPNodeLog.Infof("UPNode[%v] to insert, content[%v]\n", name, node)
	logger.UPNodeLog.Debugf("content of map[UPNodes] %v \n", upi.UPNodes)

	upNode := new(UPNode)
	upNode.Type = UPNodeType(node.Type)
	upNode.Port = node.Port
	switch upNode.Type {
	case UPNODE_AN:
		upNode.ANIP = net.ParseIP(node.ANIP)
		upi.AccessNetwork[name] = upNode
	case UPNODE_UPF:
		// ParseIp() always return 16 bytes
		// so we can't use the length of return ip to separate IPv4 and IPv6
		var (
			nodeIdType uint8
			ip         net.IP
		)

		// Find IP
		if ip = net.ParseIP(node.NodeID); ip != nil {
			// v4 or v6
			if ip.To4() != nil {
				// IPv4
				ip = ip.To4()
				nodeIdType = pfcpType.NodeIdTypeIpv4Address
			} else {
				// IPv6
				ip = ip.To16()
				nodeIdType = pfcpType.NodeIdTypeIpv6Address
			}
		} else {
			// FQDN
			nodeIdType = pfcpType.NodeIdTypeFqdn
			ip = []byte(node.NodeID)
		}
		// Populate outcome
		upNode.NodeID = pfcpType.NodeID{
			NodeIdType:  nodeIdType,
			NodeIdValue: []byte(ip),
		}

		upNode.UPF = NewUPF(&upNode.NodeID, node.InterfaceUpfInfoList)
		upNode.UPF.Port = upNode.Port

		snssaiInfos := make([]SnssaiUPFInfo, 0)
		for _, snssaiInfoConfig := range node.SNssaiInfos {
			snssaiInfo := SnssaiUPFInfo{
				SNssai: SNssai{
					Sst: snssaiInfoConfig.SNssai.Sst,
					Sd:  snssaiInfoConfig.SNssai.Sd,
				},
				DnnList: make([]DnnUPFInfoItem, 0),
			}

			for _, dnnInfoConfig := range snssaiInfoConfig.DnnUpfInfoList {
				snssaiInfo.DnnList = append(snssaiInfo.DnnList, DnnUPFInfoItem{
					Dnn:             dnnInfoConfig.Dnn,
					DnaiList:        dnnInfoConfig.DnaiList,
					PduSessionTypes: dnnInfoConfig.PduSessionTypes,
				})
			}
			snssaiInfos = append(snssaiInfos, snssaiInfo)
		}
		upNode.UPF.SNssaiInfos = snssaiInfos
		upi.UPFs[name] = upNode
	default:
		logger.InitLog.Warningf("invalid UPNodeType: %s\n", upNode.Type)
	}

	upi.UPNodes[name] = upNode

	ipStr := upNode.NodeID.ResolveNodeIdToIp().String()
	upi.UPFIPToName[ipStr] = name

	return nil
}

// Update an existing User Plane Node.
// If the node is of type AN, then the node is updated with the new port.
// If the node is of type UPF, then the node is updated with the new port and the new UPF information.
func (upi *UserPlaneInformation) UpdateSmfUserPlaneNode(name string, newNode *factory.UPNode) error {
	logger.UPNodeLog.Infof("UPNode [%v] to update, content[%v]\n", name, newNode)

	existingNode, exists := upi.UPNodes[name]
	if !exists {
		return fmt.Errorf("UPNode [%s] does not exist", name)
	}

	existingNode.Port = newNode.Port

	switch existingNode.Type {
	case UPNODE_AN:
		existingNode.ANIP = net.ParseIP(newNode.ANIP)
	case UPNODE_UPF:

		var nodeIdType uint8

		ip := net.ParseIP(newNode.NodeID)
		if ip == nil {
			nodeIdType = pfcpType.NodeIdTypeFqdn
			ip = []byte(newNode.NodeID)
		} else if ip.To4() != nil {
			nodeIdType = pfcpType.NodeIdTypeIpv4Address
			ip = ip.To4()
		} else {
			nodeIdType = pfcpType.NodeIdTypeIpv6Address
			ip = ip.To16()
		}

		newNodeID := pfcpType.NodeID{
			NodeIdType:  nodeIdType,
			NodeIdValue: []byte(ip),
		}

		if !reflect.DeepEqual(existingNode.NodeID, newNodeID) {
			existingNode.NodeID = newNodeID
			existingNode.UPF = NewUPF(&existingNode.NodeID, newNode.InterfaceUpfInfoList)
		}

		existingNode.UPF.SNssaiInfos = make([]SnssaiUPFInfo, len(newNode.SNssaiInfos))
		for i, snssaiInfoConfig := range newNode.SNssaiInfos {
			existingNode.UPF.SNssaiInfos[i] = SnssaiUPFInfo{
				SNssai: SNssai{
					Sst: snssaiInfoConfig.SNssai.Sst,
					Sd:  snssaiInfoConfig.SNssai.Sd,
				},
				DnnList: make([]DnnUPFInfoItem, len(snssaiInfoConfig.DnnUpfInfoList)),
			}

			for j, dnnInfoConfig := range snssaiInfoConfig.DnnUpfInfoList {
				existingNode.UPF.SNssaiInfos[i].DnnList[j] = DnnUPFInfoItem{
					Dnn:             dnnInfoConfig.Dnn,
					DnaiList:        dnnInfoConfig.DnaiList,
					PduSessionTypes: dnnInfoConfig.PduSessionTypes,
				}
			}
		}
		upi.UPFs[name] = existingNode
	default:
		logger.InitLog.Warningf("invalid UPNodeType: %s\n", existingNode.Type)
	}

	upi.UPNodes[name] = existingNode

	ipStr := existingNode.NodeID.ResolveNodeIdToIp().String()
	upi.UPFIPToName[ipStr] = name

	logger.CtxLog.Infof("UPNode [%s] updated successfully", name)
	return nil
}

// delete UPF
func (upi *UserPlaneInformation) DeleteSmfUserPlaneNode(name string, node *factory.UPNode) error {
	logger.UPNodeLog.Infof("UPNode[%v] to delete, content[%v]\n", name, node)
	logger.UPNodeLog.Debugf("content of map[UPNodes] %v \n", upi.UPNodes)
	// Find UPF node
	upNode := upi.UPNodes[name]

	if upNode == nil {
		upNode = upi.UPNodes[node.NodeID]
		name = node.NodeID
	}

	if upNode != nil {
		switch upNode.Type {
		case UPNODE_AN:
			// Remove from ANPOOL
			logger.UPNodeLog.Debugf("content of map[AccessNetwork] %v \n", upi.AccessNetwork)
			delete(upi.AccessNetwork, name)
		case UPNODE_UPF:
			// remove from UPF pool
			logger.UPNodeLog.Debugf("content of map[UPFs] %v \n", upi.UPFs)
			logger.UPNodeLog.Debugf("content of map[UPFsID] %v \n", upi.UPFsID)
			delete(upi.UPFs, name)
			delete(upi.UPFsID, name)
			// IP to ID map(Host may not be resolvable to IP, so iterate through all entries)
			logger.UPNodeLog.Debugf("content of map[UPFsIPtoID] %v \n", upi.UPFsIPtoID)
			for ipStr, nodeId := range upi.UPFsIPtoID {
				if nodeId == upNode.UPF.UUID() {
					delete(upi.UPFsIPtoID, ipStr)
				}
			}
			// UserPlane UPF pool
			RemoveUPFNodeByNodeID(upNode.NodeID)
			logger.UPNodeLog.Infof("UPNode[%v] deleted from UP-Pool", name)
		default:
			panic("invalid UP Node type")
		}

		// name to upNode map(//Common maps for gNB and UPF)
		logger.UPNodeLog.Debugf("content of map[UPNodes] %v \n", upi.UPNodes)
		delete(upi.UPNodes, name)
		logger.UPNodeLog.Infof("UPNode[%v] deleted from table[UPNodes]", name)

		// IP to name map(Host may not be resolvable to IP, so iterate through all entries)
		logger.UPNodeLog.Debugf("content of map[UPFIPToName] %v \n", upi.UPFIPToName)
		for ipStr, nodeName := range upi.UPFIPToName {
			if nodeName == name {
				delete(upi.UPFIPToName, ipStr)
				logger.UPNodeLog.Infof("UPNode[%v] deleted from table[UPFIPToName]", name)
			}
		}
	}

	// also clean up default paths to UPFs
	return nil
}

func (upi *UserPlaneInformation) InsertUPNodeLinks(link *factory.UPLink) error {
	// Update Links
	logger.UPNodeLog.Infof("inserting UP Node link[%v] ", link)
	logger.UPNodeLog.Debugf("current UP Nodes [%+v]", upi.UPNodes)
	nodeA := upi.UPNodes[link.A]
	nodeB := upi.UPNodes[link.B]
	if nodeA == nil || nodeB == nil {
		logger.UPNodeLog.Warningf("UPLink [%s] <=> [%s] not establish\n", link.A, link.B)
		panic("Invalid UPF Links")
	}
	nodeA.Links = append(nodeA.Links, nodeB)
	nodeB.Links = append(nodeB.Links, nodeA)

	return nil
}

func (upi *UserPlaneInformation) DeleteUPNodeLinks(link *factory.UPLink) error {
	logger.UPNodeLog.Infof("deleting UP Node link[%v] ", link)
	logger.UPNodeLog.Debugf("current UP Nodes [%+v]", upi.UPNodes)

	nodeA := upi.UPNodes[link.A]
	nodeB := upi.UPNodes[link.B]

	// Iterate through node-A links and remove Node-B
	if nodeA != nil {
		for index, upNode := range nodeA.Links {
			if bytes.Equal(upNode.NodeID.NodeIdValue, nodeB.NodeID.NodeIdValue) {
				// skip nodeB from Links
				nodeA.Links = append(nodeA.Links[:index], nodeA.Links[index+1:]...)
				break
			}
		}
	}

	// Iterate through node-B links and remove Node-A
	if nodeB != nil {
		for index, upNode := range nodeB.Links {
			if bytes.Equal(upNode.NodeID.NodeIdValue, nodeA.NodeID.NodeIdValue) {
				// skip nodeA from Links
				nodeB.Links = append(nodeB.Links[:index], nodeB.Links[index+1:]...)
				break
			}
		}
	}

	return nil
}
