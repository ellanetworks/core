package factory

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/omec-project/openapi/models"
	logger_util "github.com/omec-project/util/logger"
	protos "github.com/yeastengine/config5g/proto/sdcoreConfig"
	"github.com/yeastengine/ella/internal/smf/logger"
)

type Config struct {
	Info          *Info               `yaml:"info"`
	Configuration *Configuration      `yaml:"configuration"`
	Logger        *logger_util.Logger `yaml:"logger"`
}

type UpdateSmfConfig struct {
	DelSNssaiInfo  *[]SnssaiInfoItem
	ModSNssaiInfo  *[]SnssaiInfoItem
	AddSNssaiInfo  *[]SnssaiInfoItem
	DelUPNodes     *map[string]UPNode
	ModUPNodes     *map[string]UPNode
	AddUPNodes     *map[string]UPNode
	AddLinks       *[]UPLink
	DelLinks       *[]UPLink
	EnterpriseList *map[string]string
}

type Info struct {
	Version     string `yaml:"version,omitempty"`
	Description string `yaml:"description,omitempty"`
}

const (
	SMF_PFCP_PORT = 8805
	UPF_PFCP_PORT = 8806
)

type Configuration struct {
	PFCP                 *PFCP                `yaml:"pfcp,omitempty"`
	Sbi                  *Sbi                 `yaml:"sbi,omitempty"`
	AmfUri               string               `yaml:"amfUri,omitempty"`
	PcfUri               string               `yaml:"pcfUri,omitempty"`
	UdmUri               string               `yaml:"udmUri,omitempty"`
	WebuiUri             string               `yaml:"webuiUri"`
	SmfName              string               `yaml:"smfName,omitempty"`
	SNssaiInfo           []SnssaiInfoItem     `yaml:"snssaiInfos,omitempty"`
	StaticIpInfo         []StaticIpInfo       `yaml:"staticIpInfo"`
	ServiceNameList      []string             `yaml:"serviceNameList,omitempty"`
	EnterpriseList       map[string]string    `yaml:"enterpriseList,omitempty"`
	UserPlaneInformation UserPlaneInformation `yaml:"userplane_information"`
	ULCL                 bool                 `yaml:"ulcl,omitempty"`
}

type StaticIpInfo struct {
	ImsiIpInfo map[string]string `yaml:"imsiIpInfo"`
	Dnn        string            `yaml:"dnn"`
}

type SnssaiInfoItem struct {
	SNssai   *models.Snssai      `yaml:"sNssai"`
	PlmnId   models.PlmnId       `yaml:"plmnId"`
	DnnInfos []SnssaiDnnInfoItem `yaml:"dnnInfos"`
}

type SnssaiDnnInfoItem struct {
	Dnn      string `yaml:"dnn"`
	DNS      DNS    `yaml:"dns"`
	UESubnet string `yaml:"ueSubnet"`
	MTU      uint16 `yaml:"mtu"`
}

type Sbi struct {
	BindingIPv4 string `yaml:"bindingIPv4,omitempty"` // IP used to run the server in the node.
	Port        int    `yaml:"port,omitempty"`
}

type PFCP struct {
	Addr string `yaml:"addr,omitempty"`
	Port uint16 `yaml:"port,omitempty"`
}

type DNS struct {
	IPv4Addr string `yaml:"ipv4,omitempty"`
	IPv6Addr string `yaml:"ipv6,omitempty"`
}

type Path struct {
	DestinationIP   string   `yaml:"DestinationIP,omitempty"`
	DestinationPort string   `yaml:"DestinationPort,omitempty"`
	UPF             []string `yaml:"UPF,omitempty"`
}

type UERoutingInfo struct {
	SUPI     string `yaml:"SUPI,omitempty"`
	AN       string `yaml:"AN,omitempty"`
	PathList []Path `yaml:"PathList,omitempty"`
}

// RouteProfID is string providing a Route Profile identifier.
type RouteProfID string

// RouteProfile maintains the mapping between RouteProfileID and ForwardingPolicyID of UPF
type RouteProfile struct {
	// Forwarding Policy ID of the route profile
	ForwardingPolicyID string `yaml:"forwardingPolicyID,omitempty"`
}

