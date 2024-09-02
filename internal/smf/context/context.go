// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/omec-project/openapi/Nnrf_NFDiscovery"
	"github.com/omec-project/openapi/Nnrf_NFManagement"
	"github.com/omec-project/openapi/Nudm_SubscriberDataManagement"
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/drsm"
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

type DrsmCtxts struct {
	TeidPool drsm.DrsmInterface
	SeidPool drsm.DrsmInterface
	UeIpPool drsm.DrsmInterface
}

type SMFContext struct {
	Name         string
	NfInstanceID string

	URIScheme    models.UriScheme
	BindingIPv4  string
	RegisterIPv4 string

	UPNodeIDs []NodeID
	Key       string
	PEM       string
	KeyLog    string

	SnssaiInfos []SnssaiSmfInfo

	NrfUri                         string
	NFManagementClient             *Nnrf_NFManagement.APIClient
	NFDiscoveryClient              *Nnrf_NFDiscovery.APIClient
	SubscriberDataManagementClient *Nudm_SubscriberDataManagement.APIClient

	UserPlaneInformation *UserPlaneInformation

	// Now only "IPv4" supported
	// TODO: support "IPv6", "IPv4v6", "Ethernet"
	SupportedPDUSessionType string

	UEPreConfigPathPool map[string]*UEPreConfigPaths
	DrsmCtxts           DrsmCtxts
	EnterpriseList      *map[string]string // map to contain slice-name:enterprise-name

	NfStatusSubscriptions sync.Map // map[NfInstanceID]models.NrfSubscriptionData.SubscriptionId
	PodIp                 string

	StaticIpInfo             *[]factory.StaticIpInfo
	CPNodeID                 NodeID
	PFCPPort                 int
	UDMProfile               models.NfProfile
	NrfCacheEvictionInterval time.Duration
	SBIPort                  int
	LocalSEIDCount           uint64
	EnableNrfCaching         bool

	// For ULCL
	ULCLSupport bool
}

// RetrieveDnnInformation gets the corresponding dnn info from S-NSSAI and DNN
func RetrieveDnnInformation(Snssai models.Snssai, dnn string) *SnssaiSmfDnnInfo {
	for _, snssaiInfo := range SMF_Self().SnssaiInfos {
		if snssaiInfo.Snssai.Sst == Snssai.Sst && snssaiInfo.Snssai.Sd == Snssai.Sd {
			return snssaiInfo.DnnInfos[dnn]
		}
	}
	return nil
}

func AllocateLocalSEID() (uint64, error) {
	if smfContext.DrsmCtxts.SeidPool == nil {
		return 0, fmt.Errorf("SEID pool is not initialized")
	}
	seid32, err := smfContext.DrsmCtxts.SeidPool.AllocateInt32ID()
	if err != nil {
		logger.CtxLog.Errorf("allocate SEID error: %+v", err)
		return 0, err
	}

	return uint64(seid32), nil
}

