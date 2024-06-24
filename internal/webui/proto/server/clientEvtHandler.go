package server

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	protos "github.com/yeastengine/config5g/proto/sdcoreConfig"
	"github.com/yeastengine/ella/internal/webui/backend/factory"
	"github.com/yeastengine/ella/internal/webui/backend/logger"
	"github.com/yeastengine/ella/internal/webui/configmodels"
)

type clientNF struct {
	slicesConfigClient    map[string]*configmodels.Slice
	devgroupsConfigClient map[string]*configmodels.DeviceGroups
	outStandingPushConfig chan *configmodels.ConfigMessage
	tempGrpcReq           chan *clientReqMsg
	resStream             protos.ConfigService_NetworkSliceSubscribeServer
	resChannel            chan bool
	clientLog             *logrus.Entry
	id                    string
	ConfigPushUrl         string
	ConfigCheckUrl        string
	configChanged         bool
	metadataReqtd         bool
}

// message format received from grpc server thread to Client go routine
type clientReqMsg struct {
	networkSliceReqMsg *protos.NetworkSliceRequest
	grpcRspMsg         chan *clientRspMsg
	lastDevGroup       *configmodels.DeviceGroups
	lastSlice          *configmodels.Slice
	devGroup           *configmodels.DeviceGroups
	slice              *configmodels.Slice
	newClient          bool
}

// message format to send response from client go routine to grpc server
type clientRspMsg struct {
	networkSliceRspMsg *protos.NetworkSliceResponse
}

var (
	clientNFPool   map[string]*clientNF
	restartCounter uint32
)

type ServingPlmn struct {
	Mcc int32 `json:"mcc,omitempty"`
	Mnc int32 `json:"mnc,omitempty"`
	Tac int32 `json:"tac,omitempty"`
}

type ImsiRange struct {
	From uint64 `json:"from,omitempty"`
	To   uint64 `json:"to,omitempty"`
}

type selectionKeys struct {
	ServingPlmn  *ServingPlmn `json:"serving-plmn,omitempty"`
	ImsiRange    *ImsiRange   `json:"imsi-range,omitempty"`
	RequestedApn string       `json:"requested-apn,omitempty"`
}

type subSelectionRule struct {
	Keys                     selectionKeys `json:"keys,omitempty"`
	SelectedQoSProfile       string        `json:"selected-qos-profile,omitempty"`
	SelectedUserPlaneProfile string        `json:"selected-user-plane-profile,omitempty"`
	SelectedApnProfile       string        `json:"selected-apn-profile,omitempty"`
	Priority                 int32         `json:"priority,omitempty"`
}

type apnProfile struct {
	DnsPrimary   string `json:"dns_primary,omitempty"`
	DnsSecondary string `json:"dns_secondary,omitempty"`
	ApnName      string `json:"apn-name,omitempty"`
	Mtu          int32  `json:"mtu,omitempty"`
	GxEnabled    bool   `json:"gx_enabled,omitempty"`
}

type userPlaneProfile struct {
	UserPlane     string `json:"user-plane,omitempty"`
	GlobalAddress bool   `json:"global-address,omitempty"`
}

type qosProfile struct {
	Ambr []int32 `json:"apn-ambr,omitempty"`
	Qci  int32   `json:"qci,omitempty"`
	Arp  int32   `json:"arp,omitempty"`
}

type configSpgw struct {
	ApnProfiles       map[string]*apnProfile       `json:"apn-profiles,omitempty"`
	UserPlaneProfiles map[string]*userPlaneProfile `json:"user-plane-profiles,omitempty"`
	QosProfiles       map[string]*qosProfile       `json:"qos-profiles,omitempty"`
	SubSelectRules    []*subSelectionRule          `json:"subscriber-selection-rules,omitempty"`
}

type configHss struct {
	ApnProfiles map[string]*apnProfile `json:"apn-profiles,omitempty"`
	Key         string                 `json:"Key,omitempty"`
	Opc         string                 `json:"Opc,omitempty"`
	Rand        string                 `json:"rand,omitempty"`
	StartImsi   uint64                 `json:"start-imsi,omitempty"`
	EndImsi     uint64                 `json:"end-imsi,omitempty"`
	Sqn         uint64                 `json:"sqn,omitempty"`
	Msisdn      int64                  `json:"msisdn,omitempty"`
	AmbrUl      int32                  `json:"ambr-up,omitempty"`
	AmbrDl      int32                  `json:"ambr-dl,omitempty"`
	Qci         int32                  `json:"qci,omitempty"`
	Arp         int32                  `json:"arp,omitempty"`
}

type ruleFlowInfo struct {
	FlowDesc string `json:"Flow-Description,omitempty"`
	FlowDir  int    `json:"Flow-Direction,omitempty"`
}

type arpInfo struct {
	Priority     int32 `json:"Priority-Level,omitempty"`
	PreEmptCap   int32 `json:"Pre-emption-Capability,omitempty"`
	PreEmpVulner int32 `json:"Pre-emption-Vulnerability,omitempty"`
}

type ruleQosInfo struct {
	Arp       *arpInfo `json:"Allocation-Retention-Priority,omitempty"`
	Qci       int32    `json:"QoS-Class-Identifier,omitempty"`
	Mbr_ul    int32    `json:"Max-Requested-Bandwidth-UL,omitempty"`
	Mbr_dl    int32    `json:"Max-Requested-Bandwidth-DL,omitempty"`
	Gbr_ul    int32    `json:"Guaranteed-Bitrate-UL,omitempty"`
	Gbr_dl    int32    `json:"Guaranteed-Bitrate-DL,omitempty"`
	ApnAmbrUl int32    `json:"APN-Aggregate-Max-Bitrate-UL,omitempty"`
	ApnAmbrDl int32    `json:"APN-Aggregate-Max-Bitrate-DL,omitempty"`
}

type pcrfRuledef struct {
	QosInfo    *ruleQosInfo  `json:"QoS-Information,omitempty"`
	FlowInfo   *ruleFlowInfo `json:"Flow-Information,omitempty"`
	RuleName   string        `json:"Charging-Rule-Name,omitempty"`
	Precedence int32         `json:"Precedence,omitempty"`
	FlowStatus uint32        `json:"Flow-Status,omitempty"`
}

type pcrfRules struct {
	Definitions *pcrfRuledef `json:"definition,omitempty"`
}

type pcrfServices struct {
	Rules                 []string `json:"service-activation-rules,omitempty"`
	Activate_conditions   []string `json:"activate-confitions,omitempty"`
	Deactivate_conditions []string `json:"deactivate-conditions-rules,omitempty"`
	Deactivate_actions    []string `json:"deactivate-actions,omitempty"`
	Qci                   int32    `json:"qci,omitempty"`
	Arp                   int32    `json:"arp,omitempty"`
	Ambr_ul               int32    `json:"AMBR_UL,omitempty"`
	Ambr_dl               int32    `json:"AMBR_DL,omitempty"`
}