// PfdContent represents the flow of the application
type PfdContent struct {
	// Identifies a PFD of an application identifier.
	PfdID string `yaml:"pfdID,omitempty"`
	// Represents a 3-tuple with protocol, server ip and server port for
	// UL/DL application traffic.
	FlowDescriptions []string `yaml:"flowDescriptions,omitempty"`
	// Indicates a URL or a regular expression which is used to match the
	// significant parts of the URL.
	Urls []string `yaml:"urls,omitempty"`
	// Indicates an FQDN or a regular expression as a domain name matching
	// criteria.
	DomainNames []string `yaml:"domainNames,omitempty"`
}

// PfdDataForApp represents the PFDs for an application identifier
type PfdDataForApp struct {
	// Caching time for an application identifier.
	CachingTime *time.Time `yaml:"cachingTime,omitempty"`
	// Identifier of an application.
	AppID string `yaml:"applicationId"`
	// PFDs for the application identifier.
	Pfds []PfdContent `yaml:"pfds"`
}

type RoutingConfig struct {
	Info          *Info                        `yaml:"info"`
	UERoutingInfo []*UERoutingInfo             `yaml:"ueRoutingInfo"`
	RouteProf     map[RouteProfID]RouteProfile `yaml:"routeProfile,omitempty"`
	PfdDatas      []*PfdDataForApp             `yaml:"pfdDataForApp,omitempty"`
}

// UserPlaneInformation describe core network userplane information
type UserPlaneInformation struct {
	UPNodes map[string]UPNode `yaml:"up_nodes"`
	Links   []UPLink          `yaml:"links"`
}

// UPNode represent the user plane node
type UPNode struct {
	Type                 string                     `yaml:"type"`
	NodeID               string                     `yaml:"node_id"`
	ANIP                 string                     `yaml:"an_ip"`
	Dnn                  string                     `yaml:"dnn"`
	SNssaiInfos          []models.SnssaiUpfInfoItem `yaml:"sNssaiUpfInfos,omitempty"`
	InterfaceUpfInfoList []InterfaceUpfInfoItem     `yaml:"interfaces,omitempty"`
	Port                 uint16                     `yaml:"port"`
}

type InterfaceUpfInfoItem struct {
	NetworkInstance string                 `yaml:"networkInstance"`
	InterfaceType   models.UpInterfaceType `yaml:"interfaceType"`
	Endpoints       []string               `yaml:"endpoints"`
}

type UPLink struct {
	A string `yaml:"A"`
	B string `yaml:"B"`
}

var ConfigPodTrigger chan bool

func init() {
	ConfigPodTrigger = make(chan bool, 1)
}

func (c *Config) updateConfig(commChannel chan *protos.NetworkSliceResponse) bool {
	for {
		rsp := <-commChannel
		logger.GrpcLog.Infof("received updateConfig in the smf app: %+v \n", rsp)

		// update slice info
		cfgNew := Configuration{}
		if err := cfgNew.parseRocConfig(rsp); err != nil {
			logger.GrpcLog.Errorf("config update error: %v \n", err.Error())
			continue
		}

		// updates UpdatedSmfConfig struct to be consumed by SMF config update routine.
		compareAndProcessConfigs(c.Configuration, &cfgNew)

		// Update SMF's config copy for future compare
		// Acquire Lock before update as SMF main go-routine might be
		// still processing initial config and we don't want to update it in middle
		SmfConfigSyncLock.Lock()
		c.Configuration.SNssaiInfo = cfgNew.SNssaiInfo
		c.Configuration.UserPlaneInformation = cfgNew.UserPlaneInformation
		SmfConfigSyncLock.Unlock()
		// Send trigger to update SMF Context
		ConfigPodTrigger <- true
	}
}

