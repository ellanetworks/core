package config

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/yaml.v2"
)

type DBConfig struct {
	Url string `yaml:"url"`
}

type UPFConfig struct {
	Interfaces []string `yaml:"interfaces"`
	N3Address  string   `yaml:"n3-address"`
}

type Config struct {
	DB  *DBConfig  `yaml:"db"`
	UPF *UPFConfig `yaml:"upf"`
}

func Parse(configPath string) (Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return Config{}, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}
func (dbConfig *DBConfig) Validate() error {
	if dbConfig == nil {
		return fmt.Errorf("db is required")
	}
	if dbConfig.Url == "" {
		return fmt.Errorf("db.url is required")
	}

	clientOptions := options.Client().ApplyURI(dbConfig.Url)
	client, err := mongo.NewClient(clientOptions)
	if err != nil {
		return fmt.Errorf("failed to create MongoDB client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}
	defer client.Disconnect(ctx)

	// Ping the database to check connectivity
	err = client.Ping(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	return nil
}

func (upfConfig *UPFConfig) Validate() error {
	if upfConfig == nil {
		return fmt.Errorf("upf section is required")
	}
	if len(upfConfig.Interfaces) == 0 {
		return fmt.Errorf("upf.interfaces is required")
	}
	for _, iface := range upfConfig.Interfaces {
		if _, err := net.InterfaceByName(iface); err != nil {
			return fmt.Errorf("upf interface %s does not exist", iface)
		}
	}
	if upfConfig.N3Address == "" {
		return fmt.Errorf("upf.n3-address is required")
	}
	if net.ParseIP(upfConfig.N3Address) == nil {
		return fmt.Errorf("upf.n3-address is not a valid IP address: %s", upfConfig.N3Address)
	}
	return nil
}

func (cfg *Config) Validate() error {
	if err := cfg.DB.Validate(); err != nil {
		return err
	}
	if err := cfg.UPF.Validate(); err != nil {
		return err
	}
	return nil
}
