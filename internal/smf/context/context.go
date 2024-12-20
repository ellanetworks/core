package context

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"sync/atomic"

	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/config"
	"github.com/yeastengine/ella/internal/logger"
	nmsModels "github.com/yeastengine/ella/internal/models"
	"github.com/yeastengine/ella/internal/smf/factory"
)

const IPV4 = "IPv4"

var smfContext SMFContext

type SMFContext struct {
	Name string

	UPNodeIDs []NodeID
	Key       string
	PEM       string
	KeyLog    string

	SnssaiInfos []SnssaiSmfInfo

	UserPlaneInformation *UserPlaneInformation
	ueIPAllocatorMapping map[string]*IPAllocator

	SupportedPDUSessionType string

	EnterpriseList *map[string]string // map to contain slice-name:enterprise-name

	PodIp string

	StaticIpInfo   *[]factory.StaticIpInfo
	CPNodeID       NodeID
	PFCPPort       int
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

func InitSmfContext(config *factory.Configuration) *SMFContext {
	if config == nil {
		logger.SmfLog.Error("Config is nil")
		return nil
	}

	// Acquire master SMF config lock, no one should update it in parallel,
	// until SMF is done updating SMF context
	factory.SmfConfigSyncLock.Lock()
	defer factory.SmfConfigSyncLock.Unlock()

	smfContext.Name = config.SmfName

	// copy static UE IP Addr config
	smfContext.StaticIpInfo = &config.StaticIpInfo

	if pfcp := config.PFCP; pfcp != nil {
		if pfcp.Port == 0 {
			pfcp.Port = factory.SMF_PFCP_PORT
		}
		pfcpAddrEnv := os.Getenv(pfcp.Addr)
		if pfcpAddrEnv != "" {
			logger.SmfLog.Info("Parsing PFCP IPv4 address from ENV variable found.")
			pfcp.Addr = pfcpAddrEnv
		}
		if pfcp.Addr == "" {
			logger.SmfLog.Warn("Error parsing PFCP IPv4 address as string. Using the 0.0.0.0 address as default.")
			pfcp.Addr = "0.0.0.0"
		}
		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", pfcp.Addr, pfcp.Port))
		if err != nil {
			logger.SmfLog.Warnf("PFCP Parse Addr Fail: %v", err)
		}

		smfContext.PFCPPort = int(pfcp.Port)

		smfContext.CPNodeID.NodeIdType = 0
		smfContext.CPNodeID.NodeIdValue = addr.IP.To4()
	}

	smfContext.ULCLSupport = config.ULCL

	smfContext.SupportedPDUSessionType = IPV4

	smfContext.SnssaiInfos = make([]SnssaiSmfInfo, 0)
	smfContext.UserPlaneInformation = &UserPlaneInformation{
		UPNodes:              make(map[string]*UPNode),
		UPF:                  nil,
		AccessNetwork:        make(map[string]*UPNode),
		DefaultUserPlanePath: make(map[string][]*UPNode),
	}

	// InitUserPlaneInformation()
	smfContext.ueIPAllocatorMapping = make(map[string]*IPAllocator)
	smfContext.PodIp = os.Getenv("POD_IP")

	return &smfContext
}

func SMF_Self() *SMFContext {
	return &smfContext
}

func UpdateSMFContext(network *nmsModels.NetworkSlice, deviceGroups []nmsModels.Profile) {
	UpdateSnssaiInfo(network, deviceGroups)
	UpdateUserPlaneInformation(network, deviceGroups)
	logger.SmfLog.Infof("Updated SMF context")
}

func UpdateSnssaiInfo(network *nmsModels.NetworkSlice, deviceGroups []nmsModels.Profile) {
	smfSelf := SMF_Self()
	snssaiInfoList := make([]SnssaiSmfInfo, 0)
	snssaiInfo := SnssaiSmfInfo{
		Snssai: SNssai{
			Sst: network.Sst,
			Sd:  network.Sd,
		},
		PlmnId: models.PlmnId{
			Mcc: network.Mcc,
			Mnc: network.Mnc,
		},
		DnnInfos: make(map[string]*SnssaiSmfDnnInfo),
	}

	for _, deviceGroup := range deviceGroups {
		dnn := deviceGroup.Dnn
		dnsPrimary := deviceGroup.DnsPrimary
		mtu := deviceGroup.Mtu
		alloc, err := GetOrCreateIPAllocator(dnn, deviceGroup.UeIpPool)
		if err != nil {
			logger.SmfLog.Errorf("failed to get or create IP allocator for DNN %s: %v", dnn, err)
			continue
		}
		dnnInfo := SnssaiSmfDnnInfo{
			DNS: DNS{
				IPv4Addr: net.ParseIP(dnsPrimary).To4(),
			},
			MTU:           uint16(mtu),
			UeIPAllocator: alloc,
		}
		snssaiInfo.DnnInfos[dnn] = &dnnInfo
	}
	snssaiInfoList = append(snssaiInfoList, snssaiInfo)
	smfSelf.SnssaiInfos = snssaiInfoList
}

// This function is used to get or create IP allocator for a DNN
// There are two issues with it:
// 1. Allocation will restart from the beginning on every restart as it is not persisted
// 2. It is not cleaned up when the DNN is removed
// This issue is tracked through: https://github.com/yeastengine/ella/issues/204
func GetOrCreateIPAllocator(dnn string, cidr string) (*IPAllocator, error) {
	smfSelf := SMF_Self()
	if smfSelf.ueIPAllocatorMapping == nil {
		logger.SmfLog.Warnf("IP allocator mapping is nil")
		return nil, fmt.Errorf("IP allocator mapping is nil")
	}
	if _, ok := smfSelf.ueIPAllocatorMapping[dnn]; ok {
		return smfSelf.ueIPAllocatorMapping[dnn], nil
	}
	alloc, err := NewIPAllocator(cidr)
	if err != nil {
		return nil, fmt.Errorf("failed to create IP allocator for DNN %s: %v", dnn, err)
	}
	smfSelf.ueIPAllocatorMapping[dnn] = alloc
	return alloc, nil
}

func BuildUserPlaneInformationFromConfig(network *nmsModels.NetworkSlice, profiles []nmsModels.Profile) *UserPlaneInformation {
	if len(profiles) == 0 {
		logger.SmfLog.Warn("Profiles is empty")
		return nil
	}
	intfUpfInfoItem := factory.InterfaceUpfInfoItem{
		InterfaceType:   models.UpInterfaceType_N3,
		Endpoints:       make([]string, 0),
		NetworkInstance: config.DNN,
	}
	ifaces := []factory.InterfaceUpfInfoItem{}
	ifaces = append(ifaces, intfUpfInfoItem)

	upfNodeID := NewNodeID(network.Upf.Name)
	upf := NewUPF(upfNodeID, ifaces)
	upf.SNssaiInfos = []SnssaiUPFInfo{
		{
			SNssai: SNssai{
				Sst: network.Sst,
				Sd:  network.Sd,
			},
			DnnList: []DnnUPFInfoItem{
				{
					Dnn: config.DNN,
				},
			},
		},
	}

	upf.Port = uint16(network.Upf.Port)

	upfNode := &UPNode{
		Type:   UPNODE_UPF,
		UPF:    upf,
		NodeID: *upfNodeID,
		Links:  make([]*UPNode, 0),
		Port:   uint16(network.Upf.Port),
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
	if len(network.GNodeBs) == 0 {
		logger.SmfLog.Debugf("GNodeBs is empty")
		return nil
	}
	gnbName := network.GNodeBs[0].Name
	userPlaneInformation.AccessNetwork[gnbName] = gnbNode
	userPlaneInformation.UPNodes[gnbName] = gnbNode
	userPlaneInformation.UPNodes[network.Upf.Name] = upfNode
	return userPlaneInformation
}

// Right now we only support 1 UPF
// This function should be edited when we decide to support multiple UPFs
func UpdateUserPlaneInformation(networkSlices *nmsModels.NetworkSlice, deviceGroups []nmsModels.Profile) {
	smfSelf := SMF_Self()
	configUserPlaneInfo := BuildUserPlaneInformationFromConfig(networkSlices, deviceGroups)
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
