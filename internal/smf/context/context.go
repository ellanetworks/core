package context

import (
	"fmt"
	"net"
	"os"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/omec-project/openapi/Nudm_SubscriberDataManagement"
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/smf/factory"
	"github.com/yeastengine/ella/internal/smf/logger"
)

func init() {
	smfContext.NfInstanceID = uuid.New().String()
}

const (
	IPV4 = "IPv4"
)

var smfContext SMFContext

type SMFContext struct {
	Name         string
	NfInstanceID string

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

	// Now only "IPv4" supported
	// TODO: support "IPv6", "IPv4v6", "Ethernet"
	SupportedPDUSessionType string

	UEPreConfigPathPool map[string]*UEPreConfigPaths
	EnterpriseList      *map[string]string // map to contain slice-name:enterprise-name

	PodIp string

	CPNodeID       NodeID
	PFCPPort       int
	UDMProfile     models.NfProfile
	SBIPort        int
	LocalSEIDCount uint64

	// For ULCL
	ULCLSupport bool
}

func AllocateLocalSEID() (uint64, error) {
	atomic.AddUint64(&smfContext.LocalSEIDCount, 1)
	return smfContext.LocalSEIDCount, nil
}

func ReleaseLocalSEID(seid uint64) error {
	return nil
}

func InitSmfContext(config *factory.Config) *SMFContext {
	if config == nil {
		logger.CtxLog.Error("Config is nil")
		return nil
	}

	// Acquire master SMF config lock, no one should update it in parallel,
	// until SMF is done updating SMF context
	factory.SmfConfigSyncLock.Lock()
	defer factory.SmfConfigSyncLock.Unlock()

	logger.CtxLog.Infof("smfconfig Info: Version[%s] Description[%s]", config.Info.Version, config.Info.Description)
	configuration := config.Configuration
	if configuration.SmfName != "" {
		smfContext.Name = configuration.SmfName
	}

	sbi := configuration.Sbi
	smfContext.URIScheme = models.UriScheme_HTTP
	smfContext.SBIPort = configuration.Sbi.Port
	smfContext.SBIPort = sbi.Port
	smfContext.BindingIPv4 = sbi.BindingIPv4

	smfContext.AmfUri = configuration.AmfUri
	smfContext.PcfUri = configuration.PcfUri
	smfContext.UdmUri = configuration.UdmUri

	if pfcp := configuration.PFCP; pfcp != nil {
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
	smfContext.ULCLSupport = configuration.ULCL
	smfContext.SupportedPDUSessionType = IPV4
	smfContext.UserPlaneInformation = NewUserPlaneInformation(&configuration.UserPlaneInformation)
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

	logger.CtxLog.Infof("ue routing config Info: Version[%s] Description[%s]",
		routingConfig.Info.Version, routingConfig.Info.Description)

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

func GetUserPlaneInformation() *UserPlaneInformation {
	return smfContext.UserPlaneInformation
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
