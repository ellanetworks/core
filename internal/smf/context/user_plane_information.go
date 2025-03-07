// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"fmt"
	"reflect"
)

type UserPlaneInformation struct {
	UPNodes              map[string]*UPNode
	UPF                  *UPNode
	AccessNetwork        map[string]*UPNode
	DefaultUserPlanePath map[string][]*UPNode // DNN to Default Path
}

type UPNodeType string

const (
	UpNodeUPF UPNodeType = "UPF"
	UpNodeAN  UPNodeType = "AN"
)

type UPNode struct {
	UPF    *UPF
	Type   UPNodeType
	NodeID NodeID
	Dnn    string
	Links  []*UPNode
}

type UPPath []*UPNode

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
			URL:             "",
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
		if node.Type == UpNodeAN {
			source = node
			var path []*UPNode
			path, pathExist := getPathBetween(source, destinations[0], visited, selection)
			if pathExist {
				if path[0].Type == UpNodeAN {
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

	for _, snssaiInfo := range upi.UPF.UPF.SNssaiInfos {
		currentSnssai := &snssaiInfo.SNssai
		targetSnssai := selection.SNssai

		if currentSnssai.Equal(targetSnssai) {
			for _, dnnInfo := range snssaiInfo.DnnList {
				if dnnInfo.Dnn == selection.Dnn && dnnInfo.ContainsDNAI(selection.Dnai) {
					upList = append(upList, upi.UPF)
					break
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

			pathTail, pathExist := getPathBetween(nodes, dest, visited, selection)

			if pathExist {
				path = make([]*UPNode, 0)
				path = append(path, cur)

				path = append(path, pathTail...)
				pathExist = true

				return path, pathExist
			}
		}
	}

	return nil, false
}