// Update level-1 Configuration(Not actual SMF config structure used by SMF)
func (c *Configuration) parseRocConfig(rsp *protos.NetworkSliceResponse) error {
	// Reset previous SNSSAI structure
	if c.SNssaiInfo != nil {
		c.SNssaiInfo = nil
	}
	c.SNssaiInfo = make([]SnssaiInfoItem, 0)

	// Reset existing UP nodes and Links
	if c.UserPlaneInformation.UPNodes != nil {
		c.UserPlaneInformation.UPNodes = nil
	}
	c.UserPlaneInformation.UPNodes = make(map[string]UPNode)

	if c.UserPlaneInformation.Links != nil {
		c.UserPlaneInformation.Links = nil
	}
	c.UserPlaneInformation.Links = make([]UPLink, 0)

	c.EnterpriseList = make(map[string]string)

	// should be updated to be received from webui.
	// currently adding port info in webui causes crash.
	pfcpPortStr := os.Getenv("PFCP_PORT_UPF")
	pfcpPortVal := UPF_PFCP_PORT
	if pfcpPortStr != "" {
		if val, err := strconv.ParseUint(pfcpPortStr, 10, 32); err != nil {
			logger.CtxLog.Infoln("Parse pfcp port failed : ", pfcpPortStr)
			return err
		} else {
			pfcpPortVal = int(val)
		}
	}

	// Iterate through all NS received
	for _, ns := range rsp.NetworkSlice {
		// make new SNSSAI Info structure
		var sNssaiInfoItem SnssaiInfoItem

		// make SNSSAI
		var sNssai models.Snssai
		sNssai.Sd = ns.Nssai.Sd
		numSst, _ := strconv.Atoi(ns.Nssai.Sst)
		sNssai.Sst = int32(numSst)
		sNssaiInfoItem.SNssai = &sNssai

		// Add PLMN Id Info
		if ns.Site.Plmn != nil {
			sNssaiInfoItem.PlmnId.Mcc = ns.Site.Plmn.Mcc
			sNssaiInfoItem.PlmnId.Mnc = ns.Site.Plmn.Mnc
		}

		// Populate enterprise name
		c.EnterpriseList[ns.Nssai.Sst+ns.Nssai.Sd] = ns.Name

		// make DNN Info structure
		sNssaiInfoItem.DnnInfos = make([]SnssaiDnnInfoItem, 0)
		for _, devGrp := range ns.DeviceGroup {
			var dnnInfo SnssaiDnnInfoItem
			dnnInfo.Dnn = devGrp.IpDomainDetails.DnnName
			dnnInfo.DNS.IPv4Addr = devGrp.IpDomainDetails.DnsPrimary
			dnnInfo.UESubnet = devGrp.IpDomainDetails.UePool
			dnnInfo.MTU = uint16(devGrp.IpDomainDetails.Mtu)

			// update to Slice structure
			sNssaiInfoItem.DnnInfos = append(sNssaiInfoItem.DnnInfos, dnnInfo)
		}

		// Update to SMF config structure
		c.SNssaiInfo = append(c.SNssaiInfo, sNssaiInfoItem)

		// Check if port number is received as part of UpfName.
		// If yes, then use it as port number. else use common port number
		// from environment variable or if that also isn't available
		// then use default PFCP port 8805.
		portVal := uint16(pfcpPortVal)
		portStr := ""
		nodeStr := ns.Site.Upf.UpfName
		if strings.Contains(ns.Site.Upf.UpfName, ":") {
			if strings.LastIndex(ns.Site.Upf.UpfName, ":") < len(ns.Site.Upf.UpfName) {
				portStr = ns.Site.Upf.UpfName[strings.LastIndex(ns.Site.Upf.UpfName, ":")+1:]
			}
		}
		if portStr != "" {
			if val, err := strconv.ParseUint(portStr, 10, 32); err != nil {
				logger.CtxLog.Infoln("Parse Upf port failed : ", portStr)
				return err
			} else {
				portVal = uint16(val)
			}
			nodeStr = ns.Site.Upf.UpfName[:strings.LastIndex(ns.Site.Upf.UpfName, ":")]
		}

		ns.Site.Upf.UpfName = nodeStr
		// iterate through UPFs config received
		upf := UPNode{
			Type:                 "UPF",
			NodeID:               ns.Site.Upf.UpfName,
			Port:                 portVal,
			SNssaiInfos:          make([]models.SnssaiUpfInfoItem, 0),
			InterfaceUpfInfoList: make([]InterfaceUpfInfoItem, 0),
		}

		snsUpfInfoItem := models.SnssaiUpfInfoItem{
			SNssai:         &sNssai,
			DnnUpfInfoList: make([]models.DnnUpfInfoItem, 0),
		}

		// Popoulate DNN names per UPF slice Info
		for _, devGrp := range ns.DeviceGroup {
			// DNN Info in UPF per Slice
			var dnnUpfInfo models.DnnUpfInfoItem
			dnnUpfInfo.Dnn = devGrp.IpDomainDetails.DnnName
			snsUpfInfoItem.DnnUpfInfoList = append(snsUpfInfoItem.DnnUpfInfoList, dnnUpfInfo)

			// Populate UPF Interface Info and DNN info in UPF per Interface
			intfUpfInfoItem := InterfaceUpfInfoItem{
				InterfaceType: models.UpInterfaceType_N3,
				Endpoints:     make([]string, 0), NetworkInstance: devGrp.IpDomainDetails.DnnName,
			}
			intfUpfInfoItem.Endpoints = append(intfUpfInfoItem.Endpoints, ns.Site.Upf.UpfName)
			upf.InterfaceUpfInfoList = append(upf.InterfaceUpfInfoList, intfUpfInfoItem)
		}
		upf.SNssaiInfos = append(upf.SNssaiInfos, snsUpfInfoItem)

		// Update UPF to SMF Config Structure
		c.UserPlaneInformation.UPNodes[ns.Site.Upf.UpfName] = upf

		// Update gNB links to UPF(gNB <-> N3_UPF)
		for _, gNb := range ns.Site.Gnb {
			upLink := UPLink{A: gNb.Name, B: ns.Site.Upf.UpfName}
			c.UserPlaneInformation.Links = append(c.UserPlaneInformation.Links, upLink)

			// insert gNb to SMF Config Structure
			gNbNode := UPNode{Type: "AN", NodeID: gNb.Name}
			c.UserPlaneInformation.UPNodes[gNb.Name] = gNbNode
		}
	}
	logger.CfgLog.Infof("Parsed SMF config : %+v \n", c)
	return nil
}

