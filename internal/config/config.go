// Copyright 2024 Ella Networks

package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/vishvananda/netlink"
	"gopkg.in/yaml.v2"
)

const (
	AttachModeNative  = "native"
	AttachModeGeneric = "generic"
)

// AddressFamily specifies which IP address family to return when resolving an interface IP.
type AddressFamily int

const (
	// IPv4 filters for IPv4-only addresses.
	IPv4 AddressFamily = iota
	// AnyFamily accepts any address family, preferring non-link-local IPv6 over IPv4.
	AnyFamily
)

const (
	LoggingSystemOutputStdout = "stdout"
	LoggingSystemOutputFile   = "file"
)

type DB struct {
	Path string
}

type DBYaml struct {
	Path string `yaml:"path"`
}

type APIYaml struct {
	Port int     `yaml:"port"`
	TLS  TLSYaml `yaml:"tls"`
}

type UPFYaml struct {
	Interfaces []string `yaml:"interfaces"`
}

type TLS struct {
	Cert string
	Key  string
}

type TLSYaml struct {
	Cert string `yaml:"cert"`
	Key  string `yaml:"key"`
}

type N2InterfaceYaml struct {
	Name    string `yaml:"name"`
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
}

type N3InterfaceYaml struct {
	Name    string `yaml:"name"`
	Address string `yaml:"address"`
}

type N6InterfaceYaml struct {
	Name string `yaml:"name"`
}

type APIInterfaceYaml struct {
	Name    string  `yaml:"name"`
	Address string  `yaml:"address"`
	Port    int     `yaml:"port"`
	TLS     TLSYaml `yaml:"tls"`
}

type InterfacesYaml struct {
	N2  N2InterfaceYaml  `yaml:"n2"`
	N3  N3InterfaceYaml  `yaml:"n3"`
	N6  N6InterfaceYaml  `yaml:"n6"`
	API APIInterfaceYaml `yaml:"api"`
}

type XDPYaml struct {
	AttachMode string `yaml:"attach-mode"`
}

type SystemLoggingYaml struct {
	Level  string `yaml:"level"`
	Output string `yaml:"output"`
	Path   string `yaml:"path"`
}

type AuditLoggingYaml struct {
	Output string `yaml:"output"`
	Path   string `yaml:"path"`
}

type LoggingYaml struct {
	SystemLogging SystemLoggingYaml `yaml:"system"`
	AuditLogging  AuditLoggingYaml  `yaml:"audit"`
}

type TelemetryYaml struct {
	Enabled      bool   `yaml:"enabled"`
	OTLPEndpoint string `yaml:"otlp-endpoint"`
}

type ClusterYaml struct {
	Enabled             bool     `yaml:"enabled"`
	NodeID              int      `yaml:"node-id"`
	BindAddress         string   `yaml:"bind-address"`
	AdvertiseAPIAddress string   `yaml:"advertise-api-address"`
	BootstrapExpect     int      `yaml:"bootstrap-expect"`
	Peers               []string `yaml:"peers"`
	JoinToken           string   `yaml:"join-token"`
	JoinTimeout         string   `yaml:"join-timeout"`
	ProposeTimeout      string   `yaml:"propose-timeout"`
	SnapshotInterval    string   `yaml:"snapshot-interval"`
	SnapshotThreshold   uint64   `yaml:"snapshot-threshold"`
	InitialSuffrage     string   `yaml:"initial-suffrage"`
}

type ConfigYAML struct {
	Logging    LoggingYaml    `yaml:"logging"`
	DB         DBYaml         `yaml:"db"`
	Interfaces InterfacesYaml `yaml:"interfaces"`
	XDP        XDPYaml        `yaml:"xdp"`
	Telemetry  TelemetryYaml  `yaml:"telemetry"`
	Cluster    ClusterYaml    `yaml:"cluster"`
}

type N2Interface struct {
	Name    string
	Address string
	Port    int
}

type N3Interface struct {
	Name       string
	Address    string
	VlanConfig *VlanConfig
}

type N6Interface struct {
	Name       string
	VlanConfig *VlanConfig
}

type APIInterface struct {
	Name    string
	Address string
	Port    int
	TLS     TLS
}

type Interfaces struct {
	N2  N2Interface
	N3  N3Interface
	N6  N6Interface
	API APIInterface
}

type XDP struct {
	AttachMode string
}

type AuditLogging struct {
	Output string
	Path   string
}

