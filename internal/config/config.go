package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type Mongo struct {
	Url  string
	Name string
}

type MongoYaml struct {
	Url  string `yaml:"url"`
	Name string `yaml:"name"`
}

type Sql struct {
	Path string `yaml:"path"`
}

type SqlYaml struct {
	Path string `yaml:"path"`
}

type DB struct {
	Mongo Mongo
	Sql   Sql
}

type DBYaml struct {
	Mongo MongoYaml `yaml:"mongo"`
	Sql   SqlYaml   `yaml:"sql"`
}

type UPFYaml struct {
	Interfaces []string `yaml:"interfaces"`
	N3Address  string   `yaml:"n3-address"`
}

type UPF struct {
	Interfaces []string
	N3Address  string
}

type ConfigYAML struct {
	DB  DBYaml  `yaml:"db"`
	UPF UPFYaml `yaml:"upf"`
}

type Config struct {
	DB  DB
	UPF UPF
}

func Validate(filePath string) (Config, error) {
	config := Config{}
	configYaml, err := os.ReadFile(filePath)
	if err != nil {
		return Config{}, fmt.Errorf("cannot read config file: %w", err)
	}
	c := ConfigYAML{}
	if err := yaml.Unmarshal(configYaml, &c); err != nil {
		return Config{}, fmt.Errorf("cannot unmarshal config file: %w", err)
	}
	if c.DB == (DBYaml{}) {
		return Config{}, errors.New("db is empty")
	}
	if c.DB.Mongo.Url == "" {
		return Config{}, errors.New("db.mongo.url is empty")
	}
	if c.DB.Mongo.Name == "" {
		return Config{}, errors.New("db.mongo.name is empty")
	}
	if c.DB.Sql.Path == "" {
		return Config{}, errors.New("db.sql.path is empty")
	}
	config.DB.Mongo.Url = c.DB.Mongo.Url
	config.DB.Mongo.Name = c.DB.Mongo.Name
	config.DB.Sql.Path = c.DB.Sql.Path
	config.UPF.Interfaces = c.UPF.Interfaces
	config.UPF.N3Address = c.UPF.N3Address
	return config, nil
}
