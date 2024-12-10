package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type DB struct {
	Url  string
	Name string
}

type DBYaml struct {
	Url  string `yaml:"url"`
	Name string `yaml:"name"`
}

type APIYaml struct {
	Port int     `yaml:"port"`
	TLS  TLSYaml `yaml:"tls"`
}

type UPFYaml struct {
	Interfaces []string `yaml:"interfaces"`
	N3Address  string   `yaml:"n3-address"`
}

type UPF struct {
	Interfaces []string
	N3Address  string
}

type TLS struct {
	Cert string
	Key  string
}

type TLSYaml struct {
	Cert string `yaml:"cert"`
	Key  string `yaml:"key"`
}

type ConfigYAML struct {
	DB  DBYaml  `yaml:"db"`
	UPF UPFYaml `yaml:"upf"`
	API APIYaml `yaml:"api"`
}

type API struct {
	Port int
	TLS  TLS
}

type Config struct {
	DB  DB
	UPF UPF
	Api API
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
	if c.API.Port == 0 {
		return Config{}, errors.New("api.port is empty")
	}
	if c.API.TLS.Cert == "" {
		return Config{}, fmt.Errorf("api.tls.cert is empty")
	}
	if c.API.TLS.Key == "" {
		return Config{}, fmt.Errorf("api.tls.key is empty")
	}
	if c.DB == (DBYaml{}) {
		return Config{}, errors.New("db is empty")
	}
	if c.DB.Url == "" {
		return Config{}, errors.New("db.url is empty")
	}
	if c.DB.Name == "" {
		return Config{}, errors.New("db.name is empty")
	}
	config.Api.Port = c.API.Port
	config.Api.Port = c.API.Port
	config.Api.TLS.Cert = c.API.TLS.Cert
	config.Api.TLS.Key = c.API.TLS.Key
	config.DB.Url = c.DB.Url
	config.DB.Name = c.DB.Name
	config.UPF.Interfaces = c.UPF.Interfaces
	config.UPF.N3Address = c.UPF.N3Address
	return config, nil
}