type SystemLogging struct {
	Level  string
	Output string
	Path   string
}

type Logging struct {
	SystemLogging SystemLogging
	AuditLogging  AuditLogging
}

type Telemetry struct {
	Enabled      bool
	OTLPEndpoint string // e.g., "otel-collector.default.svc:4317"
}

// Cluster holds the resolved cluster configuration. When Enabled is false the
// binary runs as a standalone single-server instance.
type Cluster struct {
	Enabled             bool
	NodeID              int
	BindAddress         string
	AdvertiseAPIAddress string
	BootstrapExpect     int
	Peers               []string
	JoinToken           string
	JoinTimeout         time.Duration
	ProposeTimeout      time.Duration
	SnapshotInterval    time.Duration
	SnapshotThreshold   uint64
	InitialSuffrage     string
}

type Config struct {
	Logging    Logging
	DB         DB
	Interfaces Interfaces
	XDP        XDP
	Telemetry  Telemetry
	Cluster    Cluster
}

type VlanConfig struct {
	MasterInterface string
	VlanId          int
}

func Validate(filePath string) (Config, error) {
	config := Config{}

	configYaml, err := os.ReadFile(filePath) // #nosec: G304
	if err != nil {
		return Config{}, fmt.Errorf("cannot read config file: %w", err)
	}

	c := ConfigYAML{}

	if err := yaml.Unmarshal(configYaml, &c); err != nil {
		return Config{}, fmt.Errorf("cannot unmarshal config file")
	}

	if c.Logging == (LoggingYaml{}) {
		return Config{}, errors.New("logging is empty")
	}

	if c.Logging.SystemLogging == (SystemLoggingYaml{}) {
		return Config{}, errors.New("logging.system is empty")
	}

	if c.Logging.SystemLogging.Level == "" {
		return Config{}, errors.New("logging.system.level is empty")
	}

	if c.Logging.SystemLogging.Output == "" {
		return Config{}, errors.New("logging.system.output is empty")
	}

	if c.Logging.SystemLogging.Output != LoggingSystemOutputStdout && c.Logging.SystemLogging.Output != LoggingSystemOutputFile {
		return Config{}, errors.New("logging.system.output is invalid. Allowed values are: stdout, file")
	}

	if c.Logging.SystemLogging.Output == LoggingSystemOutputFile && c.Logging.SystemLogging.Path == "" {
		return Config{}, errors.New("logging.system.path is empty")
	}

	if c.Logging.AuditLogging == (AuditLoggingYaml{}) {
		return Config{}, errors.New("logging.audit is empty")
	}

	if c.Logging.AuditLogging.Output == "" {
		return Config{}, errors.New("logging.audit.output is empty")
	}

	if c.Logging.AuditLogging.Output != LoggingSystemOutputStdout && c.Logging.AuditLogging.Output != LoggingSystemOutputFile {
		return Config{}, errors.New("logging.audit.output is invalid. Allowed values are: stdout, file")
	}

	if c.Logging.AuditLogging.Output == LoggingSystemOutputFile && c.Logging.AuditLogging.Path == "" {
		return Config{}, errors.New("logging.audit.path is empty")
	}

	if c.DB == (DBYaml{}) {
		return Config{}, errors.New("db is empty")
	}

	if c.DB.Path == "" {
		return Config{}, errors.New("db.path is empty")
	}

	if c.Interfaces == (InterfacesYaml{}) {
		return Config{}, errors.New("interfaces is empty")
	}

	if c.Interfaces.N2 == (N2InterfaceYaml{}) {
		return Config{}, errors.New("interfaces.n2 is empty")
	}

	n2InterfaceName, n2Address, err := getInterfaceNameAndAddress(c.Interfaces.N2.Name, c.Interfaces.N2.Address, IPv4)
	if err != nil {
		return Config{}, fmt.Errorf("interfaces.n2: %w", err)
	}

	if c.Interfaces.N3 == (N3InterfaceYaml{}) {
		return Config{}, errors.New("interfaces.n3 is empty")
	}

	n3InterfaceName, n3Address, err := getInterfaceNameAndAddress(c.Interfaces.N3.Name, c.Interfaces.N3.Address, IPv4)
	if err != nil {
		return Config{}, fmt.Errorf("interfaces.n3: %w", err)
	}

	if c.Interfaces.N6 == (N6InterfaceYaml{}) {
		return Config{}, errors.New("interfaces.n6 is empty")
	}

	if c.Interfaces.N6.Name == "" {
		return Config{}, errors.New("interfaces.n6.name is empty")
	}

	if c.Interfaces.API == (APIInterfaceYaml{}) {
		return Config{}, errors.New("interfaces.api is empty")
	}

	apiInterfaceName, apiAddress, err := getInterfaceNameAndAddress(c.Interfaces.API.Name, c.Interfaces.API.Address, AnyFamily)
	if err != nil {
		return Config{}, fmt.Errorf("interfaces.api: %w", err)
	}

	if c.Interfaces.API.Port == 0 {
		return Config{}, errors.New("interfaces.api.port is empty")
	}

	if c.Interfaces.API.TLS.Cert != "" {
		if _, err := os.Stat(c.Interfaces.API.TLS.Cert); os.IsNotExist(err) {
			return Config{}, fmt.Errorf("cert file %s does not exist", c.Interfaces.API.TLS.Cert)
		}

		config.Interfaces.API.TLS.Cert = c.Interfaces.API.TLS.Cert
	}

	if c.Interfaces.API.TLS.Key != "" {
		if _, err := os.Stat(c.Interfaces.API.TLS.Key); os.IsNotExist(err) {
			return Config{}, fmt.Errorf("key file %s does not exist", c.Interfaces.API.TLS.Key)
		}

		config.Interfaces.API.TLS.Key = c.Interfaces.API.TLS.Key
	}

	if (c.Interfaces.API.TLS.Cert != "") != (c.Interfaces.API.TLS.Key != "") {
		return Config{}, errors.New("both interfaces.api.tls.cert and interfaces.api.tls.key must be provided together")
	}

	if c.XDP == (XDPYaml{}) {
		return Config{}, errors.New("xdp is empty")
	}

	if c.XDP.AttachMode != AttachModeNative && c.XDP.AttachMode != AttachModeGeneric {
		return Config{}, errors.New("xdp.attach-mode is invalid. Allowed values are: native, generic")
	}

	if c.XDP.AttachMode == AttachModeNative {
		config.Interfaces.N3.VlanConfig, err = GetVLANConfigForInterfaceFunc(n3InterfaceName)
		if err != nil {
			return Config{}, fmt.Errorf("cannot get vlan config for interface %s: %w", n3InterfaceName, err)
		}

		config.Interfaces.N6.VlanConfig, err = GetVLANConfigForInterfaceFunc(c.Interfaces.N6.Name)
		if err != nil {
			return Config{}, fmt.Errorf("cannot get vlan config for interface %s: %w", c.Interfaces.N6.Name, err)
		}
	}

	if c.Telemetry.Enabled && c.Telemetry.OTLPEndpoint == "" {
		return Config{}, errors.New("telemetry.otlp-endpoint is empty when telemetry.enabled is true")
	}

	config.Logging.SystemLogging.Level = c.Logging.SystemLogging.Level
	config.Logging.SystemLogging.Output = c.Logging.SystemLogging.Output
	config.Logging.SystemLogging.Path = c.Logging.SystemLogging.Path
	config.Logging.AuditLogging.Output = c.Logging.AuditLogging.Output
	config.Logging.AuditLogging.Path = c.Logging.AuditLogging.Path
	config.DB.Path = c.DB.Path
	config.Interfaces.N3.Name = n3InterfaceName
	config.Interfaces.N3.Address = n3Address
	config.Interfaces.N6.Name = c.Interfaces.N6.Name

	if c.Interfaces.N2.Name != "" {
		config.Interfaces.N2.Name = n2InterfaceName
	} else {
		config.Interfaces.N2.Address = n2Address
	}

	config.Interfaces.N2.Port = c.Interfaces.N2.Port

	if c.Interfaces.API.Name != "" {
		config.Interfaces.API.Name = apiInterfaceName
	} else {
		config.Interfaces.API.Address = apiAddress
	}

	config.Interfaces.API.Port = c.Interfaces.API.Port
	config.XDP.AttachMode = c.XDP.AttachMode
	config.Telemetry.OTLPEndpoint = c.Telemetry.OTLPEndpoint
	config.Telemetry.Enabled = c.Telemetry.Enabled

	cluster, err := validateCluster(c.Cluster)
	if err != nil {
		return Config{}, err
	}

	config.Cluster = cluster

	return config, nil
}