type pcrfServiceGroup struct {
	Def_service       []string `json:"default-activate-service,omitempty"`
	OnDemand_services []string `json:"on-demand-service,omitempty"`
}

type PcrfPolicies struct {
	ServiceGroups map[string]*pcrfServiceGroup `json:"service-groups,omitempty"`
	Services      map[string]*pcrfServices     `json:"services,omitempty"`
	Rules         map[string]*pcrfRules        `json:"rules,omitempty"`
}
type configPcrf struct {
	Policies *PcrfPolicies `json:"Policies,omitempty"`
}

type configMme struct {
	PlmnList []string `json:"plmnlist,omitempty"`
}

func init() {
	clientNFPool = make(map[string]*clientNF)
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	restartCounter = r1.Uint32()
}

func setClientConfigPushUrl(client *clientNF, url string) {
	client.ConfigPushUrl = url
}

func setClientConfigCheckUrl(client *clientNF, url string) {
	client.ConfigCheckUrl = url
}

func getClient(id string) (*clientNF, bool) {
	client := clientNFPool[id]
	if client != nil {
		client.clientLog.Infof("Found client %v ", id)
		return client, false
	}
	logger.GrpcLog.Printf("Created client %v ", id)
	client = &clientNF{}
	subField := logrus.Fields{"NF": id}
	client.clientLog = grpcLog.WithFields(subField)
	client.id = id
	client.outStandingPushConfig = make(chan *configmodels.ConfigMessage, 10)
	client.tempGrpcReq = make(chan *clientReqMsg, 10)
	clientNFPool[id] = client
	client.slicesConfigClient = make(map[string]*configmodels.Slice)
	client.devgroupsConfigClient = make(map[string]*configmodels.DeviceGroups)
	// TODO : should we lock global tables before copying them ?
	rwLock.RLock()
	for _, value := range getSlices() {
		client.slicesConfigClient[value.SliceName] = value
	}
	for _, value := range getDeviceGroups() {
		client.devgroupsConfigClient[value.DeviceGroupName] = value
	}
	rwLock.RUnlock()
	go clientEventMachine(client)
	return client, true
}

func fillSite(siteInfoConf *configmodels.SliceSiteInfo, siteInfoProto *protos.SiteInfo) {
	siteInfoProto.SiteName = siteInfoConf.SiteName
	for e := 0; e < len(siteInfoConf.GNodeBs); e++ {
		gnb := siteInfoConf.GNodeBs[e]
		gnbProto := &protos.GNodeB{}
		gnbProto.Name = gnb.Name
		gnbProto.Tac = gnb.Tac
		siteInfoProto.Gnb = append(siteInfoProto.Gnb, gnbProto)
	}
	pl := &protos.PlmnId{}
	pl.Mcc = siteInfoConf.Plmn.Mcc
	pl.Mnc = siteInfoConf.Plmn.Mnc
	siteInfoProto.Plmn = pl

	upf := &protos.UpfInfo{}
	upf.UpfName = siteInfoConf.Upf["upf-name"].(string)
	// TODO panic
	// upf.UpfPort = siteInfoConf.Upf["upf-port"].(uint32)
	siteInfoProto.Upf = upf
}

func fillDeviceGroup(groupName string, devGroupConfig *configmodels.DeviceGroups, devGroupProto *protos.DeviceGroup) {
	devGroupProto.Name = groupName
	ipdomain := &protos.IpDomain{}
	ipdomain.Name = devGroupConfig.IpDomainName
	ipdomain.DnnName = devGroupConfig.IpDomainExpanded.Dnn
	ipdomain.UePool = devGroupConfig.IpDomainExpanded.UeIpPool
	ipdomain.DnsPrimary = devGroupConfig.IpDomainExpanded.DnsPrimary
	ipdomain.Mtu = devGroupConfig.IpDomainExpanded.Mtu
	if devGroupConfig.IpDomainExpanded.UeDnnQos != nil {
		ipdomain.UeDnnQos = &protos.UeDnnQosInfo{}
		ipdomain.UeDnnQos.DnnMbrUplink = devGroupConfig.IpDomainExpanded.UeDnnQos.DnnMbrUplink
		ipdomain.UeDnnQos.DnnMbrDownlink = devGroupConfig.IpDomainExpanded.UeDnnQos.DnnMbrDownlink
		if devGroupConfig.IpDomainExpanded.UeDnnQos.TrafficClass != nil {
			ipdomain.UeDnnQos.TrafficClass = &protos.TrafficClassInfo{}
			ipdomain.UeDnnQos.TrafficClass.Name = devGroupConfig.IpDomainExpanded.UeDnnQos.TrafficClass.Name
			ipdomain.UeDnnQos.TrafficClass.Qci = devGroupConfig.IpDomainExpanded.UeDnnQos.TrafficClass.Qci
			ipdomain.UeDnnQos.TrafficClass.Arp = devGroupConfig.IpDomainExpanded.UeDnnQos.TrafficClass.Arp
			ipdomain.UeDnnQos.TrafficClass.Pdb = devGroupConfig.IpDomainExpanded.UeDnnQos.TrafficClass.Pdb
			ipdomain.UeDnnQos.TrafficClass.Pelr = devGroupConfig.IpDomainExpanded.UeDnnQos.TrafficClass.Pelr
		}
	}

	devGroupProto.IpDomainDetails = ipdomain

	for i := 0; i < len(devGroupConfig.Imsis); i++ {
		devGroupProto.Imsi = append(devGroupProto.Imsi, devGroupConfig.Imsis[i])
	}
}

