package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/logger"
)

const SubscribersTableName = "subscribers"

const QueryCreateSubscribersTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		imsi TEXT NOT NULL,

		sequenceNumber TEXT NOT NULL,
		permanentKeyValue TEXT NOT NULL,
		opcValue TEXT NOT NULL,

		profileID INTEGER NOT NULL,
    	FOREIGN KEY (profileID) REFERENCES profiles (id)
)`

const (
	listSubscribersStmt            = "SELECT &Subscriber.* from %s"
	getSubscriberStmt              = "SELECT &Subscriber.* from %s WHERE imsi==$Subscriber.imsi"
	createSubscriberStmt           = "INSERT INTO %s (imsi, sequenceNumber, permanentKeyValue, opcValue, profileID) VALUES ($Subscriber.imsi, $Subscriber.sequenceNumber, $Subscriber.permanentKeyValue, $Subscriber.opcValue, $Subscriber.profileID)"
	updateSubscriberSequenceNumber = "UPDATE %s SET sequenceNumber=$Subscriber.sequenceNumber WHERE imsi==$Subscriber.imsi"
	updateSubscriberProfile        = "UPDATE %s SET profileID=$Subscriber.profileID WHERE imsi==$Subscriber.imsi"
	deleteSubscriberStmt           = "DELETE FROM %s WHERE imsi==$Subscriber.imsi"
)

type Subscriber struct {
	ID int `db:"id"`

	Imsi string `db:"imsi"`

	SequenceNumber    string `db:"sequenceNumber"`
	PermanentKeyValue string `db:"permanentKeyValue"`
	OpcValue          string `db:"opcValue"`

	ProfileID int `db:"profileID"`
}

// ListSubscribers returns all of the subscribers and their fields available in the database.
func (db *Database) ListSubscribers() ([]Subscriber, error) {
	stmt, err := sqlair.Prepare(fmt.Sprintf(listSubscribersStmt, db.subscribersTable), Subscriber{})
	if err != nil {
		return nil, err
	}
	var subscribers []Subscriber
	err = db.conn.Query(context.Background(), stmt).GetAll(&subscribers)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return subscribers, nil
}

func (db *Database) GetSubscriber(imsi string) (*Subscriber, error) {
	row := Subscriber{
		Imsi: imsi,
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(getSubscriberStmt, db.subscribersTable), Subscriber{})
	if err != nil {
		return nil, err
	}
	err = db.conn.Query(context.Background(), stmt, row).Get(&row)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (db *Database) CreateSubscriber(subscriber *Subscriber) error {
	_, err := db.GetSubscriber(subscriber.Imsi)
	if err == nil {
		return fmt.Errorf("subscriber with imsi %s already exists", subscriber.Imsi)
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(createSubscriberStmt, db.subscribersTable), Subscriber{})
	if err != nil {
		return err
	}
	err = db.conn.Query(context.Background(), stmt, subscriber).Run()
	return err
}

func (db *Database) UpdateSubscriberSequenceNumber(imsi string, sequenceNumber string) error {
	_, err := db.GetSubscriber(imsi)
	if err != nil {
		return err
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(updateSubscriberSequenceNumber, db.subscribersTable), Subscriber{})
	if err != nil {
		return err
	}
	row := Subscriber{
		Imsi:           imsi,
		SequenceNumber: sequenceNumber,
	}
	err = db.conn.Query(context.Background(), stmt, row).Run()
	logger.DBLog.Infof("Updated sequence number for subscriber with Imsi %d", imsi)
	return err
}

func (db *Database) UpdateSubscriberProfile(imsi string, profileName string) error {
	profile, err := db.GetProfile(profileName)
	if err != nil {
		return fmt.Errorf("profile with name %s not found", profileName)
	}

	subscriber, err := db.GetSubscriber(imsi)
	if err != nil {
		return fmt.Errorf("subscriber with Imsi %s not found: %v", imsi, err)
	}

	stmt, err := sqlair.Prepare(fmt.Sprintf(updateSubscriberProfile, db.subscribersTable), Subscriber{})
	if err != nil {
		return fmt.Errorf("failed to prepare update query: %v", err)
	}

	row := Subscriber{
		Imsi:              imsi,
		SequenceNumber:    subscriber.SequenceNumber,
		PermanentKeyValue: subscriber.PermanentKeyValue,
		OpcValue:          subscriber.OpcValue,
		ProfileID:         profile.ID,
	}

	err = db.conn.Query(context.Background(), stmt, row).Run()
	if err != nil {
		return fmt.Errorf("failed to update profile for subscriber with Imsi %s: %v", imsi, err)
	}

	logger.DBLog.Infof("Updated profile for subscriber with Imsi %s to profile %s (ID: %d)", imsi, profileName, profile.ID)
	return nil
}

func (db *Database) DeleteSubscriber(imsi string) error {
	_, err := db.GetSubscriber(imsi)
	if err != nil {
		return fmt.Errorf("subscriber with imsi %s not found", imsi)
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(deleteSubscriberStmt, db.subscribersTable), Subscriber{})
	if err != nil {
		return err
	}
	row := Subscriber{
		Imsi: imsi,
	}
	err = db.conn.Query(context.Background(), stmt, row).Run()
	logger.DBLog.Infof("Deleted subscriber with Imsi %s", imsi)
	return err
}
