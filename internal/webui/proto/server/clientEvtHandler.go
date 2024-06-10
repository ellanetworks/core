package server

import (
	"math/rand"
	"strconv"
	"strings"
	"time"

	protos "github.com/omec-project/config5g/proto/sdcoreConfig"
	"github.com/sirupsen/logrus"
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

func init() {
	clientNFPool = make(map[string]*clientNF)
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	restartCounter = r1.Uint32()
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
	for {
		select {
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
			client.configChanged = false
		}
	}
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