func getInterfaceNameAndAddress(interfaceName string, address string, family AddressFamily) (string, string, error) {
	if interfaceName != "" && address != "" {
		return "", "", errors.New("interface name and address cannot be both set")
	}

	if interfaceName == "" && address == "" {
		return "", "", errors.New("interface name or address must be set")
	}

	if interfaceName != "" {
		interfaceExists, err := InterfaceExists(interfaceName)
		if !interfaceExists {
			return "", "", fmt.Errorf("interface name %s does not exist on the host: %w", interfaceName, err)
		}

		computedAddress, err := GetInterfaceIP(interfaceName, family)
		if err != nil {
			return "", "", fmt.Errorf("cannot get IP address for interface %s: %w", interfaceName, err)
		}

		return interfaceName, computedAddress, nil
	}

	if address != "" {
		if net.ParseIP(address) == nil {
			return "", "", fmt.Errorf("interface address %s is not a valid IP address", address)
		}

		computedInterfaceName, err := GetInterfaceName(address)
		if err != nil {
			return "", "", fmt.Errorf("cannot get interface name for IP address %s: %w", address, err)
		}

		return computedInterfaceName, address, nil
	}

	return "", "", nil
}

var CheckInterfaceExistsFunc = func(name string) (bool, error) {
	networkInterface, err := net.InterfaceByName(name)
	if err != nil {
		return false, err
	}

	if networkInterface == nil {
		return false, nil
	}

	return true, nil
}

