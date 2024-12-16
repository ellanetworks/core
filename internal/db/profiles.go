package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/canonical/sqlair"
	"github.com/yeastengine/ella/internal/logger"
)

const ProfilesTableName = "profiles"

const QueryCreateProfilesTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		name TEXT NOT NULL,

		imsis TEXT,

		ueIpPool TEXT NOT NULL,
		dnsPrimary TEXT NOT NULL,
		dnsSecondary TEXT,
		mtu INTEGER NOT NULL,
		dnnMbrUplink INTEGER NOT NULL,
		dnnMbrDownlink INTEGER NOT NULL,
		bitrateUnit TEXT NOT NULL,
		qci INTEGER NOT NULL,
		arp INTEGER NOT NULL,
		pdb INTEGER NOT NULL,
		pelr INTEGER NOT NULL
)`

const (
	listProfilesStmt  = "SELECT &Profile.* from %s"
	getProfileStmt    = "SELECT &Profile.* from %s WHERE id==$Profile.id or name==$Profile.name"
	createProfileStmt = "INSERT INTO %s (name, imsis, ueIpPool, dnsPrimary, mtu, dnnMbrUplink, dnnMbrDownlink, bitrateUnit, qci, arp, pdb, pelr) VALUES ($Profile.name, $Profile.imsis, $Profile.ueIpPool, $Profile.dnsPrimary, $Profile.mtu, $Profile.dnnMbrUplink, $Profile.dnnMbrDownlink, $Profile.bitrateUnit, $Profile.qci, $Profile.arp, $Profile.pdb, $Profile.pelr)"
	deleteProfileStmt = "DELETE FROM %s WHERE id==$Profile.id"
)

type Profile struct {
	ID             int    `db:"id"`
	Name           string `db:"name"`
	Imsis          string `db:"imsis"`
	UeIpPool       string `db:"ueIpPool"`
	DnsPrimary     string `db:"dnsPrimary"`
	DnsSecondary   string `db:"dnsSecondary"`
	Mtu            int32  `db:"mtu"`
	DnnMbrUplink   int64  `db:"dnnMbrUplink"`
	DnnMbrDownlink int64  `db:"dnnMbrDownlink"`
	BitrateUnit    string `db:"bitrateUnit"`
	Qci            int32  `db:"qci"`
	Arp            int32  `db:"arp"`
	Pdb            int32  `db:"pdb"`
	Pelr           int32  `db:"pelr"`
}

func (ns *Profile) SetImsis(Imsis []string) error {
	data, err := json.Marshal(Imsis)
	if err != nil {
		return err
	}
	ns.Imsis = string(data)
	return nil
}

func (ns *Profile) GetImsis() ([]string, error) {
	var Imsis []string
	if ns.Imsis == "" {
		return Imsis, nil
	}
	err := json.Unmarshal([]byte(ns.Imsis), &Imsis)
	return Imsis, err
}

func (db *Database) ListProfiles() ([]Profile, error) {
	stmt, err := sqlair.Prepare(fmt.Sprintf(listProfilesStmt, db.profilesTable), Profile{})
	if err != nil {
		return nil, err
	}
	var profiles []Profile
	err = db.conn.Query(context.Background(), stmt).GetAll(&profiles)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return profiles, nil
}

func (db *Database) GetProfileByID(id int) (*Profile, error) {
	row := Profile{
		ID: id,
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(getProfileStmt, db.profilesTable), Profile{})
	if err != nil {
		return nil, err
	}
	err = db.conn.Query(context.Background(), stmt, row).Get(&row)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (db *Database) GetProfileByName(name string) (*Profile, error) {
	row := Profile{
		Name: name,
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(getProfileStmt, db.profilesTable), Profile{})
	if err != nil {
		return nil, err
	}
	err = db.conn.Query(context.Background(), stmt, row).Get(&row)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (db *Database) CreateProfile(profile *Profile) error {
	_, err := db.GetProfileByName(profile.Name)
	if err == nil {
		return fmt.Errorf("profile with name %s already exists", profile.Name)
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(createProfileStmt, db.profilesTable), Profile{})
	if err != nil {
		return err
	}
	err = db.conn.Query(context.Background(), stmt, profile).Run()
	logger.DBLog.Infof("Created Profile: %v with Imsis: %v", profile.Name, profile.Imsis)
	return err
}

func (db *Database) DeleteProfile(id int) error {
	_, err := db.GetProfileByID(id)
	if err != nil {
		return err
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(deleteProfileStmt, db.profilesTable), Profile{})
	if err != nil {
		return err
	}
	row := Profile{
		ID: id,
	}
	err = db.conn.Query(context.Background(), stmt, row).Run()
	return err
}