func fillSlice(client *clientNF, sliceName string, sliceConf *configmodels.Slice, sliceProto *protos.NetworkSlice) bool {
	sliceProto.Name = sliceName
	nssai := &protos.NSSAI{}
	nssai.Sst = sliceConf.SliceId.Sst
	nssai.Sd = sliceConf.SliceId.Sd
	sliceProto.Nssai = nssai

	var defaultQos *configmodels.DeviceGroupsIpDomainExpandedUeDnnQos
	for d := 0; d < len(sliceConf.SiteDeviceGroup); d++ {
		group := sliceConf.SiteDeviceGroup[d]
		client.clientLog.Debugf("group %v, len of devgroupsConfigClient %v ", group, len(client.devgroupsConfigClient))
		devGroupConfig := client.devgroupsConfigClient[group]
		if devGroupConfig == nil {
			client.clientLog.Infof("Did not find group %v ", group)
			return false
		}

		if (defaultQos == nil) && (devGroupConfig.IpDomainExpanded.UeDnnQos != nil) &&
			(devGroupConfig.IpDomainExpanded.UeDnnQos.TrafficClass != nil) {
			defaultQos = &configmodels.DeviceGroupsIpDomainExpandedUeDnnQos{}
			defaultQos.TrafficClass = &configmodels.TrafficClassInfo{}
			defaultQos.TrafficClass.Qci = devGroupConfig.IpDomainExpanded.UeDnnQos.TrafficClass.Qci
			defaultQos.TrafficClass.Arp = devGroupConfig.IpDomainExpanded.UeDnnQos.TrafficClass.Arp
		}

		devGroupProto := &protos.DeviceGroup{}
		fillDeviceGroup(group, devGroupConfig, devGroupProto)
		sliceProto.DeviceGroup = append(sliceProto.DeviceGroup, devGroupProto)
	}
	site := &protos.SiteInfo{}
	sliceProto.Site = site
	fillSite(&sliceConf.SiteInfo, sliceProto.Site)

	// Add Filtering rules
	appFilters := protos.AppFilterRules{
		PccRuleBase: make([]*protos.PccRule, 0),
	}
	for _, ruleConfig := range sliceConf.ApplicationFilteringRules {
		client.clientLog.Debugf("Received Rule config = %v ", ruleConfig)
		pccRule := protos.PccRule{}

		// RuleName
		pccRule.RuleId = ruleConfig.RuleName

		// Rule Precedence
		pccRule.Priority = ruleConfig.Priority

		// Qos Info
		ruleQos := protos.PccRuleQos{}
		ruleQos.MaxbrUl = ruleConfig.AppMbrUplink
		ruleQos.MaxbrDl = ruleConfig.AppMbrDownlink
		ruleQos.GbrUl = 0
		ruleQos.GbrUl = 0

		var arpi, var5qi int32

		if ruleConfig.TrafficClass != nil {
			var5qi = ruleConfig.TrafficClass.Qci
			arpi = ruleConfig.TrafficClass.Arp
		} else if defaultQos != nil {
			var5qi = defaultQos.TrafficClass.Qci
			arpi = defaultQos.TrafficClass.Arp
		} else {
			var5qi = 9
			arpi = 1
		}
		if arpi > 15 {
			arpi = 15
		}

		ruleQos.Var5Qi = var5qi
		arp := &protos.PccArp{}
		arp.PL = arpi
		arp.PC = protos.PccArpPc(1)
		arp.PV = protos.PccArpPv(1)
		ruleQos.Arp = arp
		pccRule.Qos = &ruleQos

		// Flow Info
		// As of now config provides us only single flow
		pccRule.FlowInfos = make([]*protos.PccFlowInfo, 0)
		var desc string
		endp := ruleConfig.Endpoint
		if strings.HasPrefix(endp, "0.0.0.0") {
			endp = "any"
		}
		if ruleConfig.Protocol == int32(protos.PccFlowTos_TCP.Number()) {
			if ruleConfig.StartPort == 0 && ruleConfig.EndPort == 0 {
				desc = "permit out tcp from " + endp + " to assigned"
			} else if factory.WebUIConfig.Configuration.SdfComp {
				desc = "permit out tcp from " + endp + " " + strconv.FormatInt(int64(ruleConfig.StartPort), 10) + "-" + strconv.FormatInt(int64(ruleConfig.EndPort), 10) + " to assigned"
			} else {
				desc = "permit out tcp from " + endp + " to assigned " + strconv.FormatInt(int64(ruleConfig.StartPort), 10) + "-" + strconv.FormatInt(int64(ruleConfig.EndPort), 10)
			}
		} else if ruleConfig.Protocol == int32(protos.PccFlowTos_UDP.Number()) {
			if ruleConfig.StartPort == 0 && ruleConfig.EndPort == 0 {
				desc = "permit out udp from " + endp + " to assigned"
			} else if factory.WebUIConfig.Configuration.SdfComp {
				desc = "permit out udp from " + endp + " " + strconv.FormatInt(int64(ruleConfig.StartPort), 10) + "-" + strconv.FormatInt(int64(ruleConfig.EndPort), 10) + " to assigned"
			} else {
				desc = "permit out udp from " + endp + " to assigned " + strconv.FormatInt(int64(ruleConfig.StartPort), 10) + "-" + strconv.FormatInt(int64(ruleConfig.EndPort), 10)
			}
		} else {
			desc = "permit out ip from " + endp + " to assigned"
		}

		flowInfo := protos.PccFlowInfo{}
		flowInfo.FlowDesc = desc
		flowInfo.TosTrafficClass = "IPV4"
		flowInfo.FlowDir = protos.PccFlowDirection_BIDIRECTIONAL
		if ruleConfig.Action == "deny" {
			flowInfo.FlowStatus = protos.PccFlowStatus_DISABLED
		} else {
			flowInfo.FlowStatus = protos.PccFlowStatus_ENABLED
		}
		pccRule.FlowInfos = append(pccRule.FlowInfos, &flowInfo)

		// Add PCC rule to Rulebase
		appFilters.PccRuleBase = append(appFilters.PccRuleBase, &pccRule)
	}
	// AppFiltering rules not configured, so configuring default rule
	if len(sliceConf.ApplicationFilteringRules) == 0 {
		pccRule := protos.PccRule{}
		// RuleName
		pccRule.RuleId = "DefaultRule"
		// Rule Precedence
		pccRule.Priority = 255
		// Qos Info
		ruleQos := protos.PccRuleQos{}
		ruleQos.Var5Qi = 9
		arp := &protos.PccArp{}
		arp.PL = 1
		arp.PC = protos.PccArpPc(1)
		arp.PV = protos.PccArpPv(1)
		ruleQos.Arp = arp
		pccRule.Qos = &ruleQos
		desc := "permit out ip from any to assigned"

		flowInfo := protos.PccFlowInfo{}
		flowInfo.FlowDesc = desc
		flowInfo.TosTrafficClass = "IPV4"
		flowInfo.FlowDir = protos.PccFlowDirection_BIDIRECTIONAL
		pccRule.FlowInfos = append(pccRule.FlowInfos, &flowInfo)

		appFilters.PccRuleBase = append(appFilters.PccRuleBase, &pccRule)
	}

	// Add to Config to be pushed to client
	if len(appFilters.PccRuleBase) > 0 {
		sliceProto.AppFilters = &appFilters
	}

	return true
}

