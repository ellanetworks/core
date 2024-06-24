package factory

import (
	"fmt"
	"time"

	"github.com/omec-project/openapi/models"
	logger_util "github.com/omec-project/util/logger"
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

type Mongodb struct {
	Name string `yaml:"name"`
	Url  string `yaml:"url"`
}

type Configuration struct {
	Mongodb                  *Mongodb             `yaml:"mongodb,omitempty"`
	PFCP                     *PFCP                `yaml:"pfcp,omitempty"`
	Sbi                      *Sbi                 `yaml:"sbi,omitempty"`
	NrfUri                   string               `yaml:"nrfUri,omitempty"`
	WebuiUri                 string               `yaml:"webuiUri"`
	SmfName                  string               `yaml:"smfName,omitempty"`
	SmfDbName                string               `yaml:"smfDBName,omitempty"`
	SNssaiInfo               []SnssaiInfoItem     `yaml:"snssaiInfos,omitempty"`
	StaticIpInfo             []StaticIpInfo       `yaml:"staticIpInfo"`
	ServiceNameList          []string             `yaml:"serviceNameList,omitempty"`
	EnterpriseList           map[string]string    `yaml:"enterpriseList,omitempty"`
	UserPlaneInformation     UserPlaneInformation `yaml:"userplane_information"`
	NrfCacheEvictionInterval int                  `yaml:"nrfCacheEvictionInterval"`
	EnableNrfCaching         bool                 `yaml:"enableNrfCaching"`
	ULCL                     bool                 `yaml:"ulcl,omitempty"`
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
	RegisterIPv4 string `yaml:"registerIPv4,omitempty"` // IP that is registered at NRF.
	// IPv6Addr string `yaml:"ipv6Addr,omitempty"`
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
