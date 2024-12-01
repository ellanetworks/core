package context

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"sync/atomic"

	"github.com/omec-project/openapi/Nudm_SubscriberDataManagement"
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/smf/factory"
	"github.com/yeastengine/ella/internal/smf/logger"
	"github.com/yeastengine/ella/internal/webui/configapi"
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

	InitUserPlaneInformation()
	smfContext.ueIPAllocatorMapping = make(map[string]*IPAllocator)
	smfContext.PodIp = os.Getenv("POD_IP")

	return &smfContext
}

func SMF_Self() *SMFContext {
	return &smfContext
}

func GetSnssaiInfo() []SnssaiSmfInfo {
	snssaiInfoList := make([]SnssaiSmfInfo, 0)
	networkSliceNames := configapi.ListNetworkSlices()
	for _, networkSliceName := range networkSliceNames {
		networkSlice := configapi.GetNetworkSliceByName2(networkSliceName)
		plmnID := models.PlmnId{
			Mcc: networkSlice.SiteInfo.Plmn.Mcc,
			Mnc: networkSlice.SiteInfo.Plmn.Mnc,
		}
		sstInt, err := strconv.Atoi(networkSlice.SliceId.Sst)
		if err != nil {
			logger.CtxLog.Errorf("Failed to convert sst to int: %v", err)
			continue
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

		for _, deviceGroupNames := range networkSlice.SiteDeviceGroup {
			deviceGroup := configapi.GetDeviceGroupByName2(deviceGroupNames)
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
	return snssaiInfoList
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

func InitUserPlaneInformation() {
	smfSelf := SMF_Self()
	upfNodeID := NewNodeID("0.0.0.0")
	upfName := "0.0.0.0"
	gnbNodeID := NewNodeID("1.1.1.1")
	gnbName := "dev2-gnbsim"

	intfUpfInfoItem := factory.InterfaceUpfInfoItem{
		InterfaceType:   models.UpInterfaceType_N3,
		Endpoints:       make([]string, 0),
		NetworkInstance: "internet",
	}
	ifaces := []factory.InterfaceUpfInfoItem{}
	ifaces = append(ifaces, intfUpfInfoItem)

	upf := NewUPF(upfNodeID, ifaces)
	upf.SNssaiInfos = []SnssaiUPFInfo{
		{
			SNssai: SNssai{
				Sst: 1,
				Sd:  "102030",
			},
			DnnList: []DnnUPFInfoItem{
				{
					Dnn: "internet",
				},
			},
		},
	}
	upf.Port = 8806
	upf.UPFStatus = AssociatedSetUpSuccess
	upfNode := &UPNode{
		Type:   UPNODE_UPF,
		UPF:    upf,
		NodeID: *upfNodeID,
		Links:  make([]*UPNode, 0),
		Port:   8806,
		Dnn:    "internet",
	}
	userPlaneInformation := &UserPlaneInformation{
		UPNodes:              make(map[string]*UPNode),
		UPF:                  upfNode,
		AccessNetwork:        make(map[string]*UPNode),
		DefaultUserPlanePath: make(map[string][]*UPNode),
	}

	gnbNode := &UPNode{
		Type:   UPNODE_AN,
		NodeID: *gnbNodeID,
		Links:  make([]*UPNode, 0),
		Dnn:    "internet",
		ANIP:   net.ParseIP("1.1.1.1"),
	}
	gnbNode.Links = append(gnbNode.Links, upfNode)
	upfNode.Links = append(upfNode.Links, gnbNode)
	userPlaneInformation.AccessNetwork[gnbName] = gnbNode
	userPlaneInformation.UPNodes[gnbName] = gnbNode
	userPlaneInformation.UPNodes[upfName] = upfNode
	smfSelf.UserPlaneInformation = userPlaneInformation
}

func GetUserPlaneInformation() *UserPlaneInformation {
	return SMF_Self().UserPlaneInformation
}
