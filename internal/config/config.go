// Copyright 2024 Ella Networks

package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

const (
	DNN       = "internet"
	UpfNodeId = "0.0.0.0"
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

type N3InterfaceYaml struct {
	Name    string `yaml:"name"`
	Address string `yaml:"address"`
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
	N3  N3InterfaceYaml  `yaml:"n3"`
	N6  N6InterfaceYaml  `yaml:"n6"`
	API APIInterfaceYaml `yaml:"api"`
}

type ConfigYAML struct {
	DB         DBYaml         `yaml:"db"`
	LogLevel   string         `yaml:"log-level"`
	Interfaces InterfacesYaml `yaml:"interfaces"`
}

type API struct {
	Port int
	TLS  TLS
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
	N3  N3Interface
	N6  N6Interface
	API APIInterface
}

type Config struct {
	DB         DB
	LogLevel   string
	Interfaces Interfaces
}

func Validate(filePath string) (Config, error) {
	config := Config{}
	configYaml, err := os.ReadFile(filePath)
	if err != nil {
		return Config{}, fmt.Errorf("cannot read config file: %w", err)
	}
	c := ConfigYAML{}
	if err := yaml.Unmarshal(configYaml, &c); err != nil {
		return Config{}, fmt.Errorf("cannot unmarshal config file")
	}
	if c.LogLevel == "" {
		return Config{}, errors.New("log-level is empty. Allowed values are: debug, info, warn, error, panic, fatal")
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
	if c.Interfaces.N3 == (N3InterfaceYaml{}) {
		return Config{}, errors.New("interfaces.n3 is empty")
	}
	if c.Interfaces.N3.Address == "" {
		return Config{}, errors.New("interfaces.n3.address is empty")
	}
	if c.Interfaces.N3.Name == "" {
		return Config{}, errors.New("interfaces.n3.name is empty")
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
	if c.Interfaces.API.TLS.Cert == "" {
		return Config{}, fmt.Errorf("interfaces.api.tls.cert is empty")
	}
	if c.Interfaces.API.TLS.Key == "" {
		return Config{}, fmt.Errorf("interfaces.api.tls.key is empty")
	}

	if _, err := os.Stat(c.Interfaces.API.TLS.Cert); os.IsNotExist(err) {
		return Config{}, fmt.Errorf("cert file %s does not exist", c.Interfaces.API.TLS.Cert)
	}

	if _, err := os.Stat(c.Interfaces.API.TLS.Key); os.IsNotExist(err) {
		return Config{}, fmt.Errorf("key file %s does not exist", c.Interfaces.API.TLS.Key)
	}

	config.LogLevel = c.LogLevel
	config.DB.Path = c.DB.Path
	config.Interfaces.N3.Name = c.Interfaces.N3.Name
	config.Interfaces.N3.Address = c.Interfaces.N3.Address
	config.Interfaces.N6.Name = c.Interfaces.N6.Name
	config.Interfaces.API.Name = c.Interfaces.API.Name
	config.Interfaces.API.Port = c.Interfaces.API.Port
	config.Interfaces.API.TLS.Cert = c.Interfaces.API.TLS.Cert
	config.Interfaces.API.TLS.Key = c.Interfaces.API.TLS.Key
	return config, nil
}
