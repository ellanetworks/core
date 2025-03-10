// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

type UserPlaneInformation struct {
	UPNodes       map[string]*UPNode
	UPF           *UPNode
	AccessNetwork map[string]*UPNode
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

func GenerateDataPath(upNode UPNode, smContext *SMContext) *DataPath {
	curDataPathNode := NewDataPathNode()
	curDataPathNode.UPF = upNode.UPF

	dataPath := &DataPath{
		Destination: Destination{
			DestinationIP:   "",
			DestinationPort: "",
			URL:             "",
		},
		FirstDPNode: curDataPathNode,
	}
	return dataPath
}
