// Copyright 2024 Ella Networks

package config

import (
	"errors"
	"fmt"
	"net"
	"os"

	"gopkg.in/yaml.v2"
)

const (
	UpfNodeID = "0.0.0.0"
)

const (
	AttachModeNative  = "native"
	AttachModeGeneric = "generic"
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
	Name string `yaml:"name"`
	Port int    `yaml:"port"`
}

type N3InterfaceYaml struct {
	Name string `yaml:"name"`
}

type N6InterfaceYaml struct {
	Name string `yaml:"name"`
}

type APIInterfaceYaml struct {
	Name string `yaml:"name"`
	Port int    `yaml:"port"`
	TLS  TLSYaml
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

type ConfigYAML struct {
	Logging    LoggingYaml    `yaml:"logging"`
	DB         DBYaml         `yaml:"db"`
	Interfaces InterfacesYaml `yaml:"interfaces"`
	XDP        XDPYaml        `yaml:"xdp"`
	Telemetry  TelemetryYaml  `yaml:"telemetry"`
}

type API struct {
	Port int
	TLS  TLS
}

type N2Interface struct {
	Name    string
	Address string
	Port    int
}

type N3Interface struct {
	Name    string
	Address string
}

type N6Interface struct {
	Name string
}

type APIInterface struct {
	Name string
	Port int
	TLS  TLS
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

type Config struct {
	Logging    Logging
	DB         DB
	Interfaces Interfaces
	XDP        XDP
	Telemetry  Telemetry
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

	if c.Interfaces.N2.Name == "" {
		return Config{}, errors.New("interfaces.n2.name is empty")
	}

	if c.Interfaces.N2.Port == 0 {
		return Config{}, errors.New("interfaces.n2.port is empty")
	}

	n2Exists, err := InterfaceExists(c.Interfaces.N2.Name)
	if !n2Exists {
		return Config{}, fmt.Errorf("interfaces.n2.name %s does not exist on the host: %w", c.Interfaces.N2.Name, err)
	}

	if c.Interfaces.N3 == (N3InterfaceYaml{}) {
		return Config{}, errors.New("interfaces.n3 is empty")
	}

	if c.Interfaces.N3.Name == "" {
		return Config{}, errors.New("interfaces.n3.name is empty")
	}

	n3Exists, err := InterfaceExists(c.Interfaces.N3.Name)
	if !n3Exists {
		return Config{}, fmt.Errorf("interfaces.n3.name %s does not exist on the host: %w", c.Interfaces.N3.Name, err)
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

	if c.Interfaces.API.Name == "" {
		return Config{}, errors.New("interfaces.api.name is empty")
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

	if c.XDP == (XDPYaml{}) {
		return Config{}, errors.New("xdp is empty")
	}

	if c.XDP.AttachMode != AttachModeNative && c.XDP.AttachMode != AttachModeGeneric {
		return Config{}, errors.New("xdp.attach-mode is invalid. Allowed values are: native, generic")
	}

	n2Address, err := GetInterfaceIP(c.Interfaces.N2.Name)
	if err != nil {
		return Config{}, fmt.Errorf("cannot get IPv4 address for interface %s: %w", c.Interfaces.N2.Name, err)
	}

	n3Address, err := GetInterfaceIP(c.Interfaces.N3.Name)
	if err != nil {
		return Config{}, fmt.Errorf("cannot get IPv4 address for interface %s: %w", c.Interfaces.N3.Name, err)
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
	config.Interfaces.N2.Address = n2Address
	config.Interfaces.N2.Port = c.Interfaces.N2.Port
	config.Interfaces.N3.Name = c.Interfaces.N3.Name
	config.Interfaces.N3.Address = n3Address
	config.Interfaces.N6.Name = c.Interfaces.N6.Name
	config.Interfaces.API.Name = c.Interfaces.API.Name
	config.Interfaces.API.Port = c.Interfaces.API.Port
	config.XDP.AttachMode = c.XDP.AttachMode
	config.Telemetry.OTLPEndpoint = c.Telemetry.OTLPEndpoint
	config.Telemetry.Enabled = c.Telemetry.Enabled
	return config, nil
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

var GetInterfaceIPFunc = func(name string) (string, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return "", err
	}

	addresses, err := iface.Addrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addresses {
		if ip, _, err := net.ParseCIDR(addr.String()); err == nil {
			if ip.To4() != nil {
				return ip.String(), nil
			}
		}
	}

	return "", errors.New("no valid IPv4 address found")
}

func GetInterfaceIP(name string) (string, error) {
	return GetInterfaceIPFunc(name)
}

func InterfaceExists(name string) (bool, error) {
	return CheckInterfaceExistsFunc(name)
}