func compareAndProcessConfigs(smfCfg, newCfg *Configuration) {
	// compare Network slices
	match, addSlices, modSlices, delSlices := compareNetworkSlices(smfCfg.SNssaiInfo, newCfg.SNssaiInfo)

	if !match {
		logger.CfgLog.Infof("changes in network slice config")

		if len(delSlices) > 0 {
			// delete slices from SMF config
			logger.CfgLog.Infof("network slices to be deleted : %+v", PrettyPrintNetworkSlices(delSlices))
			UpdatedSmfConfig.DelSNssaiInfo = &delSlices
		}

		if len(addSlices) > 0 {
			// insert slices to SMF config
			logger.CfgLog.Infof("network slices to be added : %+v", PrettyPrintNetworkSlices(addSlices))
			UpdatedSmfConfig.AddSNssaiInfo = &addSlices
		}

		if len(modSlices) > 0 {
			// Modify slices to SMF config
			logger.CfgLog.Infof("network slices to be modified : %+v", PrettyPrintNetworkSlices(modSlices))
			UpdatedSmfConfig.ModSNssaiInfo = &modSlices
		}
	} else {
		logger.CfgLog.Infoln("no changes in network slice config")
	}

	// compare Userplane
	match, addUPNodes, modUPNodes, delUPNodes := compareUPNodesConfigs(smfCfg.UserPlaneInformation.UPNodes, newCfg.UserPlaneInformation.UPNodes)

	if !match {
		logger.CfgLog.Infof("changes in user plane config")

		if len(delUPNodes) > 0 {
			// delete slices from SMF config
			logger.CfgLog.Infof("UP nodes to be deleted : %+v", PrettyPrintUPNodes(delUPNodes))
			UpdatedSmfConfig.DelUPNodes = &delUPNodes
		}

		if len(addUPNodes) > 0 {
			// insert slices to SMF config
			logger.CfgLog.Infof("UP nodes to be added : %+v", PrettyPrintUPNodes(addUPNodes))
			UpdatedSmfConfig.AddUPNodes = &addUPNodes
		}

		if len(modUPNodes) > 0 {
			// Modify slices to SMF config
			logger.CfgLog.Infof("UP nodes to be modified : %+v", PrettyPrintUPNodes(modUPNodes))
			UpdatedSmfConfig.ModUPNodes = &modUPNodes
		}
	} else {
		logger.CfgLog.Infoln("no change in user plane config")
	}

	// compare Links
	match, addLinks, delLinks := compareGenericSlices(smfCfg.UserPlaneInformation.Links,
		newCfg.UserPlaneInformation.Links, compareUPLinks)
	if !match {
		logger.CfgLog.Infof("changes in UP nodes links config")

		if s := addLinks.([]UPLink); len(s) > 0 {
			logger.CfgLog.Infof("UP nodes links to be added : %+v", s)
			UpdatedSmfConfig.AddLinks = &s
		}

		if s := delLinks.([]UPLink); len(s) > 0 {
			logger.CfgLog.Infof("UP nodes links to be deleted : %+v", s)
			UpdatedSmfConfig.DelLinks = &s
		}
	} else {
		logger.CfgLog.Infoln("no change in UP nodes links config")
	}

	// Enterprise Name
	UpdatedSmfConfig.EnterpriseList = &newCfg.EnterpriseList
}

