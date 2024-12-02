package context

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync/atomic"

	"github.com/omec-project/openapi/Nudm_SubscriberDataManagement"
	"github.com/omec-project/openapi/models"
	nmsModels "github.com/yeastengine/ella/internal/nms/models"
	"github.com/yeastengine/ella/internal/smf/factory"
	"github.com/yeastengine/ella/internal/smf/logger"
)

const IPV4 = "IPv4"

var smfContext SMFContext

type SMFContext struct {
	Name string

	URIScheme   models.UriScheme
	BindingIPv4 string

	UPNodeIDs []NodeID
	Key       string
	PEM       string
	KeyLog    string

	SnssaiInfos []SnssaiSmfInfo

	AmfUri string
	PcfUri string
	UdmUri string

	SubscriberDataManagementClient *Nudm_SubscriberDataManagement.APIClient

	UserPlaneInformation *UserPlaneInformation
	ueIPAllocatorMapping map[string]*IPAllocator

	// Now only "IPv4" supported
	// TODO: support "IPv6", "IPv4v6", "Ethernet"
	SupportedPDUSessionType string

	EnterpriseList *map[string]string // map to contain slice-name:enterprise-name

	PodIp string

	StaticIpInfo   *[]factory.StaticIpInfo
	CPNodeID       NodeID
	PFCPPort       int
	UDMProfile     models.NfProfile
	SBIPort        int
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
		logger.CtxLog.Error("Config is nil")
		return nil
	}

	// Acquire master SMF config lock, no one should update it in parallel,
	// until SMF is done updating SMF context
	factory.SmfConfigSyncLock.Lock()
	defer factory.SmfConfigSyncLock.Unlock()

	smfContext.Name = config.SmfName

	// copy static UE IP Addr config
	smfContext.StaticIpInfo = &config.StaticIpInfo

	sbi := config.Sbi

	smfContext.URIScheme = models.UriScheme_HTTP
	smfContext.SBIPort = sbi.Port
	smfContext.BindingIPv4 = sbi.BindingIPv4

	smfContext.AmfUri = config.AmfUri
	smfContext.PcfUri = config.PcfUri
	smfContext.UdmUri = config.UdmUri

	if pfcp := config.PFCP; pfcp != nil {
		if pfcp.Port == 0 {
			pfcp.Port = factory.SMF_PFCP_PORT
		}
		pfcpAddrEnv := os.Getenv(pfcp.Addr)
		if pfcpAddrEnv != "" {
			logger.CtxLog.Info("Parsing PFCP IPv4 address from ENV variable found.")
			pfcp.Addr = pfcpAddrEnv
		}
		if pfcp.Addr == "" {
			logger.CtxLog.Warn("Error parsing PFCP IPv4 address as string. Using the 0.0.0.0 address as default.")
			pfcp.Addr = "0.0.0.0"
		}
		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", pfcp.Addr, pfcp.Port))
		if err != nil {
			logger.CtxLog.Warnf("PFCP Parse Addr Fail: %v", err)
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

func UpdateSMFContext(networkSlices []nmsModels.Slice, deviceGroups []nmsModels.DeviceGroups) {
	UpdateSnssaiInfo(networkSlices, deviceGroups)
	UpdateUserPlaneInformation(networkSlices, deviceGroups)
	logger.CtxLog.Infof("Updated SMF context")
}

func UpdateSnssaiInfo(networkSlices []nmsModels.Slice, deviceGroups []nmsModels.DeviceGroups) {
	smfSelf := SMF_Self()
	snssaiInfoList := make([]SnssaiSmfInfo, 0)
	for _, networkSlice := range networkSlices {
		plmnID := models.PlmnId{
			Mcc: networkSlice.SiteInfo.Plmn.Mcc,
			Mnc: networkSlice.SiteInfo.Plmn.Mnc,
		}
		sstInt, err := strconv.Atoi(networkSlice.SliceId.Sst)
		if err != nil {
			logger.CtxLog.Errorf("failed to convert sst to int: %v", err)
			return
		}
		snssai := SNssai{
			Sst: int32(sstInt),
			Sd:  networkSlice.SliceId.Sd,
		}
		snssaiInfo := SnssaiSmfInfo{
			Snssai:   snssai,
			PlmnId:   plmnID,
			DnnInfos: make(map[string]*SnssaiSmfDnnInfo),
		}

		for _, deviceGroup := range deviceGroups {
			dnn := deviceGroup.IpDomainExpanded.Dnn
			dnsPrimary := deviceGroup.IpDomainExpanded.DnsPrimary
			mtu := deviceGroup.IpDomainExpanded.Mtu
			alloc, err := GetOrCreateIPAllocator(dnn, deviceGroup.IpDomainExpanded.UeIpPool)
			if err != nil {
				logger.CtxLog.Errorf("failed to get or create IP allocator for DNN %s: %v", dnn, err)
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
	}
	smfSelf.SnssaiInfos = snssaiInfoList
}

// This function is used to get or create IP allocator for a DNN
// There are two issues with it:
// 1. Allocation will restart from the beginning on every restart as it is not persisted
// 2. It is not cleaned up when the DNN is removed
// This issue is tracked through: https://github.com/yeastengine/ella/issues/204
func GetOrCreateIPAllocator(dnn string, cidr string) (*IPAllocator, error) {
	smfSelf := SMF_Self()
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

func BuildUserPlaneInformationFromConfig(networkSlices []nmsModels.Slice, deviceGroups []nmsModels.DeviceGroups) *UserPlaneInformation {
	// check if len of networkSlices is 0
	if len(networkSlices) == 0 {
		logger.CtxLog.Warn("Network slices is empty")
		return nil
	}
	for _, networkSlice := range networkSlices {
		if len(deviceGroups) == 0 {
			logger.CtxLog.Warn("Device groups is empty")
			return nil
		}
		for _, deviceGroup := range deviceGroups {
			dnn := deviceGroup.IpDomainExpanded.Dnn
			intfUpfInfoItem := factory.InterfaceUpfInfoItem{
				InterfaceType:   models.UpInterfaceType_N3,
				Endpoints:       make([]string, 0),
				NetworkInstance: dnn,
			}
			ifaces := []factory.InterfaceUpfInfoItem{}
			ifaces = append(ifaces, intfUpfInfoItem)

			upfNameObj, exists := networkSlice.SiteInfo.Upf["upf-name"]
			if !exists {
				logger.CtxLog.Warnf("Key 'upf-name' does not exist in UPF info")
				continue
			}
			upfPortObj, exists := networkSlice.SiteInfo.Upf["upf-port"]
			if !exists {
				logger.CtxLog.Warnf("Key 'upf-port' does not exist in UPF info")
				continue
			}

			upfName, ok := upfNameObj.(string)
			if !ok {
				logger.CtxLog.Warnf("'upf-name' is not a string, actual type: %T, value: %v", upfNameObj, upfNameObj)
				continue
			}
			upfPortStr, ok := upfPortObj.(string)
			if !ok {
				logger.CtxLog.Warnf("'upf-port' is not a string, actual type: %T, value: %v", upfPortObj, upfPortObj)
				continue
			}

			upfNodeID := NewNodeID(upfName)
			upf := NewUPF(upfNodeID, ifaces)
			sstStr := networkSlice.SliceId.Sst
			sstInt, err := strconv.Atoi(sstStr)
			if err != nil {
				logger.CtxLog.Errorf("Failed to convert sst to int: %v", err)
				continue
			}
			upf.SNssaiInfos = []SnssaiUPFInfo{
				{
					SNssai: SNssai{
						Sst: int32(sstInt),
						Sd:  networkSlice.SliceId.Sd,
					},
					DnnList: []DnnUPFInfoItem{
						{
							Dnn: dnn,
						},
					},
				},
			}

			upfPort, err := strconv.Atoi(upfPortStr)
			if err != nil {
				logger.CtxLog.Errorf("Failed to convert upf port to int: %v", err)
				continue
			}
			upf.Port = uint16(upfPort)

			upfNode := &UPNode{
				Type:   UPNODE_UPF,
				UPF:    upf,
				NodeID: *upfNodeID,
				Links:  make([]*UPNode, 0),
				Port:   uint16(upfPort),
				Dnn:    dnn,
			}
			gnbNode := &UPNode{
				Type:   UPNODE_AN,
				NodeID: *NewNodeID("1.1.1.1"),
				Links:  make([]*UPNode, 0),
				Dnn:    dnn,
			}
			gnbNode.Links = append(gnbNode.Links, upfNode)
			upfNode.Links = append(upfNode.Links, gnbNode)

			userPlaneInformation := &UserPlaneInformation{
				UPNodes:              make(map[string]*UPNode),
				UPF:                  upfNode,
				AccessNetwork:        make(map[string]*UPNode),
				DefaultUserPlanePath: make(map[string][]*UPNode),
			}
			gnbName := networkSlice.SiteInfo.GNodeBs[0].Name
			userPlaneInformation.AccessNetwork[gnbName] = gnbNode
			userPlaneInformation.UPNodes[gnbName] = gnbNode
			userPlaneInformation.UPNodes[upfName] = upfNode
			return userPlaneInformation
		}
	}
	return nil
}

// Right now we only support 1 UPF
// This function should be edited when we decide to support multiple UPFs
func UpdateUserPlaneInformation(networkSlices []nmsModels.Slice, deviceGroups []nmsModels.DeviceGroups) {
	smfSelf := SMF_Self()
	configUserPlaneInfo := BuildUserPlaneInformationFromConfig(networkSlices, deviceGroups)
	same := UserPlaneInfoMatch(configUserPlaneInfo, smfSelf.UserPlaneInformation)
	if same {
		logger.CtxLog.Info("Context user plane info matches config")
		return
	}
	if configUserPlaneInfo == nil {
		logger.CtxLog.Warn("Config user plane info is nil")
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
			logger.CtxLog.Warnf("Node type mismatch for node %s", nodeName)
			return false
		}

		if !bytes.Equal(node.NodeID.NodeIdValue, contextUserPlaneInfo.UPNodes[nodeName].NodeID.NodeIdValue) {
			logger.CtxLog.Warnf("Node ID mismatch for node %s", nodeName)
			return false
		}

		if node.Port != contextUserPlaneInfo.UPNodes[nodeName].Port {
			logger.CtxLog.Warnf("Port mismatch for node %s", nodeName)
			return false
		}

		if node.Dnn != contextUserPlaneInfo.UPNodes[nodeName].Dnn {
			logger.CtxLog.Warnf("DNN mismatch for node %s", nodeName)
			return false
		}

		if node.Type == UPNODE_UPF {
			if !node.UPF.SNssaiInfos[0].SNssai.Equal(&contextUserPlaneInfo.UPNodes[nodeName].UPF.SNssaiInfos[0].SNssai) {
				logger.CtxLog.Warnf("SNssai mismatch for node %s", nodeName)
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