func clientEventMachine(client *clientNF) {
	ticker := time.NewTicker(10 * time.Second)

	for {
		select {
		case t := <-ticker.C:
			if client.ConfigCheckUrl != "" {
				go func() {
					c := &http.Client{}
					httpend := client.ConfigCheckUrl
					req, err := http.NewRequest(http.MethodPost, httpend, nil)
					if err != nil {
						client.clientLog.Infof("An Error Occurred %v for channel %v \n", err, t)
					}
					resp, err := c.Do(req)
					if err != nil {
						client.clientLog.Infof("An Error Occurred %v\n", err)
					} else {
						err = resp.Body.Close()
						if err != nil {
							client.clientLog.Infof("An Error Occurred %v\n", err)
						}
						if !factory.WebUIConfig.Configuration.Mode5G && resp.StatusCode == http.StatusNotFound {
							client.clientLog.Infof("Config Check Message POST to %v. Status Code -  %v \n", client.id, resp.StatusCode)
							if client.id == "hss" {
								rwLock.RLock()
								postConfigHss(client, nil, nil)
								rwLock.RUnlock()
							} else if client.id == "mme-app" || client.id == "mme-s1ap" {
								postConfigMme(client)
							} else if client.id == "pcrf" {
								postConfigPcrf(client)
							} else if client.id == "spgw" {
								postConfigSpgw(client)
							}
						}
					}
				}()
			}

		case configMsg := <-client.outStandingPushConfig:
			var lastDevGroup *configmodels.DeviceGroups
			var lastSlice *configmodels.Slice

			// update config snapshot
			if configMsg.DevGroup != nil {
				lastDevGroup = client.devgroupsConfigClient[configMsg.DevGroupName]
				client.clientLog.Debugf("Received configuration for device Group  %v ", configMsg.DevGroupName)
				client.devgroupsConfigClient[configMsg.DevGroupName] = configMsg.DevGroup
			} else if configMsg.DevGroupName != "" && configMsg.MsgMethod == configmodels.Delete_op {
				lastDevGroup = client.devgroupsConfigClient[configMsg.DevGroupName]
				client.clientLog.Debugf("Received delete configuration for  Device Group: %v ", configMsg.DevGroupName)
				delete(client.devgroupsConfigClient, configMsg.DevGroupName)
			}

			if configMsg.Slice != nil {
				lastSlice = client.slicesConfigClient[configMsg.SliceName]
				client.clientLog.Infof("Received new configuration for slice %v ", configMsg.SliceName)
				client.slicesConfigClient[configMsg.SliceName] = configMsg.Slice
			} else if configMsg.SliceName != "" && configMsg.MsgMethod == configmodels.Delete_op {
				lastSlice = client.slicesConfigClient[configMsg.SliceName]
				client.clientLog.Infof("Received delete configuration for Slice: %v ", configMsg.SliceName)
				// checking whether the slice is exist or not
				if lastSlice == nil {
					client.clientLog.Warnf("Received non-exist slice: [%v] from Roc/Simapp", configMsg.SliceName)
					continue
				}
				delete(client.slicesConfigClient, configMsg.SliceName)
			}

			client.configChanged = true
			/*If client is attached through stream, then
			  send update to client */
			if client.resStream != nil {
				client.clientLog.Infoln("resStream available")
				var reqMsg clientReqMsg
				var nReq protos.NetworkSliceRequest
				nReq.MetadataRequested = client.metadataReqtd
				reqMsg.networkSliceReqMsg = &nReq
				reqMsg.grpcRspMsg = make(chan *clientRspMsg)
				reqMsg.newClient = false
				reqMsg.lastDevGroup = lastDevGroup
				reqMsg.lastSlice = lastSlice
				reqMsg.devGroup = configMsg.DevGroup
				reqMsg.slice = configMsg.Slice
				client.tempGrpcReq <- &reqMsg
				client.clientLog.Infoln("sent data to client from push config ")
			}
			if !factory.WebUIConfig.Configuration.Mode5G {
				// push config to 4G network functions
				if client.id == "hss" {
					// client.clientLog.Debugf("Received configuration: %v", spew.Sdump(configMsg))
					if configMsg.MsgType == configmodels.Sub_data && configMsg.MsgMethod == configmodels.Delete_op {
						imsiVal := strings.ReplaceAll(configMsg.Imsi, "imsi-", "")
						deleteConfigHss(client, imsiVal)
					} else if configMsg.SliceName != "" && configMsg.MsgMethod == configmodels.Delete_op {
						for _, name := range lastSlice.SiteDeviceGroup {
							if client.devgroupsConfigClient[name] != nil {
								if ok, _ := isDeviceGroupInExistingSlices(client, name); !ok {
									imsis := deletedImsis(client.devgroupsConfigClient[name], nil)
									for _, val := range imsis {
										deleteConfigHss(client, val)
									}
								}
							}
						}
					} else {
						rwLock.RLock()
						postConfigHss(client, lastDevGroup, lastSlice)
						rwLock.RUnlock()
					}
				} else if client.id == "mme-app" || client.id == "mme-s1ap" {
					if (configMsg.SliceName != "") || (configMsg.DevGroupName != "") {
						postConfigMme(client)
					}
				} else if client.id == "pcrf" {
					if (configMsg.SliceName != "") || (configMsg.DevGroupName != "") {
						postConfigPcrf(client)
					}
				} else if client.id == "spgw" {
					if (configMsg.SliceName != "") || (configMsg.DevGroupName != "") {
						postConfigSpgw(client)
					}
				}
			}

		case cReqMsg := <-client.tempGrpcReq:
			client.clientLog.Infof("Config changed %t and NewClient %t\n", client.configChanged, cReqMsg.newClient)

			sliceDetails := &protos.NetworkSliceResponse{}
			sliceDetails.RestartCounter = restartCounter

			envMsg := &clientRspMsg{}
			envMsg.networkSliceRspMsg = sliceDetails

			if !client.configChanged && !cReqMsg.newClient {
				client.clientLog.Infoln("No new update to be sent")
				if client.resStream == nil {
					cReqMsg.grpcRspMsg <- envMsg
				} else {
					if err := client.resStream.Send(
						envMsg.networkSliceRspMsg); err != nil {
						client.clientLog.Infoln("Failed to send data to client: ", err)
						select {
						case client.resChannel <- true:
							client.clientLog.Infoln("Unsubscribed client: ", client.id)
						default:
							// Default case is to avoid blocking in case client has already unsubscribed
						}
					}
				}
				client.clientLog.Infoln("sent data to client: ")
				continue
			}
			client.clientLog.Infof("Send complete snapshoot to client. Number of Network Slices %v ", len(client.slicesConfigClient))
			client.clientLog.Debugf("is client requested for metadata: %v ", client.metadataReqtd)

			// currently pcf request for metadata
			if client.metadataReqtd && !cReqMsg.newClient {
				sliceProto := &protos.NetworkSlice{}
				prevSlice := cReqMsg.lastSlice
				slice := cReqMsg.slice

				// slice Added
				if prevSlice == nil && slice != nil {
					fillSlice(client, slice.SliceName, slice, sliceProto)
					dgnames := getAddedGroupsList(slice, nil)
					for _, dgname := range dgnames {
						aimsis := getAddedImsisList(client.devgroupsConfigClient[dgname], nil)
						sliceProto.AddUpdatedImsis = aimsis
						sliceProto.OperationType = protos.OpType_SLICE_ADD
					}
					sliceDetails.NetworkSlice = append(sliceDetails.NetworkSlice, sliceProto)
				}
				client.clientLog.Infof("PrevSlice Msg: %v", prevSlice)
				client.clientLog.Infof("Slice Msg: %v", slice)

				// slice updated
				if prevSlice != nil && slice != nil {
					client.clientLog.Infof("Slice: %v Updated", slice.SliceName)
					fillSlice(client, slice.SliceName, slice, sliceProto)
					dgnames := getDeleteGroupsList(slice, prevSlice)
					for _, dgname := range dgnames {
						dimsis := getDeletedImsisList(nil, client.devgroupsConfigClient[dgname])
						sliceProto.DeletedImsis = append(sliceProto.DeletedImsis, dimsis...)
						sliceProto.OperationType = protos.OpType_SLICE_UPDATE
					}
					/*dgnames = getAddedGroupsList(slice, prevSlice)
					for _, dgname := range dgnames {
						aimsis := getAddedImsisList(client.devgroupsConfigClient[dgname], nil)
						sliceProto.AddUpdatedImsis = append(sliceProto.AddUpdatedImsis, aimsis...)
						sliceProto.OperationType = protos.OpType_SLICE_UPDATE
					}*/
					//updated other than device group list, adding all imsis because update is required for all
					//if sliceProto.OperationType != protos.OpType_SLICE_UPDATE {
					dgnames = getAddedGroupsList(slice, nil)
					for _, dgname := range dgnames {
						aimsis := getAddedImsisList(client.devgroupsConfigClient[dgname], nil)
						sliceProto.AddUpdatedImsis = append(sliceProto.AddUpdatedImsis, aimsis...)
						sliceProto.OperationType = protos.OpType_SLICE_UPDATE
					}
					//}
					sliceDetails.NetworkSlice = append(sliceDetails.NetworkSlice, sliceProto)
				}
				// slice deleted
				if prevSlice != nil && slice == nil {
					client.clientLog.Infof("Slice: %v Deleted", prevSlice.SliceName)
					fillSlice(client, prevSlice.SliceName, prevSlice, sliceProto)
					dgnames := getDeleteGroupsList(slice, prevSlice)
					for _, dgname := range dgnames {
						dimsis := getDeletedImsisList(nil, client.devgroupsConfigClient[dgname])
						sliceProto.DeletedImsis = dimsis
						sliceProto.OperationType = protos.OpType_SLICE_DELETE
					}
					sliceDetails.NetworkSlice = append(sliceDetails.NetworkSlice, sliceProto)
				}

				// device add: Not Applicable

				// device group updated
				if cReqMsg.devGroup != nil && cReqMsg.lastDevGroup != nil {
					client.clientLog.Infof("PrevDevGroup Msg: %v", cReqMsg.lastDevGroup)
					client.clientLog.Infof("DevGroup Msg: %v", cReqMsg.devGroup)
					name := cReqMsg.devGroup.DeviceGroupName
					if ok, sliceName := isDeviceGroupInExistingSlices(client, name); ok {
						client.clientLog.Infof("DeviceGroup: %v updated, slice of this device group: %v", name, sliceName)
						slice := client.slicesConfigClient[sliceName]
						fillSlice(client, slice.SliceName, slice, sliceProto)
						aimsis := getAddedImsisList(cReqMsg.devGroup, cReqMsg.lastDevGroup)
						sliceProto.AddUpdatedImsis = aimsis
						dimsis := getDeletedImsisList(cReqMsg.devGroup, cReqMsg.lastDevGroup)
						sliceProto.DeletedImsis = dimsis
						sliceProto.OperationType = protos.OpType_SLICE_UPDATE
						sliceDetails.NetworkSlice = append(sliceDetails.NetworkSlice, sliceProto)
					} else {
						client.clientLog.Infof("Device Group: %s is not exist in available slices", name)
						client.configChanged = false
						continue
					}
				}
				// device group deleted
				if cReqMsg.devGroup == nil && cReqMsg.lastDevGroup != nil {
					name := cReqMsg.lastDevGroup.DeviceGroupName
					if ok, sliceName := isDeviceGroupInExistingSlices(client, name); ok {
						client.clientLog.Infof("DeviceGroup: %v deleted, slice of this device group: %v", name, sliceName)
						slice := client.slicesConfigClient[sliceName]
						fillSlice(client, slice.SliceName, slice, sliceProto)
						dimsis := getDeletedImsisList(nil, cReqMsg.lastDevGroup)
						sliceProto.DeletedImsis = dimsis
						sliceProto.OperationType = protos.OpType_SLICE_UPDATE
						sliceDetails.NetworkSlice = append(sliceDetails.NetworkSlice, sliceProto)
					} else {
						client.clientLog.Infof("Device Group: %s is not exist in available slices", name)
						client.configChanged = false
						continue
					}
				}
			} else {
				for sliceName, sliceConfig := range client.slicesConfigClient {
					if sliceConfig == nil {
						continue
					}
					sliceProto := &protos.NetworkSlice{}
					result := fillSlice(client, sliceName, sliceConfig, sliceProto)
					if result {
						sliceDetails.NetworkSlice = append(sliceDetails.NetworkSlice, sliceProto)
					} else {
						client.clientLog.Infoln("Not sending slice config")
					}
				}
			}
			sliceDetails.ConfigUpdated = 1
			if client.resStream == nil {
				cReqMsg.grpcRspMsg <- envMsg
			} else {
				client.clientLog.Infof("sliceDetails: %v", envMsg.networkSliceRspMsg)
				if err := client.resStream.Send(
					envMsg.networkSliceRspMsg); err != nil {
					client.clientLog.Infoln("Failed to send data to client: ", err)
					select {
					case client.resChannel <- true:
						client.clientLog.Infoln("Unsubscribed client: ", client.id)
					default:
						// Default case is to avoid blocking in case client has already unsubscribed
					}
				}
			}
			client.clientLog.Infoln("send slice success")
			client.configChanged = false // TODO RACE CONDITION
		}
	}
}

