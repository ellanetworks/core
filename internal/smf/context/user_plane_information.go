package context

import (
	"fmt"
	"net"
	"reflect"

	"github.com/yeastengine/ella/internal/smf/logger"
)

// UserPlaneInformation store userplane topology
type UserPlaneInformation struct {
	UPNodes              map[string]*UPNode
	UPFs                 map[string]*UPNode
	AccessNetwork        map[string]*UPNode
	UPFsIPtoID           map[string]string    // ip->id table, for speed optimization
	DefaultUserPlanePath map[string][]*UPNode // DNN to Default Path
}

type UPNodeType string

const (
	UPNODE_UPF UPNodeType = "UPF"
	UPNODE_AN  UPNodeType = "AN"
)

type UPNode struct {
	UPF    *UPF
	Type   UPNodeType
	NodeID NodeID
	ANIP   net.IP
	Dnn    string
	Links  []*UPNode
	Port   uint16
}

type UPPath []*UPNode

func AllocateUPFID() {
	userPlaneInformation := GetUserPlaneInformation()
	UPFsIPtoID := userPlaneInformation.UPFsIPtoID

	for _, upfNode := range userPlaneInformation.UPFs {
		upfid := upfNode.UPF.UUID()
		upfip := upfNode.NodeID.ResolveNodeIdToIp().String()

		UPFsIPtoID[upfip] = upfid
	}
}

func (upi *UserPlaneInformation) GetUPFNodeIDByName(name string) NodeID {
	return upi.UPFs[name].NodeID
}

func (upi *UserPlaneInformation) GetUPFIDByIP(ip string) string {
	return upi.UPFsIPtoID[ip]
}

func (upi *UserPlaneInformation) ResetDefaultUserPlanePath() {
	logger.UPNodeLog.Infof("resetting the default user plane paths [%v]", upi.DefaultUserPlanePath)
	upi.DefaultUserPlanePath = make(map[string][]*UPNode)
}

func (upi *UserPlaneInformation) GetDefaultUserPlanePathByDNN(selection *UPFSelectionParams) (UPPath, error) {
	path, pathExist := upi.DefaultUserPlanePath[selection.String()]
	if pathExist {
		return path, nil
	}
	err := upi.GenerateDefaultPath(selection)
	if err != nil {
		return nil, fmt.Errorf("couldn't generate default path: %v", err)
	}
	return upi.DefaultUserPlanePath[selection.String()], nil
}

func (upi *UserPlaneInformation) ExistDefaultPath(dnn string) bool {
	_, exist := upi.DefaultUserPlanePath[dnn]
	return exist
}

func GenerateDataPath(upPath UPPath, smContext *SMContext) (*DataPath, error) {
	if len(upPath) < 1 {
		return nil, fmt.Errorf("user plane path is empty")
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
	return dataPath, nil
}

func (upi *UserPlaneInformation) GenerateDefaultPath(selection *UPFSelectionParams) error {
	var source *UPNode
	var destinations []*UPNode

	for len(upi.AccessNetwork) == 0 {
		return fmt.Errorf("user plane information does not contain any AN node")
	}

	destinations = upi.selectMatchUPF(selection)

	if len(destinations) == 0 {
		return fmt.Errorf("can't find UPF with DNN[%s] S-NSSAI[sst: %d sd: %s] DNAI[%s]", selection.Dnn, selection.SNssai.Sst, selection.SNssai.Sd, selection.Dnai)
	}

	// Run DFS
	visited := make(map[*UPNode]bool)

	for _, upNode := range upi.UPNodes {
		visited[upNode] = false
	}

	for _, node := range upi.AccessNetwork {
		if node.Type == UPNODE_AN {
			source = node
			var path []*UPNode
			path, pathExist := getPathBetween(source, destinations[0], visited, selection)
			if pathExist {
				if path[0].Type == UPNODE_AN {
					path = path[1:]
				}
				upi.DefaultUserPlanePath[selection.String()] = path
				break
			} else {
				continue
			}
		}
	}

	return nil
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
	logger.CtxLog.Warnf("getPathBetween: cur: %v, dest: %v, visited: %v, selection: %v", cur, dest, visited, selection)

	if reflect.DeepEqual(*cur, *dest) {
		path = make([]*UPNode, 0)
		path = append(path, cur)
		pathExist = true
		return path, pathExist
	}

	selectedSNssai := selection.SNssai

	for _, nodes := range cur.Links {
		logger.AppLog.Warnf("Nodes in cur.Links: %v", nodes)
		logger.CtxLog.Warnf("getPathBetween: nodes: %v", nodes)
		if !visited[nodes] {
			if !nodes.UPF.isSupportSnssai(selectedSNssai) {
				visited[nodes] = true
				logger.CtxLog.Warnf("getPathBetween: nodes: %v is not support snssai: %v", nodes, selectedSNssai)
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
