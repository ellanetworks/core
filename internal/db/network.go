package db

import (
	"context"
	"fmt"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/logger"
)

const NetworkTableName = "network"

const QueryCreateNetworkTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		mcc TEXT NOT NULL,
		mnc TEXT NOT NULL
)`

const (
	DefaultMcc = "001"
	DefaultMnc = "01"
)

const (
	getNetworkStmt        = "SELECT &Network.* FROM %s WHERE id=1"
	updateNetworkStmt     = "UPDATE %s SET mcc=$Network.mcc, mnc=$Network.mnc WHERE id=1"
	initializeNetworkStmt = "INSERT INTO %s (mcc, mnc) VALUES ($Network.mcc, $Network.mnc)"
)

type Network struct {
	ID  int    `db:"id"`
	Mcc string `db:"mcc"`
	Mnc string `db:"mnc"`
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
		Mcc: DefaultMcc,
		Mnc: DefaultMnc,
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