var GetInterfaceIPFunc = func(name string, family AddressFamily) (string, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return "", err
	}

	addresses, err := iface.Addrs()
	if err != nil {
		return "", err
	}

	var ipv4Fallback string

	for _, addr := range addresses {
		if ip, _, err := net.ParseCIDR(addr.String()); err == nil {
			switch family {
			case IPv4:
				if ip.To4() != nil {
					return ip.String(), nil
				}
			case AnyFamily:
				if ip.To4() == nil && !ip.IsLinkLocalUnicast() {
					return ip.String(), nil
				}

				if ip.To4() != nil && ipv4Fallback == "" {
					ipv4Fallback = ip.String()
				}
			}
		}
	}

	if family == AnyFamily && ipv4Fallback != "" {
		return ipv4Fallback, nil
	}

	return "", errors.New("no valid IP address found")
}

var GetInterfaceNameFunc = func(address string) (string, error) {
	if address == "" {
		return "", errors.New("address is empty")
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("cannot list network interfaces: %w", err)
	}

	for _, iface := range interfaces {
		addresses, err := iface.Addrs()
		if err != nil {
			return "", err
		}

		for _, addr := range addresses {
			if ip, _, err := net.ParseCIDR(addr.String()); err == nil {
				if ip.String() == address {
					return iface.Name, nil
				}
			}
		}
	}

	return "", nil
}

var GetVLANConfigForInterfaceFunc = func(name string) (*VlanConfig, error) {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return nil, err
	}

	if link.Type() == "vlan" {
		vlanLink := link.(*netlink.Vlan)

		parentLink, err := netlink.LinkByIndex(vlanLink.ParentIndex)
		if err != nil {
			return nil, err
		}

		config := VlanConfig{MasterInterface: parentLink.Attrs().Name, VlanId: vlanLink.VlanId}

		return &config, nil
	}

	return nil, nil
}

func GetInterfaceIP(name string, family AddressFamily) (string, error) {
	return GetInterfaceIPFunc(name, family)
}

func InterfaceExists(name string) (bool, error) {
	return CheckInterfaceExistsFunc(name)
}

func GetInterfaceName(address string) (string, error) {
	return GetInterfaceNameFunc(address)
}

var GetInterfaceIPsFunc = func(name string) ([]string, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}

	addresses, err := iface.Addrs()
	if err != nil {
		return nil, err
	}

	var ips []string

	for _, addr := range addresses {
		if ip, _, err := net.ParseCIDR(addr.String()); err == nil {
			if !ip.IsLinkLocalUnicast() && !ip.IsLoopback() {
				ips = append(ips, ip.String())
			}
		}
	}

	return ips, nil
}

func GetInterfaceIPs(name string) ([]string, error) {
	return GetInterfaceIPsFunc(name)
}

const maxClusterNodeID = 63