func compareNsDnn(c1, c2 interface{}) bool {
	return c1.(SnssaiDnnInfoItem) == c2.(SnssaiDnnInfoItem)
}

func compareUPLinks(c1, c2 interface{}) bool {
	return c1.(UPLink).A == c2.(UPLink).A && c1.(UPLink).B == c2.(UPLink).B
}

func compareUpfDnn(c1, c2 interface{}) bool {
	return c1.(models.DnnUpfInfoItem).Dnn == c2.(models.DnnUpfInfoItem).Dnn
}

// Returns false if there is mismatch
func compareUPNode(u1, u2 UPNode) bool {
	if u1.ANIP == u2.ANIP &&
		u1.Dnn == u2.Dnn &&
		u1.NodeID == u2.NodeID &&
		u1.Type == u2.Type {
		if match, _, _, _ := compareUPNetworkSlices(u1.SNssaiInfos, u2.SNssaiInfos); !match {
			return false
		}
		// Todo: match InterfaceUpfInfoList
		return true
	}

	return false
}

func compareUPNodesConfigs(existingUPNodes, newUPNodes map[string]UPNode) (match bool, add, mod, del map[string]UPNode) {
	match = true
	add, mod, del = make(map[string]UPNode), make(map[string]UPNode), make(map[string]UPNode)

	// Check for modifications and deletions in existingUPNodes
	for existingUPNodename, existingUPNode := range existingUPNodes {
		if newUPNode, ok := newUPNodes[existingUPNodename]; ok {
			if !compareUPNode(existingUPNode, newUPNode) {
				mod[existingUPNodename] = newUPNode
				match = false
			}
		} else {
			del[existingUPNodename] = existingUPNode
			match = false
		}
	}

	// Check for additions in newUPNodes
	for newUPNodename, newUPNode := range newUPNodes {
		if _, ok := existingUPNodes[newUPNodename]; !ok {
			add[newUPNodename] = newUPNode
			match = false
		}
	}

	return match, add, mod, del
}

func compareNetworkSliceInstance(s1, s2 SnssaiInfoItem) (match bool) {
	if matching, _, _ := compareGenericSlices(s1.DnnInfos, s2.DnnInfos, compareNsDnn); !matching {
		return false
	}

	if s1.PlmnId != s2.PlmnId {
		return false
	}

	return true
}

func compareNetworkSlices(slice1, slice2 []SnssaiInfoItem) (match bool, add, mod, del []SnssaiInfoItem) {
	match = true
	// Loop two times, first to find slice1 strings not in slice2,
	// second loop to find slice2 strings not in slice1
	for i := 0; i < 2; i++ {
		for _, s1 := range slice1 {
			found := false
			for _, s2 := range slice2 {
				logger.CfgLog.Debugf("comparing slices [existing sd/sst:%+v/%+v][received sd/sst:%+v/%+v]", s1.SNssai.Sd, s1.SNssai.Sst, s2.SNssai.Sd, s2.SNssai.Sst)
				if s1.SNssai.Sd == s2.SNssai.Sd && s1.SNssai.Sst == s2.SNssai.Sst {
					if matching := compareNetworkSliceInstance(s1, s2); !matching && i == 0 {
						// only keep updated slice
						mod = append(mod, s2)
						match = false
					}
					found = true
					break
				}
			}

			// match not found. We add it to return slice
			if !found {
				match = false
				if i == 0 {
					del = append(del, s1)
				} else {
					add = append(add, s1)
				}
			}
		}
		// Swap the slices, only if it was the first loop
		if i == 0 {
			slice1, slice2 = slice2, slice1
		}
	}
	return match, add, mod, del
}

func compareUPNetworkSlices(slice1, slice2 []models.SnssaiUpfInfoItem) (match bool, add, mod, del []models.SnssaiUpfInfoItem) {
	match = true
	// Loop two times, first to find slice1 strings not in slice2,
	// second loop to find slice2 strings not in slice1
	for i := 0; i < 2; i++ {
		for _, s1 := range slice1 {
			found := false
			for _, s2 := range slice2 {
				logger.CfgLog.Debugf("comparing up slices[existing sd/sst: %+v/%+v][received sd/sst: %+v/%+v]", s1.SNssai.Sd, s1.SNssai.Sst, s2.SNssai.Sd, s2.SNssai.Sst)
				if s1.SNssai.Sd == s2.SNssai.Sd && s1.SNssai.Sst == s2.SNssai.Sst {
					if matching, _, _ := compareGenericSlices(s1.DnnUpfInfoList, s2.DnnUpfInfoList, compareUpfDnn); !matching && i == 0 {
						// only keep updated slice
						mod = append(mod, s2)
						match = false
					}
					found = true
					break
				}
			}

			// match not found. We add it to return slice
			if !found {
				match = false
				if i == 0 {
					del = append(del, s1)
				} else {
					add = append(add, s1)
				}
			}
		}
		// Swap the slices, only if it was the first loop
		if i == 0 {
			slice1, slice2 = slice2, slice1
		}
	}
	return match, add, mod, del
}

