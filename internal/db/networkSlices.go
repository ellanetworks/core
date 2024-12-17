package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/canonical/sqlair"
)

const NetworkSlicesTableName = "networkSlices"

const QueryCreateNetworkSlicesTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		name TEXT NOT NULL,

		sst TEXT NOT NULL,
		sd TEXT NOT NULL,
		deviceGroups TEXT NOT NULL,
		mcc TEXT NOT NULL,
		mnc TEXT NOT NULL,
		gNodeBs TEXT NOT NULL,
		upf TEXT NOT NULL
)`

const (
	listNetworkSlicesStmt  = "SELECT &NetworkSlice.* from %s"
	getNetworkSliceStmt    = "SELECT &NetworkSlice.* from %s WHERE name==$NetworkSlice.name"
	createNetworkSliceStmt = "INSERT INTO %s (name, sst, sd, deviceGroups, mcc, mnc, gNodeBs, upf) VALUES ($NetworkSlice.name, $NetworkSlice.sst, $NetworkSlice.sd, $NetworkSlice.deviceGroups, $NetworkSlice.mcc, $NetworkSlice.mnc, $NetworkSlice.gNodeBs, $NetworkSlice.upf)"
	deleteNetworkSliceStmt = "DELETE FROM %s WHERE name==$NetworkSlice.name"
)

type GNodeB struct {
	Name string `json:"name,omitempty"`
	Tac  int32  `json:"tac,omitempty"`
}

type UPF struct {
	Name string `json:"name,omitempty"`
	Port int    `json:"port,omitempty"`
}

type NetworkSlice struct {
	ID           int    `db:"id"`
	Name         string `db:"name"`
	Sst          string `db:"sst"`
	Sd           string `db:"sd"`
	DeviceGroups string `db:"deviceGroups"`
	Mcc          string `db:"mcc"`
	Mnc          string `db:"mnc"`
	GNodeBs      string `db:"gNodeBs"`
	Upf          string `db:"upf"`
}

func (ns *NetworkSlice) GetDeviceGroups() []string {
	if ns.DeviceGroups == "" {
		return []string{}
	}
	return strings.Split(ns.DeviceGroups, ",")
}

func (ns *NetworkSlice) SetDeviceGroups(groups []string) {
	ns.DeviceGroups = strings.Join(groups, ",")
}

func (ns *NetworkSlice) GetGNodeBs() ([]GNodeB, error) {
	var gNodeBs []GNodeB
	if ns.GNodeBs == "" {
		return gNodeBs, nil
	}
	err := json.Unmarshal([]byte(ns.GNodeBs), &gNodeBs)
	return gNodeBs, err
}

func (ns *NetworkSlice) SetGNodeBs(gNodeBs []GNodeB) error {
	data, err := json.Marshal(gNodeBs)
	if err != nil {
		return err
	}
	ns.GNodeBs = string(data)
	return nil
}

func (ns *NetworkSlice) GetUpf() (*UPF, error) {
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

func (ns *NetworkSlice) SetUpf(upf UPF) error {
	data, err := json.Marshal(upf)
	if err != nil {
		return err
	}
	ns.Upf = string(data)
	return nil
}

func (db *Database) ListNetworkSlices() ([]NetworkSlice, error) {
	stmt, err := sqlair.Prepare(fmt.Sprintf(listNetworkSlicesStmt, db.networkSlicesTable), NetworkSlice{})
	if err != nil {
		return nil, err
	}
	var networkSlices []NetworkSlice
	err = db.conn.Query(context.Background(), stmt).GetAll(&networkSlices)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return networkSlices, nil
}

func (db *Database) GetNetworkSlice(name string) (*NetworkSlice, error) {
	row := NetworkSlice{
		Name: name,
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(getNetworkSliceStmt, db.networkSlicesTable), NetworkSlice{})
	if err != nil {
		return nil, err
	}
	err = db.conn.Query(context.Background(), stmt, row).Get(&row)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (db *Database) CreateNetworkSlice(networkSlice *NetworkSlice) error {
	_, err := db.GetNetworkSlice(networkSlice.Name)
	if err == nil {
		return fmt.Errorf("network slice with name %s already exists", networkSlice.Name)
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(createNetworkSliceStmt, db.networkSlicesTable), NetworkSlice{})
	if err != nil {
		return err
	}
	err = db.conn.Query(context.Background(), stmt, networkSlice).Run()
	return err
}

func (db *Database) DeleteNetworkSlice(name string) error {
	_, err := db.GetNetworkSlice(name)
	if err != nil {
		return err
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(deleteNetworkSliceStmt, db.networkSlicesTable), NetworkSlice{})
	if err != nil {
		return err
	}
	row := NetworkSlice{
		Name: name,
	}
	err = db.conn.Query(context.Background(), stmt, row).Run()
	return err
}