func validateCluster(c ClusterYaml) (Cluster, error) {
	if !c.Enabled {
		return Cluster{}, nil
	}

	if c.NodeID < 1 || c.NodeID > maxClusterNodeID {
		return Cluster{}, fmt.Errorf("cluster.node-id must be between 1 and %d (constrained by the 6-bit AMF Pointer field), got %d", maxClusterNodeID, c.NodeID)
	}

	if c.BindAddress == "" {
		return Cluster{}, errors.New("cluster.bind-address is required when cluster is enabled")
	}

	if _, _, err := net.SplitHostPort(c.BindAddress); err != nil {
		return Cluster{}, fmt.Errorf("cluster.bind-address %q is not a valid host:port: %w", c.BindAddress, err)
	}

	if c.AdvertiseAPIAddress == "" {
		return Cluster{}, errors.New("cluster.advertise-api-address is required when cluster is enabled")
	}

	selfURL, err := url.Parse(c.AdvertiseAPIAddress)
	if err != nil || selfURL.Scheme == "" || selfURL.Host == "" {
		return Cluster{}, fmt.Errorf("cluster.advertise-api-address %q is not a valid URL", c.AdvertiseAPIAddress)
	}

	if c.BootstrapExpect < 1 {
		return Cluster{}, fmt.Errorf("cluster.bootstrap-expect must be >= 1, got %d", c.BootstrapExpect)
	}

	if len(c.Peers) == 0 {
		return Cluster{}, errors.New("cluster.peers must not be empty when cluster is enabled")
	}

	if len(c.Peers) < c.BootstrapExpect {
		return Cluster{}, fmt.Errorf("cluster.peers has %d entries but cluster.bootstrap-expect is %d; peers must be >= bootstrap-expect", len(c.Peers), c.BootstrapExpect)
	}

	selfFound := false

	normalizedSelf := strings.TrimRight(c.AdvertiseAPIAddress, "/")

	for i, peer := range c.Peers {
		peerURL, pErr := url.Parse(peer)
		if pErr != nil || peerURL.Scheme == "" || peerURL.Host == "" {
			return Cluster{}, fmt.Errorf("cluster.peers[%d] %q is not a valid URL", i, peer)
		}

		if strings.TrimRight(peer, "/") == normalizedSelf {
			selfFound = true
		}
	}

	if !selfFound {
		return Cluster{}, fmt.Errorf("cluster.peers must include this node's advertise-api-address %q", c.AdvertiseAPIAddress)
	}

	if c.JoinToken == "" {
		return Cluster{}, errors.New("cluster.join-token is required when cluster is enabled")
	}

	var joinTimeout time.Duration

	if c.JoinTimeout != "" {
		joinTimeout, err = time.ParseDuration(c.JoinTimeout)
		if err != nil {
			return Cluster{}, fmt.Errorf("cluster.join-timeout %q: %w", c.JoinTimeout, err)
		}
	}

	var proposeTimeout time.Duration

	if c.ProposeTimeout != "" {
		proposeTimeout, err = time.ParseDuration(c.ProposeTimeout)
		if err != nil {
			return Cluster{}, fmt.Errorf("cluster.propose-timeout %q: %w", c.ProposeTimeout, err)
		}
	}

	var snapshotInterval time.Duration

	if c.SnapshotInterval != "" {
		snapshotInterval, err = time.ParseDuration(c.SnapshotInterval)
		if err != nil {
			return Cluster{}, fmt.Errorf("cluster.snapshot-interval %q: %w", c.SnapshotInterval, err)
		}
	}

	initialSuffrage := c.InitialSuffrage
	if initialSuffrage == "" {
		initialSuffrage = "voter"
	}

	if initialSuffrage != "voter" && initialSuffrage != "nonvoter" {
		return Cluster{}, fmt.Errorf("cluster.initial-suffrage must be \"voter\" or \"nonvoter\", got %q", initialSuffrage)
	}

	return Cluster{
		Enabled:             true,
		NodeID:              c.NodeID,
		BindAddress:         c.BindAddress,
		AdvertiseAPIAddress: c.AdvertiseAPIAddress,
		BootstrapExpect:     c.BootstrapExpect,
		Peers:               c.Peers,
		JoinToken:           c.JoinToken,
		JoinTimeout:         joinTimeout,
		ProposeTimeout:      proposeTimeout,
		SnapshotInterval:    snapshotInterval,
		SnapshotThreshold:   c.SnapshotThreshold,
		InitialSuffrage:     initialSuffrage,
	}, nil
}
