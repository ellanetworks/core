package factory

import (
	"time"

	"github.com/omec-project/openapi/models"
	logger_util "github.com/omec-project/util/logger"
)

const (
	SMF_PFCP_PORT = 8805
	UPF_PFCP_PORT = 8806
)

type Configuration struct {
	Logger          *logger_util.Logger
	PFCP            *PFCP
	Sbi             *Sbi
	AmfUri          string
	PcfUri          string
	UdmUri          string
	SmfName         string
	StaticIpInfo    []StaticIpInfo
	ServiceNameList []string
	ULCL            bool
}

type StaticIpInfo struct {
	ImsiIpInfo map[string]string
	Dnn        string
}

type Sbi struct {
	BindingIPv4 string
	Port        int
}

type PFCP struct {
	Addr string
	Port uint16
}

type DNS struct {
	IPv4Addr string
	IPv6Addr string
}

type Path struct {
	DestinationIP   string
	DestinationPort string
	UPF             []string
}

type UERoutingInfo struct {
	SUPI     string
	AN       string
	PathList []Path
}

// RouteProfID is string providing a Route Profile identifier.
type RouteProfID string

// RouteProfile maintains the mapping between RouteProfileID and ForwardingPolicyID of UPF
type RouteProfile struct {
	// Forwarding Policy ID of the route profile
	ForwardingPolicyID string
}

// PfdContent represents the flow of the application
type PfdContent struct {
	// Identifies a PFD of an application identifier.
	PfdID string
	// Represents a 3-tuple with protocol, server ip and server port for
	// UL/DL application traffic.
	FlowDescriptions []string
	// Indicates a URL or a regular expression which is used to match the
	// significant parts of the URL.
	Urls []string
	// Indicates an FQDN or a regular expression as a domain name matching
	// criteria.
	DomainNames []string
}

// PfdDataForApp represents the PFDs for an application identifier
type PfdDataForApp struct {
	// Caching time for an application identifier.
	CachingTime *time.Time
	// Identifier of an application.
	AppID string
	// PFDs for the application identifier.
	Pfds []PfdContent
}

type RoutingConfig struct {
	UERoutingInfo []*UERoutingInfo
	RouteProf     map[RouteProfID]RouteProfile
	PfdDatas      []*PfdDataForApp
}

type InterfaceUpfInfoItem struct {
	NetworkInstance string
	InterfaceType   models.UpInterfaceType
	Endpoints       []string
}