func ReleaseLocalSEID(seid uint64) error {
	seid32 := (int32)(seid)
	err := smfContext.DrsmCtxts.SeidPool.ReleaseInt32ID(seid32)
	if err != nil {
		logger.CtxLog.Errorf("allocate SEID error: %+v", err)
		return err
	}
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

	// copy static UE IP Addr config
	smfContext.StaticIpInfo = &configuration.StaticIpInfo

	sbi := configuration.Sbi

	if sbi == nil {
		logger.CtxLog.Errorln("Configuration needs \"sbi\" value")
		return nil
	} else {
		smfContext.URIScheme = models.UriScheme_HTTP
		smfContext.RegisterIPv4 = configuration.Sbi.RegisterIPv4
		smfContext.SBIPort = configuration.Sbi.Port
		if sbi.Port != 0 {
			smfContext.SBIPort = sbi.Port
		}

		smfContext.BindingIPv4 = os.Getenv(sbi.BindingIPv4)
		if smfContext.BindingIPv4 != "" {
			logger.CtxLog.Info("Parsing ServerIPv4 address from ENV Variable.")
		} else {
			smfContext.BindingIPv4 = sbi.BindingIPv4
			if smfContext.BindingIPv4 == "" {
				logger.CtxLog.Warn("Error parsing ServerIPv4 address as string. Using the 0.0.0.0 address as default.")
				smfContext.BindingIPv4 = "0.0.0.0"
			}
		}
	}

	if configuration.NrfUri != "" {
		smfContext.NrfUri = configuration.NrfUri
	} else {
		logger.CtxLog.Warn("NRF Uri is empty! Using localhost as NRF IPv4 address.")
		smfContext.NrfUri = fmt.Sprintf("%s://%s:%d", smfContext.URIScheme, "127.0.0.1", 29510)
	}

	if pfcp := configuration.PFCP; pfcp != nil {
		if pfcp.Port == 0 {
			pfcp.Port = factory.DEFAULT_PFCP_PORT
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

	// Static config
	for _, snssaiInfoConfig := range configuration.SNssaiInfo {
		err := smfContext.insertSmfNssaiInfo(&snssaiInfoConfig)
		if err != nil {
			logger.CtxLog.Warnln(err)
		}
	}

	// Set client and set url
	ManagementConfig := Nnrf_NFManagement.NewConfiguration()
	ManagementConfig.SetBasePath(SMF_Self().NrfUri)
	smfContext.NFManagementClient = Nnrf_NFManagement.NewAPIClient(ManagementConfig)

	NFDiscovryConfig := Nnrf_NFDiscovery.NewConfiguration()
	NFDiscovryConfig.SetBasePath(SMF_Self().NrfUri)
	smfContext.NFDiscoveryClient = Nnrf_NFDiscovery.NewAPIClient(NFDiscovryConfig)

	smfContext.ULCLSupport = configuration.ULCL

	smfContext.SupportedPDUSessionType = IPV4

	smfContext.UserPlaneInformation = NewUserPlaneInformation(&configuration.UserPlaneInformation)

	smfContext.EnableNrfCaching = configuration.EnableNrfCaching

	if configuration.EnableNrfCaching {
		if configuration.NrfCacheEvictionInterval == 0 {
			smfContext.NrfCacheEvictionInterval = time.Duration(900) // 15 mins
		} else {
			smfContext.NrfCacheEvictionInterval = time.Duration(configuration.NrfCacheEvictionInterval)
		}
	}

	smfContext.PodIp = os.Getenv("POD_IP")
	SetupNFProfile(config)

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

func ProcessConfigUpdate() bool {
	logger.CtxLog.Infof("Dynamic config update received [%+v]", factory.UpdatedSmfConfig)

	sendNrfRegistration := false
	// Lets check updated config
	updatedCfg := factory.UpdatedSmfConfig

	// Lets parse through network slice configs first
	if updatedCfg.DelSNssaiInfo != nil {
		for _, slice := range *updatedCfg.DelSNssaiInfo {
			err := SMF_Self().deleteSmfNssaiInfo(&slice)
			if err != nil {
				logger.CtxLog.Errorf("delete network slice [%v] failed: %v", slice, err)
			}
		}
		factory.UpdatedSmfConfig.DelSNssaiInfo = nil
		sendNrfRegistration = true
	}

	if updatedCfg.AddSNssaiInfo != nil {
		for _, slice := range *updatedCfg.AddSNssaiInfo {
			err := SMF_Self().insertSmfNssaiInfo(&slice)
			if err != nil {
				logger.CtxLog.Errorf("insert network slice [%v] failed: %v", slice, err)
			}
		}
		factory.UpdatedSmfConfig.AddSNssaiInfo = nil
		sendNrfRegistration = true
	}

	if updatedCfg.ModSNssaiInfo != nil {
		for _, slice := range *updatedCfg.ModSNssaiInfo {
			err := SMF_Self().updateSmfNssaiInfo(&slice)
			if err != nil {
				logger.CtxLog.Errorf("update network slice [%v] failed: %v", slice, err)
			}
		}
		factory.UpdatedSmfConfig.ModSNssaiInfo = nil
		sendNrfRegistration = true
	}

	// UP Node Links should be deleted before underlying UPFs are deleted
	if updatedCfg.DelLinks != nil {
		for _, link := range *updatedCfg.DelLinks {
			err := GetUserPlaneInformation().DeleteUPNodeLinks(&link)
			if err != nil {
				logger.CtxLog.Errorf("delete UP Node Links failed: %v", err)
			}
		}
		factory.UpdatedSmfConfig.DelLinks = nil
	}

	// Iterate through UserPlane Info
	if updatedCfg.DelUPNodes != nil {
		for name, upf := range *updatedCfg.DelUPNodes {
			err := GetUserPlaneInformation().DeleteSmfUserPlaneNode(name, &upf)
			if err != nil {
				logger.CtxLog.Errorf("delete UP Node [%s] failed: %v", name, err)
			}
		}
		factory.UpdatedSmfConfig.DelUPNodes = nil
	}

	if updatedCfg.AddUPNodes != nil {
		for name, upf := range *updatedCfg.AddUPNodes {
			err := GetUserPlaneInformation().InsertSmfUserPlaneNode(name, &upf)
			if err != nil {
				logger.CtxLog.Errorf("insert UP Node [%s] failed: %v", name, err)
			}
		}
		factory.UpdatedSmfConfig.AddUPNodes = nil
		AllocateUPFID()
		// TODO: allocate UPF ID
	}

	if updatedCfg.ModUPNodes != nil {
		for name, upf := range *updatedCfg.ModUPNodes {
			err := GetUserPlaneInformation().UpdateSmfUserPlaneNode(name, &upf)
			if err != nil {
				logger.CtxLog.Errorf("update UP Node [%s] failed: %v", name, err)
			}
		}
		factory.UpdatedSmfConfig.ModUPNodes = nil
	}

	// Iterate through add UP Node Links info
	// UP Links should be added only after underlying UPFs have been added
	if updatedCfg.AddLinks != nil {
		for _, link := range *updatedCfg.AddLinks {
			err := GetUserPlaneInformation().InsertUPNodeLinks(&link)
			if err != nil {
				logger.CtxLog.Errorf("insert UP Node Links failed: %v", err)
			}
		}
		factory.UpdatedSmfConfig.AddLinks = nil
	}

	// Update Enterprise Info
	SMF_Self().EnterpriseList = updatedCfg.EnterpriseList
	logger.CtxLog.Infof("Dynamic config update, enterprise info [%v] ", *updatedCfg.EnterpriseList)

	// Any time config changes(Slices/UPFs/Links) then reset Default path(Key= nssai+Dnn)
	GetUserPlaneInformation().ResetDefaultUserPlanePath()

	// Send NRF Re-register if Slice info got updated
	if sendNrfRegistration {
		SetupNFProfile(&factory.SmfConfig)
	}

	return sendNrfRegistration
}

func (smfCtxt *SMFContext) InitDrsm() error {
	podname := os.Getenv("HOSTNAME")
	podip := os.Getenv("POD_IP")
	podId := drsm.PodId{PodName: podname, PodInstance: smfCtxt.NfInstanceID, PodIp: podip}
	dbName := "sdcore_smf"
	dbUrl := "mongodb://mongodb-arbiter-headless"

	if factory.SmfConfig.Configuration.Mongodb.Url != "" {
		dbUrl = factory.SmfConfig.Configuration.Mongodb.Url
	}

	if factory.SmfConfig.Configuration.SmfDbName != "" {
		dbName = factory.SmfConfig.Configuration.SmfDbName
	}

	logger.CfgLog.Infof("initialising drsm name [%v]", dbName)

	opt := &drsm.Options{ResIdSize: 24, Mode: drsm.ResourceClient}
	db := drsm.DbInfo{Url: dbUrl, Name: dbName}

	// for local FSEID
	if drsmCtxt, err := drsm.InitDRSM("fseid", podId, db, opt); err == nil {
		smfCtxt.DrsmCtxts.SeidPool = drsmCtxt
	} else {
		return err
	}

	// for local FTEID
	if drsmCtxt, err := drsm.InitDRSM("fteid", podId, db, opt); err == nil {
		smfCtxt.DrsmCtxts.TeidPool = drsmCtxt
	} else {
		return err
	}

	// for IP-Addr
	// TODO, use UPF based allocation for now

	return nil
}

func (smfCtxt *SMFContext) GetDnnStaticIpInfo(dnn string) *factory.StaticIpInfo {
	for _, info := range *smfCtxt.StaticIpInfo {
		if info.Dnn == dnn {
			logger.CfgLog.Debugf("get static ip info for dnn [%s] found [%v]", dnn, info)
			return &info
		}
	}
	return nil
}