func postConfigMme(client *clientNF) {
	if len(client.slicesConfigClient) == 0 {
		client.clientLog.Infoln("Not posting config to MME since number of slices: 0")
		return
	}

	client.clientLog.Infoln("Post configuration to MME")
	config := configMme{}

	for sliceName, sliceConfig := range client.slicesConfigClient {
		if sliceConfig == nil {
			continue
		}
		siteInfo := sliceConfig.SiteInfo
		client.clientLog.Infof("Slice %v, siteInfo.GNodeBs %v", sliceName, siteInfo.GNodeBs)

		// keys.ServingPlmn.Tac = gnb.Tac
		plmn := "mcc=" + siteInfo.Plmn.Mcc + ", mnc=" + siteInfo.Plmn.Mnc
		var found bool
		for _, p := range config.PlmnList {
			if p == plmn {
				found = true
			}
		}
		if !found {
			client.clientLog.Infof("Adding plmn for mme %v", plmn)
			config.PlmnList = append(config.PlmnList, plmn)
		}
	}
	client.clientLog.Infoln("Config sending to mme:")
	b, err := json.Marshal(config)
	if err != nil {
		client.clientLog.Infoln("error in marshalling json -", err)
	}

	reqMsgBody := bytes.NewBuffer(b)
	client.clientLog.Debugln("mme reqMsgBody -", reqMsgBody)
	c := &http.Client{}
	httpend := client.ConfigPushUrl
	req, err := http.NewRequest(http.MethodPost, httpend, reqMsgBody)
	if err != nil {
		client.clientLog.Infof("An Error Occurred %v", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := c.Do(req)
	if err != nil {
		client.clientLog.Infof("An Error Occurred %v", err)
	} else {
		err = resp.Body.Close()
		if err != nil {
			client.clientLog.Infof("An Error Occurred %v", err)
		}
		client.clientLog.Infof("mme Message POST %v %v \n", reqMsgBody, resp.StatusCode)
	}
}

func deleteConfigHss(client *clientNF, imsi string) {
	config := configHss{}
	num, err := strconv.ParseInt(imsi, 10, 64)
	if err != nil {
		client.clientLog.Errorf("Could not parse IMSI: %v", imsi)
	}
	config.StartImsi = uint64(num)
	config.EndImsi = uint64(num)
	client.clientLog.Infoln("HSS config ", config)
	b, err := json.Marshal(config)
	if err != nil {
		client.clientLog.Errorln("error in marshalling json -", err)
		return
	}

	client.clientLog.Infof("Deleting SubscriptionData for imsi: %v from HSS", imsi)
	reqMsgBody := bytes.NewBuffer(b)
	client.clientLog.Debugln("reqMsgBody -", reqMsgBody)
	c := &http.Client{}
	httpend := client.ConfigPushUrl
	req, err := http.NewRequest(http.MethodDelete, httpend, reqMsgBody)
	if err != nil {
		client.clientLog.Infof("An Error Occurred %v", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := c.Do(req)
	if err != nil {
		client.clientLog.Infof("An Error Occurred %v", err)
	} else {
		err = resp.Body.Close()
		if err != nil {
			client.clientLog.Infof("An Error Occurred %v", err)
		}
		client.clientLog.Infof("Message DELETE to HSS %v %v Success\n", reqMsgBody, resp.StatusCode)
	}
}

func deletedImsis(prev, curr *configmodels.DeviceGroups) (imsis []string) {
	if curr == nil {
		if prev == nil {
			return
		}
		return prev.Imsis
	}

	if prev == nil {
		return
	}

	for _, pval1 := range prev.Imsis {
		var found bool
		for _, cval2 := range curr.Imsis {
			if pval1 == cval2 {
				found = true
				break
			}
		}
		if !found {
			imsis = append(imsis, pval1)
		}
	}

	return
}

func addedImsis(prev, curr *configmodels.DeviceGroups) (imsis []string) {
	if curr == nil {
		return
	}
	if prev == nil {
		return curr.Imsis
	}

	for _, cval1 := range curr.Imsis {
		var found bool
		for _, pval2 := range prev.Imsis {
			if cval1 == pval2 {
				found = true
				break
			}
		}
		if !found {
			imsis = append(imsis, cval1)
		}
	}

	return
}

func isDeviceGroupInExistingSlices(client *clientNF, name string) (bool, string) {
	for sliceName, sliceConfig := range client.slicesConfigClient {
		for _, dg := range sliceConfig.SiteDeviceGroup {
			if dg == name {
				return true, sliceName
			}
		}
	}

	return false, ""
}

func postConfigHss(client *clientNF, lastDevGroup *configmodels.DeviceGroups, lastSlice *configmodels.Slice) {
	if len(client.slicesConfigClient) == 0 {
		client.clientLog.Infoln("slice config not received yet, not pushing subscriber configuration to HSS.")
		return
	}
	client.clientLog.Infoln("postConfigHss API Enter")

	for sliceName, sliceConfig := range client.slicesConfigClient {
		/* handling of disable devicegroup in slice */
		if lastSlice != nil && lastSlice.SliceId == sliceConfig.SliceId {
			for _, oldG := range lastSlice.SiteDeviceGroup {
				var found bool
				// checking lastSlice.DevGroup is exist in current slice or not
				for _, newG := range sliceConfig.SiteDeviceGroup {
					if oldG == newG {
						found = true
					}
				}

				// devGroup not exist in current but exist in lastSlice
				devGroup := client.devgroupsConfigClient[oldG]
				if !found && devGroup != nil {
					if ok, _ := isDeviceGroupInExistingSlices(client, oldG); !ok {
						imsis := deletedImsis(devGroup, nil)
						client.clientLog.Infoln("DeviceGroup Deleted from Slice: ", oldG)
						for _, val := range imsis {
							deleteConfigHss(client, val)
						}
					}
				}
			}
		}

		for _, d := range sliceConfig.SiteDeviceGroup {
			devGroup := client.devgroupsConfigClient[d]
			if devGroup == nil {
				client.clientLog.Errorf("Device Group [%v] is deleted but bound to slice [%v]: ", d, sliceName)
				continue
			}
			client.clientLog.Infof("Processing DeviceGroup: %v in slice: [%v] ", devGroup, sliceName)
			config := configHss{
				ApnProfiles: make(map[string]*apnProfile),
			}

			// Traffic Class
			// override with device-group specific if available
			if devGroup.IpDomainExpanded.UeDnnQos != nil && devGroup.IpDomainExpanded.UeDnnQos.TrafficClass != nil {
				config.Qci = devGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Qci
				config.Arp = devGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Arp
			}

			// UL AMBR
			// override with device-group specific if available
			if devGroup.IpDomainExpanded.UeDnnQos != nil && devGroup.IpDomainExpanded.UeDnnQos.DnnMbrUplink != 0 {
				config.AmbrUl = int32(devGroup.IpDomainExpanded.UeDnnQos.DnnMbrUplink)
			}

			// DL AMBR
			// override with device-group specific if available
			if devGroup.IpDomainExpanded.UeDnnQos != nil && devGroup.IpDomainExpanded.UeDnnQos.DnnMbrDownlink != 0 {
				config.AmbrDl = int32(devGroup.IpDomainExpanded.UeDnnQos.DnnMbrDownlink)
			}

			var apnProf apnProfile
			apnProf.ApnName = devGroup.IpDomainExpanded.Dnn
			apnProfName := sliceName + "-" + apnProf.ApnName + "-apn"
			config.ApnProfiles[apnProfName] = &apnProf

			var newImsis []string
			if lastDevGroup != nil && lastDevGroup.DeviceGroupName == devGroup.DeviceGroupName {
				// imsi is not present in latest device Group
				delImsis := deletedImsis(lastDevGroup, devGroup)
				client.clientLog.Infoln("Deleted Imsi list from DeviceGroup: ", delImsis)
				for _, val := range delImsis {
					deleteConfigHss(client, val)
				}
				newImsis = addedImsis(lastDevGroup, devGroup)
			} else {
				/* TODO: DG1 exist in slice. now DG2 added to the same slice, below code should hit only for DG2 but
				it hits for DG1 also which lead to adding imsis exist in DG1 to Hss again */
				newImsis = addedImsis(nil, devGroup)
			}

			for _, imsi := range newImsis {
				num, err := strconv.ParseInt(imsi, 10, 64)
				if err != nil {
					client.clientLog.Errorf("Could not parse IMSI %v", imsi)
					continue
				}
				config.StartImsi = uint64(num)
				config.EndImsi = uint64(num)
				authSubsData := imsiData[imsi]
				if authSubsData == nil {
					client.clientLog.Infoln("SIM card details not found for IMSI ", imsi)
					continue
				}
				config.Opc = authSubsData.Opc.OpcValue
				config.Key = authSubsData.PermanentKey.PermanentKeyValue
				num, err = strconv.ParseInt(authSubsData.SequenceNumber, 10, 64)
				if err != nil {
					client.clientLog.Errorf("Could not parse SequenceNumber %v", authSubsData.SequenceNumber)
					continue
				}
				config.Sqn = uint64(num)
				client.clientLog.Infof("Adding SubscritionData for IMSI: %v in HSS ", imsi)
				b, err := json.Marshal(config)
				if err != nil {
					client.clientLog.Errorln("error in marshalling json -", err)
				}

				reqMsgBody := bytes.NewBuffer(b)
				client.clientLog.Debugln("reqMsgBody -", reqMsgBody)
				c := &http.Client{}
				httpend := client.ConfigPushUrl
				req, err := http.NewRequest(http.MethodPost, httpend, reqMsgBody)
				if err != nil {
					client.clientLog.Infof("An Error Occurred %v", err)
				}
				req.Header.Set("Content-Type", "application/json; charset=utf-8")
				resp, err := c.Do(req)
				if err != nil {
					client.clientLog.Infof("An Error Occurred %v", err)
				} else {
					err = resp.Body.Close()
					if err != nil {
						client.clientLog.Infof("An Error Occurred %v", err)
					}
					client.clientLog.Infof("Message POST to HSS %v %v Success\n", reqMsgBody, resp.StatusCode)
				}
			}
			// multiple groups handling?
		}
	}
}

func postConfigPcrf(client *clientNF) {
	if len(client.slicesConfigClient) == 0 {
		client.clientLog.Infoln("DeviceGroup config received, waiting for first slice config.")
		return
	}
	client.clientLog.Infoln("postConfigPcrf API Enter")
	config := configPcrf{}
	config.Policies = &PcrfPolicies{
		ServiceGroups: make(map[string]*pcrfServiceGroup),
		Services:      make(map[string]*pcrfServices),
		Rules:         make(map[string]*pcrfRules),
	}

	for sliceName, sliceConfig := range client.slicesConfigClient {
		if sliceConfig == nil {
			continue
		}
		// siteInfo := sliceConfig.SiteInfo
		// apn profile
		for _, d := range sliceConfig.SiteDeviceGroup {
			devGroup := client.devgroupsConfigClient[d]
			if devGroup == nil {
				client.clientLog.Errorf("Device Group : [%v] doesn't exist in slice [%v]: ", d, sliceName)
				continue
			}
			// client.clientLog.Infoln("PCRF devgroup ", d)
			sgroup := &pcrfServiceGroup{}
			pcrfServiceName := d + "-service"
			sgroup.Def_service = append(sgroup.Def_service, pcrfServiceName)
			config.Policies.ServiceGroups[devGroup.IpDomainExpanded.Dnn] = sgroup
			pcrfService := &pcrfServices{}
			// Traffic Class
			if devGroup.IpDomainExpanded.UeDnnQos != nil && devGroup.IpDomainExpanded.UeDnnQos.TrafficClass != nil {
				pcrfService.Qci = devGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Qci
				pcrfService.Arp = devGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Arp
			}

			// AMBR UL
			if devGroup.IpDomainExpanded.UeDnnQos != nil && devGroup.IpDomainExpanded.UeDnnQos.DnnMbrUplink != 0 {
				pcrfService.Ambr_ul = int32(devGroup.IpDomainExpanded.UeDnnQos.DnnMbrUplink)
			}
			// AMBR DL
			if devGroup.IpDomainExpanded.UeDnnQos != nil && devGroup.IpDomainExpanded.UeDnnQos.DnnMbrDownlink != 0 {
				pcrfService.Ambr_dl = int32(devGroup.IpDomainExpanded.UeDnnQos.DnnMbrDownlink)
			}

			if len(sliceConfig.ApplicationFilteringRules) == 0 {
				app := configmodels.SliceApplicationFilteringRules{RuleName: "rule1", Priority: 1, Action: "permit", Endpoint: "0.0.0.0/0"}
				sliceConfig.ApplicationFilteringRules = append(sliceConfig.ApplicationFilteringRules, app)
			}
			for _, app := range sliceConfig.ApplicationFilteringRules {
				ruleName := d + app.RuleName
				client.clientLog.Infof("rulename: %v, Rules: %v", ruleName, pcrfService.Rules)
				pcrfService.Rules = append(pcrfService.Rules, ruleName)
				// client.clientLog.Infoln("pcrf Service ", pcrfService.Rules)
				config.Policies.Services[pcrfServiceName] = pcrfService
				pcrfRule := &pcrfRules{}
				ruledef := &pcrfRuledef{}
				pcrfRule.Definitions = ruledef
				ruledef.RuleName = ruleName
				ruledef.Precedence = app.Priority
				ruledef.FlowStatus = 3 // disabled by default
				if app.Action == "permit" {
					ruledef.FlowStatus = 2
				}
				ruleQInfo := &ruleQosInfo{}
				ruledef.QosInfo = ruleQInfo
				var arpi int32
				if app.TrafficClass != nil {
					ruleQInfo.Qci = app.TrafficClass.Qci
					arpi = app.TrafficClass.Arp
				} else if devGroup.IpDomainExpanded.UeDnnQos != nil &&
					devGroup.IpDomainExpanded.UeDnnQos.TrafficClass != nil {
					ruleQInfo.Qci = devGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Qci
					arpi = devGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Arp
				} else {
					ruleQInfo.Qci = 9
					arpi = 1
				}
				if arpi > 15 {
					arpi = 15
				}
				ruleQInfo.Mbr_ul = app.AppMbrUplink
				ruleQInfo.Mbr_dl = app.AppMbrDownlink
				ruleQInfo.Gbr_ul = 0
				ruleQInfo.Gbr_dl = 0

				// override with device-group specific if available
				if devGroup.IpDomainExpanded.UeDnnQos != nil && devGroup.IpDomainExpanded.UeDnnQos.DnnMbrUplink != 0 {
					ruleQInfo.ApnAmbrUl = int32(devGroup.IpDomainExpanded.UeDnnQos.DnnMbrUplink)
				}
				if ruleQInfo.Mbr_ul == 0 {
					ruleQInfo.Mbr_ul = ruleQInfo.ApnAmbrUl
				}

				// override with device-group specific if available
				if devGroup.IpDomainExpanded.UeDnnQos != nil && devGroup.IpDomainExpanded.UeDnnQos.DnnMbrDownlink != 0 {
					ruleQInfo.ApnAmbrDl = int32(devGroup.IpDomainExpanded.UeDnnQos.DnnMbrDownlink)
				}
				if ruleQInfo.Mbr_dl == 0 {
					ruleQInfo.Mbr_dl = ruleQInfo.ApnAmbrDl
				}
				arp := &arpInfo{}
				arp.Priority = arpi
				arp.PreEmptCap = 1
				arp.PreEmpVulner = 1
				ruleQInfo.Arp = arp
				ruleFInfo := &ruleFlowInfo{}
				// permit out udp from 8.8.8.8/32 to assigned sport-dport
				var desc string
				if app.Protocol == 6 {
					if app.StartPort == 0 && app.EndPort == 0 {
						desc = "permit out tcp from " + app.Endpoint + " to assigned"
					} else if factory.WebUIConfig.Configuration.SdfComp {
						desc = "permit out tcp from " + app.Endpoint + " " + strconv.FormatInt(int64(app.StartPort), 10) + "-" + strconv.FormatInt(int64(app.EndPort), 10) + " to assigned "
					} else {
						desc = "permit out tcp from " + app.Endpoint + " to assigned " + strconv.FormatInt(int64(app.StartPort), 10) + "-" + strconv.FormatInt(int64(app.EndPort), 10)
					}
				} else if app.Protocol == 17 {
					if app.StartPort == 0 && app.EndPort == 0 {
						desc = "permit out udp from " + app.Endpoint + " to assigned"
					} else if factory.WebUIConfig.Configuration.SdfComp {
						desc = "permit out udp from " + app.Endpoint + " " + strconv.FormatInt(int64(app.StartPort), 10) + "-" + strconv.FormatInt(int64(app.EndPort), 10) + " to assigned "
					} else {
						desc = "permit out udp from " + app.Endpoint + " to assigned " + strconv.FormatInt(int64(app.StartPort), 10) + "-" + strconv.FormatInt(int64(app.EndPort), 10)
					}
				} else {
					desc = "permit out ip from " + app.Endpoint + " to assigned"
				}
				ruleFInfo.FlowDesc = desc
				ruleFInfo.FlowDir = 3
				ruledef.FlowInfo = ruleFInfo
				config.Policies.Rules[ruleName] = pcrfRule
			}
		}
	}

	client.clientLog.Infoln("PCRF Config after filling details ", config)
	client.clientLog.Infoln("PCRF Config after filling details ", config.Policies)
	b, err := json.Marshal(config)
	if err != nil {
		client.clientLog.Infoln("PCRF error in marshalling json -", err)
	}

	reqMsgBody := bytes.NewBuffer(b)
	client.clientLog.Debugln("PCRF reqMsgBody -", reqMsgBody)
	c := &http.Client{}
	httpend := client.ConfigPushUrl
	req, err := http.NewRequest(http.MethodPost, httpend, reqMsgBody)
	if err != nil {
		client.clientLog.Infof("An Error Occurred %v", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := c.Do(req)
	if err != nil {
		client.clientLog.Infof("An Error Occurred %v", err)
	} else {
		err = resp.Body.Close()
		if err != nil {
			client.clientLog.Infof("An Error Occurred %v", err)
		}
		client.clientLog.Infof("PCRF Message POST %v %v Success\n", reqMsgBody, resp.StatusCode)
	}
}

func postConfigSpgw(client *clientNF) {
	if len(client.slicesConfigClient) == 0 {
		client.clientLog.Infoln("DeviceGroup config received, waiting for first slice config.")
		return
	}
	client.clientLog.Infoln("postConfigSpgw API Enter")
	config := configSpgw{
		ApnProfiles:       make(map[string]*apnProfile),
		UserPlaneProfiles: make(map[string]*userPlaneProfile),
		QosProfiles:       make(map[string]*qosProfile),
	}

	for sliceName, sliceConfig := range client.slicesConfigClient {
		if sliceConfig == nil {
			continue
		}
		siteInfo := sliceConfig.SiteInfo
		client.clientLog.Infof("slice: %v, siteInfo.GNodeBs %v", sliceName, siteInfo.GNodeBs)
		for _, d := range sliceConfig.SiteDeviceGroup {
			devGroup := client.devgroupsConfigClient[d]
			if devGroup == nil {
				client.clientLog.Errorln("Device Group is not exist: ", d)
				continue
			}
			var rule subSelectionRule
			rule.Priority = 1
			var apnProf apnProfile
			apnProf.DnsPrimary = devGroup.IpDomainExpanded.DnsPrimary
			apnProf.DnsSecondary = devGroup.IpDomainExpanded.DnsSecondary
			apnProf.ApnName = devGroup.IpDomainExpanded.Dnn
			apnProf.Mtu = devGroup.IpDomainExpanded.Mtu
			apnProf.GxEnabled = false
			apnProfName := sliceName + "-" + apnProf.ApnName + "-apn"
			config.ApnProfiles[apnProfName] = &apnProf
			rule.SelectedApnProfile = apnProfName

			// user plane profile
			var upProf userPlaneProfile
			userProfName := sliceName + "_up"
			upProf.UserPlane = siteInfo.Upf["upf-name"].(string)
			upProf.GlobalAddress = true
			config.UserPlaneProfiles[userProfName] = &upProf
			rule.SelectedUserPlaneProfile = userProfName

			// qos profile
			qosProfName := sliceName + "_qos"
			var qosProf qosProfile
			if (devGroup.IpDomainExpanded.UeDnnQos != nil) &&
				(devGroup.IpDomainExpanded.UeDnnQos.TrafficClass != nil) {
				qosProf.Qci = devGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Qci
				qosProf.Arp = devGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Arp
				qosProf.Ambr = append(qosProf.Ambr, int32(devGroup.IpDomainExpanded.UeDnnQos.DnnMbrUplink))
				qosProf.Ambr = append(qosProf.Ambr, int32(devGroup.IpDomainExpanded.UeDnnQos.DnnMbrDownlink))
			}

			config.QosProfiles[qosProfName] = &qosProf
			rule.SelectedQoSProfile = qosProfName

			var key selectionKeys
			key.RequestedApn = devGroup.IpDomainExpanded.Dnn
			rule.Keys = key
			config.SubSelectRules = append(config.SubSelectRules, &rule)
		}
	}
	client.clientLog.Infoln("spgw Config after filling details ", config)
	b, err := json.Marshal(config)
	if err != nil {
		client.clientLog.Infoln("error in marshalling json -", err)
	}

	reqMsgBody := bytes.NewBuffer(b)
	client.clientLog.Debugln("spgw reqMsgBody -", reqMsgBody)
	c := &http.Client{}
	httpend := client.ConfigPushUrl
	req, err := http.NewRequest(http.MethodPost, httpend, reqMsgBody)
	if err != nil {
		client.clientLog.Infof("An Error Occurred %v", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := c.Do(req)
	if err != nil {
		client.clientLog.Infof("An Error Occurred %v", err)
	} else {
		err = resp.Body.Close()
		if err != nil {
			client.clientLog.Infof("An Error Occurred %v", err)
		}
		client.clientLog.Infof("spgw Message POST %v %v Success\n", reqMsgBody, resp.StatusCode)
	}
}
