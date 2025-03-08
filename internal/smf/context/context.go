// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"bytes"
	"net"
	"sync/atomic"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
)

var smfContext SMFContext

type InterfaceUpfInfoItem struct {
	NetworkInstance string
	InterfaceType   models.UpInterfaceType
	Endpoints       []string
}

type SMFContext struct {
	DBInstance           *db.Database
	UserPlaneInformation *UserPlaneInformation
	CPNodeID             NodeID
	LocalSEIDCount       uint64
	ULCLSupport          bool
}

// RetrieveDnnInformation gets the corresponding dnn info from S-NSSAI and DNN
func RetrieveDnnInformation(Snssai models.Snssai, dnn string) *SnssaiSmfDnnInfo {
	snssaiInfo := GetSnssaiInfo()
	for _, snssaiInfo := range snssaiInfo {
		if snssaiInfo.Snssai.Sst == Snssai.Sst && snssaiInfo.Snssai.Sd == Snssai.Sd {
			return snssaiInfo.DnnInfos[dnn]
		}
	}
	return nil
}

func AllocateLocalSEID() (uint64, error) {
	atomic.AddUint64(&smfContext.LocalSEIDCount, 1)
	return smfContext.LocalSEIDCount, nil
}

func SMF_Self() *SMFContext {
	return &smfContext
}

func BuildUserPlaneInformationFromConfig() *UserPlaneInformation {
	smfSelf := SMF_Self()
	operator, err := smfSelf.DBInstance.GetOperator()
	if err != nil {
		logger.SmfLog.Errorf("failed to get operator information from db: %v", err)
		return nil
	}
	intfUpfInfoItem := InterfaceUpfInfoItem{
		InterfaceType:   models.UpInterfaceType_N3,
		Endpoints:       make([]string, 0),
		NetworkInstance: config.DNN,
	}
	ifaces := []InterfaceUpfInfoItem{}
	ifaces = append(ifaces, intfUpfInfoItem)

	upfNodeID := NewNodeID(config.UpfNodeID)
	upf := NewUPF(upfNodeID, ifaces)
	upf.SNssaiInfos = []SnssaiUPFInfo{
		{
			SNssai: SNssai{
				Sst: operator.Sst,
				Sd:  operator.GetHexSd(),
			},
			DnnList: []DnnUPFInfoItem{
				{
					Dnn: config.DNN,
				},
			},
		},
	}

	upfNode := &UPNode{
		Type:   UPNODE_UPF,
		UPF:    upf,
		NodeID: *upfNodeID,
		Links:  make([]*UPNode, 0),
		Dnn:    config.DNN,
	}

	userPlaneInformation := &UserPlaneInformation{
		UPNodes:              make(map[string]*UPNode),
		UPF:                  upfNode,
		AccessNetwork:        make(map[string]*UPNode),
		DefaultUserPlanePath: make(map[string][]*UPNode),
	}

	gnbNode := &UPNode{
		Type:   UPNODE_AN,
		NodeID: *NewNodeID("1.1.1.1"),
		Links:  make([]*UPNode, 0),
		Dnn:    config.DNN,
	}
	gnbNode.Links = append(gnbNode.Links, upfNode)
	upfNode.Links = append(upfNode.Links, gnbNode)
	gnbName := "gnb"
	userPlaneInformation.AccessNetwork[gnbName] = gnbNode
	userPlaneInformation.UPNodes[gnbName] = gnbNode

	userPlaneInformation.UPNodes[config.UpfNodeID] = upfNode
	return userPlaneInformation
}

