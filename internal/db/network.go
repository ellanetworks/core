package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/canonical/sqlair"
	"github.com/yeastengine/ella/internal/logger"
)

const NetworkTableName = "network"

const QueryCreateNetworkTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		sst TEXT NOT NULL,
		sd TEXT NOT NULL,
		profiles TEXT NOT NULL,
		mcc TEXT NOT NULL,
		mnc TEXT NOT NULL,
		gNodeBs TEXT NOT NULL,
		upf TEXT NOT NULL
)`

const (
	DefaultSst = "1"
	DefaultSd  = "102030"
	DefaultMcc = "001"
	DefaultMnc = "01"
)
const (
	getNetworkStmt        = "SELECT &Network.* FROM %s WHERE id=1"
	updateNetworkStmt     = "UPDATE %s SET sst=$Network.sst, sd=$Network.sd, profiles=$Network.profiles, mcc=$Network.mcc, mnc=$Network.mnc, gNodeBs=$Network.gNodeBs, upf=$Network.upf WHERE id=1"
	initializeNetworkStmt = "INSERT INTO %s (sst, sd, profiles, mcc, mnc, gNodeBs, upf) VALUES ($Network.sst, $Network.sd, $Network.profiles, $Network.mcc, $Network.mnc, $Network.gNodeBs, $Network.upf)"
)

type GNodeB struct {
	Name string `json:"name,omitempty"`
	Tac  int32  `json:"tac,omitempty"`
}

type UPF struct {
	Name string `json:"name,omitempty"`
	Port int    `json:"port,omitempty"`
}

type Network struct {
	ID       int    `db:"id"`
	Sst      string `db:"sst"`
	Sd       string `db:"sd"`
	Profiles string `db:"profiles"`
	Mcc      string `db:"mcc"`
	Mnc      string `db:"mnc"`
	GNodeBs  string `db:"gNodeBs"`
	Upf      string `db:"upf"`
}

func (ns *Network) ListProfiles() []string {
	if ns == nil {
		return []string{}
	}
	if ns.Profiles == "" {
		return []string{}
	}
	return strings.Split(ns.Profiles, ",")
}

func (ns *Network) SetProfiles(groups []string) {
	ns.Profiles = strings.Join(groups, ",")
}

func (ns *Network) GetGNodeBs() ([]GNodeB, error) {
	var gNodeBs []GNodeB
	if ns.GNodeBs == "" {
		return gNodeBs, nil
	}
	err := json.Unmarshal([]byte(ns.GNodeBs), &gNodeBs)
	return gNodeBs, err
}

func (ns *Network) SetGNodeBs(gNodeBs []GNodeB) error {
	data, err := json.Marshal(gNodeBs)
	if err != nil {
		return err
	}
	ns.GNodeBs = string(data)
	return nil
}

func (ns *Network) GetUpf() (*UPF, error) {
	if ns.Upf == "" {
		return nil, nil
	}
	var upf UPF
	err := json.Unmarshal([]byte(ns.Upf), &upf)
	if err != nil {
		return nil, err
	}
	return &upf, nil
}

func (ns *Network) SetUpf(upf UPF) error {
	data, err := json.Marshal(upf)
	if err != nil {
		return err
	}
	ns.Upf = string(data)
	return nil
}

func (db *Database) GetNetwork() (*Network, error) {
	stmt, err := sqlair.Prepare(fmt.Sprintf(getNetworkStmt, db.networkTable), Network{})
	if err != nil {
		return nil, fmt.Errorf("failed to prepare get network configuration statement: %v", err)
	}
	var network Network
	err = db.conn.Query(context.Background(), stmt).Get(&network)
	if err != nil {
		return nil, fmt.Errorf("failed to get network configuration: %v", err)
	}
	return &network, nil
}

func (db *Database) InitializeNetwork() error {
	stmt, err := sqlair.Prepare(fmt.Sprintf(initializeNetworkStmt, db.networkTable), Network{})
	if err != nil {
		return fmt.Errorf("failed to prepare initialize network configuration statement: %v", err)
	}
	network := Network{
		Sst:      DefaultSst,
		Sd:       DefaultSd,
		Profiles: "",
		Mcc:      DefaultMcc,
		Mnc:      DefaultMnc,
		GNodeBs:  "",
		Upf:      "",
	}
	err = db.conn.Query(context.Background(), stmt, network).Run()
	if err != nil {
		return fmt.Errorf("failed to initialize network configuration: %v", err)
	}
	logger.DBLog.Infof("Initialized network configuration")
	return nil
}

func (db *Database) UpdateNetwork(config *Network) error {
	stmt, err := sqlair.Prepare(fmt.Sprintf(updateNetworkStmt, db.networkTable), Network{})
	if err != nil {
		return err
	}
	err = db.conn.Query(context.Background(), stmt, config).Run()
	if err != nil {
		return fmt.Errorf("failed to update network configuration: %v", err)
	}
	logger.DBLog.Infof("Updated network configuration")
	return nil
}
