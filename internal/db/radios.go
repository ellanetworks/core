// Copyright 2024 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/canonical/sqlair"
)

const RadiosTableName = "radios"

const QueryCreateRadiosTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		name TEXT NOT NULL,
		tac TEXT NOT NULL
)`

const (
	listRadiosStmt  = "SELECT &Radio.* from %s"
	getRadioStmt    = "SELECT &Radio.* from %s WHERE name==$Radio.name"
	createRadioStmt = "INSERT INTO %s (name, tac) VALUES ($Radio.name, $Radio.tac)"
	editRadioStmt   = "UPDATE %s SET tac=$Radio.tac WHERE name==$Radio.name"
	deleteRadioStmt = "DELETE FROM %s WHERE name==$Radio.name"
)

type Radio struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
	Tac  string `db:"tac"`
}

func (db *Database) ListRadios() ([]Radio, error) {
	stmt, err := sqlair.Prepare(fmt.Sprintf(listRadiosStmt, db.radiosTable), Radio{})
	if err != nil {
		return nil, err
	}
	var radios []Radio
	err = db.conn.Query(context.Background(), stmt).GetAll(&radios)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return radios, nil
}

func (db *Database) GetRadio(name string) (*Radio, error) {
	row := Radio{
		Name: name,
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(getRadioStmt, db.radiosTable), Radio{})
	if err != nil {
		return nil, err
	}
	err = db.conn.Query(context.Background(), stmt, row).Get(&row)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (db *Database) CreateRadio(radio *Radio) error {
	_, err := db.GetRadio(radio.Name)
	if err == nil {
		return fmt.Errorf("radio with name %s already exists", radio.Name)
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(createRadioStmt, db.radiosTable), Radio{})
	if err != nil {
		return err
	}
	err = db.conn.Query(context.Background(), stmt, radio).Run()
	return err
}

func (db *Database) UpdateRadio(radio *Radio) error {
	_, err := db.GetRadio(radio.Name)
	if err != nil {
		return err
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(editRadioStmt, db.radiosTable), Radio{})
	if err != nil {
		return err
	}
	err = db.conn.Query(context.Background(), stmt, radio).Run()
	return err
}

func (db *Database) DeleteRadio(name string) error {
	_, err := db.GetRadio(name)
	if err != nil {
		return err
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(deleteRadioStmt, db.radiosTable), Radio{})
	if err != nil {
		return err
	}
	row := Radio{
		Name: name,
	}
	err = db.conn.Query(context.Background(), stmt, row).Run()
	return err
}
