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

	// UserPlaneInformation *UserPlaneInformation
	ueIPAllocatorMapping map[string]*IPAllocator

	// Now only "IPv4" supported
	// TODO: support "IPv6", "IPv4v6", "Ethernet"
	SupportedPDUSessionType string

	UEPreConfigPathPool map[string]*UEPreConfigPaths
	EnterpriseList      *map[string]string // map to contain slice-name:enterprise-name

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

	// // Static config
	// for _, snssaiInfoConfig := range config.SNssaiInfo {
	// 	err := smfContext.insertSmfNssaiInfo(&snssaiInfoConfig)
	// 	if err != nil {
	// 		logger.CtxLog.Warnln(err)
	// 	}
	// }

	smfContext.ULCLSupport = config.ULCL

	smfContext.SupportedPDUSessionType = IPV4

	// smfContext.UserPlaneInformation = NewUserPlaneInformation(&config.UserPlaneInformation)
	smfContext.ueIPAllocatorMapping = make(map[string]*IPAllocator)
	smfContext.PodIp = os.Getenv("POD_IP")

	return &smfContext
}

func InitSMFUERouting(routingConfig *factory.RoutingConfig) {
	if !smfContext.ULCLSupport {
		return
	}

	if routingConfig == nil {
		logger.CtxLog.Error("configuration needs the routing config")
		return
	}

	UERoutingInfo := routingConfig.UERoutingInfo
	smfContext.UEPreConfigPathPool = make(map[string]*UEPreConfigPaths)

	for _, routingInfo := range UERoutingInfo {
		supi := routingInfo.SUPI
		uePreConfigPaths, err := NewUEPreConfigPaths(supi, routingInfo.PathList)
		if err != nil {
			logger.CtxLog.Warnln(err)
			continue
		}

		smfContext.UEPreConfigPathPool[supi] = uePreConfigPaths
	}
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

// type UPF struct {
// 	SNssaiInfos        []SnssaiUPFInfo
// 	N3Interfaces       []UPFInterfaceInfo
// 	N9Interfaces       []UPFInterfaceInfo
// 	UPFunctionFeatures *UPFunctionFeatures

// 	pdrPool sync.Map
// 	farPool sync.Map
// 	barPool sync.Map
// 	qerPool sync.Map
// 	// urrPool        sync.Map
// 	pdrIDGenerator *idgenerator.IDGenerator
// 	farIDGenerator *idgenerator.IDGenerator
// 	barIDGenerator *idgenerator.IDGenerator
// 	qerIDGenerator *idgenerator.IDGenerator

// 	RecoveryTimeStamp RecoveryTimeStamp
// 	NodeID            NodeID
// 	UPFStatus         UPFStatus
// 	uuid              uuid.UUID
// 	Port              uint16
// 	NHeartBeat        uint8

// 	// lock
// 	UpfLock sync.RWMutex
// }

func GetUserPlaneInformation() *UserPlaneInformation {
	upfNodeID := NewNodeID("0.0.0.0")
	upfName := "0.0.0.0"
	gnbNodeID := NewNodeID("1.1.1.1")
	gnbName := "1.1.1.1"

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
	upfNode := UPNode{
		Type:   UPNODE_UPF,
		UPF:    upf,
		NodeID: *upfNodeID,
	}
	userPlaneInformation := &UserPlaneInformation{
		UPNodes:              make(map[string]*UPNode),
		UPFs:                 make(map[string]*UPNode),
		AccessNetwork:        make(map[string]*UPNode),
		UPFsIPtoID:           make(map[string]string),
		DefaultUserPlanePath: make(map[string][]*UPNode),
	}
	userPlaneInformation.UPNodes[upfName] = &upfNode
	userPlaneInformation.UPFs[upfName] = &upfNode
	gnbNode := &UPNode{
		Type:   UPNODE_AN,
		NodeID: *gnbNodeID,
	}
	userPlaneInformation.AccessNetwork[gnbName] = gnbNode
	userPlaneInformation.UPNodes[gnbName] = gnbNode

	return userPlaneInformation
}

// func ProcessConfigUpdate() bool {
// 	logger.CtxLog.Infof("Dynamic config update received [%+v]", factory.UpdatedSmfConfig)

// 	// Lets check updated config
// 	updatedCfg := factory.UpdatedSmfConfig

// 	// Lets parse through network slice configs first
// 	if updatedCfg.DelSNssaiInfo != nil {
// 		for _, slice := range *updatedCfg.DelSNssaiInfo {
// 			err := SMF_Self().deleteSmfNssaiInfo(&slice)
// 			if err != nil {
// 				logger.CtxLog.Errorf("delete network slice [%v] failed: %v", slice, err)
// 			}
// 		}
// 		factory.UpdatedSmfConfig.DelSNssaiInfo = nil
// 	}

// 	if updatedCfg.AddSNssaiInfo != nil {
// 		for _, slice := range *updatedCfg.AddSNssaiInfo {
// 			err := SMF_Self().insertSmfNssaiInfo(&slice)
// 			if err != nil {
// 				logger.CtxLog.Errorf("insert network slice [%v] failed: %v", slice, err)
// 			}
// 		}
// 		factory.UpdatedSmfConfig.AddSNssaiInfo = nil
// 	}

// 	if updatedCfg.ModSNssaiInfo != nil {
// 		for _, slice := range *updatedCfg.ModSNssaiInfo {
// 			err := SMF_Self().updateSmfNssaiInfo(&slice)
// 			if err != nil {
// 				logger.CtxLog.Errorf("update network slice [%v] failed: %v", slice, err)
// 			}
// 		}
// 		factory.UpdatedSmfConfig.ModSNssaiInfo = nil
// 	}

// 	// UP Node Links should be deleted before underlying UPFs are deleted
// 	if updatedCfg.DelLinks != nil {
// 		for _, link := range *updatedCfg.DelLinks {
// 			err := GetUserPlaneInformation().DeleteUPNodeLinks(&link)
// 			if err != nil {
// 				logger.CtxLog.Errorf("delete UP Node Links failed: %v", err)
// 			}
// 		}
// 		factory.UpdatedSmfConfig.DelLinks = nil
// 	}

// 	// Iterate through UserPlane Info
// 	if updatedCfg.DelUPNodes != nil {
// 		for name, upf := range *updatedCfg.DelUPNodes {
// 			err := GetUserPlaneInformation().DeleteSmfUserPlaneNode(name, &upf)
// 			if err != nil {
// 				logger.CtxLog.Errorf("delete UP Node [%s] failed: %v", name, err)
// 			}
// 		}
// 		factory.UpdatedSmfConfig.DelUPNodes = nil
// 	}

// 	if updatedCfg.AddUPNodes != nil {
// 		for name, upf := range *updatedCfg.AddUPNodes {
// 			err := GetUserPlaneInformation().InsertSmfUserPlaneNode(name, &upf)
// 			if err != nil {
// 				logger.CtxLog.Errorf("insert UP Node [%s] failed: %v", name, err)
// 			}
// 		}
// 		factory.UpdatedSmfConfig.AddUPNodes = nil
// 		AllocateUPFID()
// 		// TODO: allocate UPF ID
// 	}

// 	if updatedCfg.ModUPNodes != nil {
// 		for name, upf := range *updatedCfg.ModUPNodes {
// 			err := GetUserPlaneInformation().UpdateSmfUserPlaneNode(name, &upf)
// 			if err != nil {
// 				logger.CtxLog.Errorf("update UP Node [%s] failed: %v", name, err)
// 			}
// 		}
// 		factory.UpdatedSmfConfig.ModUPNodes = nil
// 	}

// 	// Iterate through add UP Node Links info
// 	// UP Links should be added only after underlying UPFs have been added
// 	if updatedCfg.AddLinks != nil {
// 		for _, link := range *updatedCfg.AddLinks {
// 			err := GetUserPlaneInformation().InsertUPNodeLinks(&link)
// 			if err != nil {
// 				logger.CtxLog.Errorf("insert UP Node Links failed: %v", err)
// 			}
// 		}
// 		factory.UpdatedSmfConfig.AddLinks = nil
// 	}

// 	// Update Enterprise Info
// 	SMF_Self().EnterpriseList = updatedCfg.EnterpriseList
// 	logger.CtxLog.Infof("Dynamic config update, enterprise info [%v] ", *updatedCfg.EnterpriseList)

// 	// Any time config changes(Slices/UPFs/Links) then reset Default path(Key= nssai+Dnn)
// 	GetUserPlaneInformation().ResetDefaultUserPlanePath()

// 	return true
// }

func (smfCtxt *SMFContext) GetDnnStaticIpInfo(dnn string) *factory.StaticIpInfo {
	for _, info := range *smfCtxt.StaticIpInfo {
		if info.Dnn == dnn {
			logger.CfgLog.Debugf("get static ip info for dnn [%s] found [%v]", dnn, info)
			return &info
		}
	}
	return nil
}