// Right now we only support 1 UPF
// This function should be edited when we decide to support multiple UPFs
func UpdateUserPlaneInformation() {
	smfSelf := SMF_Self()
	configUserPlaneInfo := BuildUserPlaneInformationFromConfig()
	same := UserPlaneInfoMatch(configUserPlaneInfo, smfSelf.UserPlaneInformation)
	if same {
		logger.SmfLog.Info("Context user plane info matches config")
		return
	}
	if configUserPlaneInfo == nil {
		logger.SmfLog.Debugf("Config user plane info is nil")
		return
	}
	if smfSelf.UserPlaneInformation == nil {
		logger.SmfLog.Warnf("Context user plane info is nil")
		return
	}
	smfSelf.UserPlaneInformation.UPNodes = configUserPlaneInfo.UPNodes
	smfSelf.UserPlaneInformation.UPF = configUserPlaneInfo.UPF
	smfSelf.UserPlaneInformation.AccessNetwork = configUserPlaneInfo.AccessNetwork
	smfSelf.UserPlaneInformation.DefaultUserPlanePath = configUserPlaneInfo.DefaultUserPlanePath
}

func UserPlaneInfoMatch(configUserPlaneInfo, contextUserPlaneInfo *UserPlaneInformation) bool {
	if configUserPlaneInfo == nil || contextUserPlaneInfo == nil {
		return false
	}
	if len(configUserPlaneInfo.UPNodes) != len(contextUserPlaneInfo.UPNodes) {
		return false
	}
	for nodeName, node := range configUserPlaneInfo.UPNodes {
		if _, ok := contextUserPlaneInfo.UPNodes[nodeName]; !ok {
			return false
		}

		if node.Type != contextUserPlaneInfo.UPNodes[nodeName].Type {
			logger.SmfLog.Warnf("Node type mismatch for node %s", nodeName)
			return false
		}

		if !bytes.Equal(node.NodeID.NodeIdValue, contextUserPlaneInfo.UPNodes[nodeName].NodeID.NodeIdValue) {
			logger.SmfLog.Warnf("Node ID mismatch for node %s", nodeName)
			return false
		}

		if node.Dnn != contextUserPlaneInfo.UPNodes[nodeName].Dnn {
			logger.SmfLog.Warnf("DNN mismatch for node %s", nodeName)
			return false
		}

		if node.Type == UPNODE_UPF {
			if !node.UPF.SNssaiInfos[0].SNssai.Equal(&contextUserPlaneInfo.UPNodes[nodeName].UPF.SNssaiInfos[0].SNssai) {
				logger.SmfLog.Warnf("SNssai mismatch for node %s", nodeName)
				return false
			}
		}
	}
	return true
}

func GetUserPlaneInformation() *UserPlaneInformation {
	return SMF_Self().UserPlaneInformation
}

func GetSnssaiInfo() []SnssaiSmfInfo {
	self := SMF_Self()
	operator, err := self.DBInstance.GetOperator()
	if err != nil {
		logger.SmfLog.Warnf("failed to get operator information from db: %v", err)
		return nil
	}
	profiles, err := self.DBInstance.ListProfiles()
	if err != nil {
		logger.SmfLog.Warnf("failed to get profiles from db: %v", err)
		return nil
	}
	snssaiInfoList := make([]SnssaiSmfInfo, 0)
	snssaiInfo := SnssaiSmfInfo{
		Snssai: SNssai{
			Sst: operator.Sst,
			Sd:  operator.GetHexSd(),
		},
		PlmnId: models.PlmnId{
			Mcc: operator.Mcc,
			Mnc: operator.Mnc,
		},
		DnnInfos: make(map[string]*SnssaiSmfDnnInfo),
	}

	for _, profile := range profiles {
		dnn := config.DNN
		dnsPrimary := profile.DNS
		mtu := profile.Mtu
		dnnInfo := SnssaiSmfDnnInfo{
			DNS: DNS{
				IPv4Addr: net.ParseIP(dnsPrimary).To4(),
			},
			MTU: uint16(mtu),
		}
		snssaiInfo.DnnInfos[dnn] = &dnnInfo
	}
	snssaiInfoList = append(snssaiInfoList, snssaiInfo)
	return snssaiInfoList
}
