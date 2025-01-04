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
	nmsModels "github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/openapi/models"
)

const IPV4 = "IPv4"

var smfContext SMFContext

type StaticIpInfo struct {
	ImsiIpInfo map[string]string
	Dnn        string
}

type InterfaceUpfInfoItem struct {
	NetworkInstance string
	InterfaceType   models.UpInterfaceType
	Endpoints       []string
}

type SMFContext struct {
	Name string

	DbInstance *db.Database

	UPNodeIDs []NodeID
	Key       string
	PEM       string
	KeyLog    string

	SnssaiInfos []SnssaiSmfInfo

	UserPlaneInformation *UserPlaneInformation

	SupportedPDUSessionType string

	EnterpriseList *map[string]string // map to contain slice-name:enterprise-name

	PodIp string

	StaticIpInfo   *[]StaticIpInfo
	CPNodeID       NodeID
	PFCPPort       int
	UpfPfcpPort    int
	UDMProfile     models.NfProfile
	LocalSEIDCount uint64

	// For ULCL
	ULCLSupport bool
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

func ReleaseLocalSEID(seid uint64) error {
	return nil
}

func SMF_Self() *SMFContext {
	return &smfContext
}

func UpdateSMFContext(network *nmsModels.Network, profiles []nmsModels.Profile, radios []nmsModels.Radio) {
	UpdateSnssaiInfo(network, profiles)
	UpdateUserPlaneInformation(profiles, radios)
	logger.SmfLog.Infof("Updated SMF context")
}

func UpdateSnssaiInfo(network *nmsModels.Network, profiles []nmsModels.Profile) {
	smfSelf := SMF_Self()
	snssaiInfoList := make([]SnssaiSmfInfo, 0)
	snssaiInfo := SnssaiSmfInfo{
		Snssai: SNssai{
			Sst: config.Sst,
			Sd:  config.Sd,
		},
		PlmnId: models.PlmnId{
			Mcc: network.Mcc,
			Mnc: network.Mnc,
		},
		DnnInfos: make(map[string]*SnssaiSmfDnnInfo),
	}

	for _, profile := range profiles {
		dnn := config.DNN
		dnsPrimary := profile.Dns
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
	smfSelf.SnssaiInfos = snssaiInfoList
}

func BuildUserPlaneInformationFromConfig(profiles []nmsModels.Profile, radios []nmsModels.Radio) *UserPlaneInformation {
	if len(profiles) == 0 {
		logger.SmfLog.Warn("Profiles not found")
		return nil
	}
	if len(radios) == 0 {
		logger.SmfLog.Debugf("Radios not found")
		return nil
	}
	intfUpfInfoItem := InterfaceUpfInfoItem{
		InterfaceType:   models.UpInterfaceType_N3,
		Endpoints:       make([]string, 0),
		NetworkInstance: config.DNN,
	}
	ifaces := []InterfaceUpfInfoItem{}
	ifaces = append(ifaces, intfUpfInfoItem)

	upfNodeID := NewNodeID(config.UpfName)
	upf := NewUPF(upfNodeID, ifaces)
	upf.SNssaiInfos = []SnssaiUPFInfo{
		{
			SNssai: SNssai{
				Sst: config.Sst,
				Sd:  config.Sd,
			},
			DnnList: []DnnUPFInfoItem{
				{
					Dnn: config.DNN,
				},
			},
		},
	}

	upf.Port = config.UpfPort

	upfNode := &UPNode{
		Type:   UPNODE_UPF,
		UPF:    upf,
		NodeID: *upfNodeID,
		Links:  make([]*UPNode, 0),
		Port:   config.UpfPort,
		Dnn:    config.DNN,
	}
	gnbNode := &UPNode{
		Type:   UPNODE_AN,
		NodeID: *NewNodeID("1.1.1.1"),
		Links:  make([]*UPNode, 0),
		Dnn:    config.DNN,
	}
	gnbNode.Links = append(gnbNode.Links, upfNode)
	upfNode.Links = append(upfNode.Links, gnbNode)

	userPlaneInformation := &UserPlaneInformation{
		UPNodes:              make(map[string]*UPNode),
		UPF:                  upfNode,
		AccessNetwork:        make(map[string]*UPNode),
		DefaultUserPlanePath: make(map[string][]*UPNode),
	}

	gnbName := radios[0].Name
	userPlaneInformation.AccessNetwork[gnbName] = gnbNode
	userPlaneInformation.UPNodes[gnbName] = gnbNode
	userPlaneInformation.UPNodes[config.UpfName] = upfNode
	return userPlaneInformation
}

// Right now we only support 1 UPF
// This function should be edited when we decide to support multiple UPFs
func UpdateUserPlaneInformation(profiles []nmsModels.Profile, radios []nmsModels.Radio) {
	smfSelf := SMF_Self()
	configUserPlaneInfo := BuildUserPlaneInformationFromConfig(profiles, radios)
	same := UserPlaneInfoMatch(configUserPlaneInfo, smfSelf.UserPlaneInformation)
	if same {
		logger.SmfLog.Info("Context user plane info matches config")
		return
	}
	if configUserPlaneInfo == nil {
		logger.SmfLog.Debugf("Config user plane info is nil")
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

		if node.Port != contextUserPlaneInfo.UPNodes[nodeName].Port {
			logger.SmfLog.Warnf("Port mismatch for node %s", nodeName)
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
	return SMF_Self().SnssaiInfos
}