func compareGenericSlices(t1, t2 interface{}, compare func(i, j interface{}) bool) (match bool, add, remove interface{}) {
	contentType := reflect.TypeOf(t1)
	logger.CfgLog.Infoln("Comparing slices of type: ", contentType)

	slice1 := reflect.ValueOf(t1)
	slice2 := reflect.ValueOf(t2)

	insert := reflect.MakeSlice(contentType, 0, 0)
	deleteSlice := reflect.MakeSlice(contentType, 0, 0)

	match = true
	// Loop two times, first to find slice1 strings not in slice2,
	// second loop to find slice2 strings not in slice1
	for i := 0; i < 2; i++ {
		for s1 := 0; s1 < slice1.Len(); s1++ {
			found := false
			for s2 := 0; s2 < slice2.Len(); s2++ {
				if compare(slice1.Index(s1).Interface(), slice2.Index(s2).Interface()) {
					found = true
					break
				}
			}
			// String not found. We add it to return slice
			if !found {
				match = false
				if i == 0 {
					deleteSlice = reflect.Append(deleteSlice, slice1.Index(s1))
				} else {
					insert = reflect.Append(insert, slice1.Index(s1))
				}
			}
		}
		// Swap the slices, only if it was the first loop
		if i == 0 {
			slice1, slice2 = slice2, slice1
		}
	}

	return match, insert.Interface(), deleteSlice.Interface()
}

func PrettyPrintUPNodes(u map[string]UPNode) (s string) {
	for name, node := range u {
		s += fmt.Sprintf("\n UPNode Name[%v], Type[%v], NodeId[%v], Port[%v], ", name, node.Type, node.NodeID, node.Port)
		s += PrettyPrintUPSlices(node.SNssaiInfos)
		s += PrettyPrintUPInterfaces(node.InterfaceUpfInfoList)
	}
	return
}

func PrettyPrintUPSlices(upSlice []models.SnssaiUpfInfoItem) (s string) {
	for _, slice := range upSlice {
		s += fmt.Sprintf("\n Slice SST[%v] SD[%v] ", slice.SNssai.Sst, slice.SNssai.Sd)
		s += PrettyPrintUpfDnnSlices(slice.DnnUpfInfoList)
	}
	return
}

func PrettyPrintUpfDnnSlices(dnnSlice []models.DnnUpfInfoItem) (s string) {
	for _, dnn := range dnnSlice {
		s += fmt.Sprintf("\n DNN name[%v], DNAI[%v], PDU Sess Type[%v] ", dnn.Dnn, dnn.DnaiList, dnn.PduSessionTypes)
	}
	return
}

func PrettyPrintUPInterfaces(intfUpf []InterfaceUpfInfoItem) (s string) {
	for _, intf := range intfUpf {
		s += fmt.Sprintf("\n UP interface type[%v], network instance[%v], endpoints[%v], ",
			intf.InterfaceType, intf.NetworkInstance, intf.Endpoints)
	}
	return
}

func PrettyPrintNetworkSlices(networkSlice []SnssaiInfoItem) (s string) {
	for _, slice := range networkSlice {
		s += fmt.Sprintf("\n Slice SST[%v] SD[%v] ", slice.SNssai.Sst, slice.SNssai.Sd)
		s += PrettyPrintNetworkDnnSlices(slice.DnnInfos)
	}
	return
}

func PrettyPrintNetworkDnnSlices(dnnSlice []SnssaiDnnInfoItem) (s string) {
	for _, dnn := range dnnSlice {
		s += fmt.Sprintf("\n DNN name[%v], DNS v4[%v], v6[%v], UE-Pool[%v] ", dnn.Dnn, dnn.DNS.IPv4Addr, dnn.DNS.IPv6Addr, dnn.UESubnet)
	}
	return
}
