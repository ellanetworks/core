// Copyright 2024 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/canonical/sqlair"
)

const ProfilesTableName = "profiles"

const QueryCreateProfilesTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		name TEXT NOT NULL,

		ueIPPool TEXT NOT NULL,
		dns TEXT NOT NULL,
		mtu INTEGER NOT NULL,
		bitrateUplink TEXT NOT NULL,
		bitrateDownlink TEXT NOT NULL,
		var5qi INTEGER NOT NULL,
		priorityLevel INTEGER NOT NULL
)`

const (
	listProfilesStmt   = "SELECT &Profile.* from %s"
	getProfileStmt     = "SELECT &Profile.* from %s WHERE name==$Profile.name"
	getProfileByIDStmt = "SELECT &Profile.* FROM %s WHERE id==$Profile.id"
	createProfileStmt  = "INSERT INTO %s (name, ueIPPool, dns, mtu, bitrateUplink, bitrateDownlink, var5qi, priorityLevel) VALUES ($Profile.name, $Profile.ueIPPool, $Profile.dns, $Profile.mtu, $Profile.bitrateUplink, $Profile.bitrateDownlink, $Profile.var5qi, $Profile.priorityLevel)"
	editProfileStmt    = "UPDATE %s SET ueIPPool=$Profile.ueIPPool, dns=$Profile.dns, mtu=$Profile.mtu, bitrateUplink=$Profile.bitrateUplink, bitrateDownlink=$Profile.bitrateDownlink, var5qi=$Profile.var5qi, priorityLevel=$Profile.priorityLevel WHERE name==$Profile.name"
	deleteProfileStmt  = "DELETE FROM %s WHERE name==$Profile.name"
)

type Profile struct {
	ID              int    `db:"id"`
	Name            string `db:"name"`
	UeIPPool        string `db:"ueIPPool"`
	DNS             string `db:"dns"`
	Mtu             int32  `db:"mtu"`
	BitrateUplink   string `db:"bitrateUplink"`
	BitrateDownlink string `db:"bitrateDownlink"`
	Var5qi          int32  `db:"var5qi"`
	PriorityLevel   int32  `db:"priorityLevel"`
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

func (db *Database) GetProfile(name string) (*Profile, error) {
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

func (db *Database) GetProfileByID(id int) (*Profile, error) {
	row := Profile{
		ID: id,
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(getProfileByIDStmt, db.profilesTable), Profile{})
	if err != nil {
		return nil, err
	}
	err = db.conn.Query(context.Background(), stmt, row).Get(&row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("profile with ID %d not found", id)
		}
		return nil, err
	}
	return &row, nil
}

func (db *Database) CreateProfile(profile *Profile) error {
	_, err := db.GetProfile(profile.Name)
	if err == nil {
		return fmt.Errorf("profile with name %s already exists", profile.Name)
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(createProfileStmt, db.profilesTable), Profile{})
	if err != nil {
		return err
	}
	err = db.conn.Query(context.Background(), stmt, profile).Run()
	return err
}

func (db *Database) UpdateProfile(profile *Profile) error {
	_, err := db.GetProfile(profile.Name)
	if err != nil {
		return err
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(editProfileStmt, db.profilesTable), Profile{})
	if err != nil {
		return err
	}
	err = db.conn.Query(context.Background(), stmt, profile).Run()
	return err
}

func (db *Database) DeleteProfile(name string) error {
	_, err := db.GetProfile(name)
	if err != nil {
		return err
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(deleteProfileStmt, db.profilesTable), Profile{})
	if err != nil {
		return err
	}
	row := Profile{
		Name: name,
	}
	err = db.conn.Query(context.Background(), stmt, row).Run()
	return err
}
