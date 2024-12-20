package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/canonical/sqlair"
	"github.com/yeastengine/ella/internal/logger"
)

const SubscribersTableName = "subscribers"

const QueryCreateSubscribersTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		ueId TEXT NOT NULL,

		sequenceNumber TEXT NOT NULL,
		permanentKeyValue TEXT NOT NULL,
		opcValue TEXT NOT NULL,

		profileID INTEGER NOT NULL,
    	FOREIGN KEY (profileID) REFERENCES profiles (id)
)`

const (
	listSubscribersStmt            = "SELECT &Subscriber.* from %s"
	getSubscriberStmt              = "SELECT &Subscriber.* from %s WHERE ueId==$Subscriber.ueId"
	createSubscriberStmt           = "INSERT INTO %s (ueId, sequenceNumber, permanentKeyValue, opcValue, profileID) VALUES ($Subscriber.ueId, $Subscriber.sequenceNumber, $Subscriber.permanentKeyValue, $Subscriber.opcValue, $Subscriber.profileID)"
	updateSubscriberSequenceNumber = "UPDATE %s SET sequenceNumber=$Subscriber.sequenceNumber WHERE ueId==$Subscriber.ueId"
	updateSubscriberProfile        = "UPDATE %s SET profileID=$Subscriber.profileID WHERE ueId==$Subscriber.ueId"
	deleteSubscriberStmt           = "DELETE FROM %s WHERE ueId==$Subscriber.ueId"
)

type Subscriber struct {
	ID int `db:"id"`

	UeId string `db:"ueId"`

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

func (db *Database) GetSubscriber(ueID string) (*Subscriber, error) {
	row := Subscriber{
		UeId: ueID,
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
	_, err := db.GetSubscriber(subscriber.UeId)
	if err == nil {
		return fmt.Errorf("subscriber with ueId %s already exists", subscriber.UeId)
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(createSubscriberStmt, db.subscribersTable), Subscriber{})
	if err != nil {
		return err
	}
	err = db.conn.Query(context.Background(), stmt, subscriber).Run()
	return err
}

func (db *Database) UpdateSubscriberSequenceNumber(ueID string, sequenceNumber string) error {
	_, err := db.GetSubscriber(ueID)
	if err != nil {
		return err
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(updateSubscriberSequenceNumber, db.subscribersTable), Subscriber{})
	if err != nil {
		return err
	}
	row := Subscriber{
		UeId:           ueID,
		SequenceNumber: sequenceNumber,
	}
	err = db.conn.Query(context.Background(), stmt, row).Run()
	logger.DBLog.Infof("Updated sequence number for subscriber with ueID %d", ueID)
	return err
}

func (db *Database) UpdateSubscriberProfile(ueID string, profileName string) error {
	profile, err := db.GetProfile(profileName)
	if err != nil {
		return fmt.Errorf("profile with name %s not found", profileName)
	}

	subscriber, err := db.GetSubscriber(ueID)
	if err != nil {
		return fmt.Errorf("subscriber with ueID %s not found: %v", ueID, err)
	}

	stmt, err := sqlair.Prepare(fmt.Sprintf(updateSubscriberProfile, db.subscribersTable), Subscriber{})
	if err != nil {
		return fmt.Errorf("failed to prepare update query: %v", err)
	}

	row := Subscriber{
		UeId:              ueID,
		SequenceNumber:    subscriber.SequenceNumber,
		PermanentKeyValue: subscriber.PermanentKeyValue,
		OpcValue:          subscriber.OpcValue,
		ProfileID:         profile.ID,
	}

	err = db.conn.Query(context.Background(), stmt, row).Run()
	if err != nil {
		return fmt.Errorf("failed to update profile for subscriber with ueID %s: %v", ueID, err)
	}

	logger.DBLog.Infof("Updated profile for subscriber with ueID %s to profile %s (ID: %d)", ueID, profileName, profile.ID)
	return nil
}

func (db *Database) DeleteSubscriber(ueID string) error {
	_, err := db.GetSubscriber(ueID)
	if err != nil {
		return fmt.Errorf("subscriber with ueID %s not found", ueID)
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(deleteSubscriberStmt, db.subscribersTable), Subscriber{})
	if err != nil {
		return err
	}
	row := Subscriber{
		UeId: ueID,
	}
	err = db.conn.Query(context.Background(), stmt, row).Run()
	logger.DBLog.Infof("Deleted subscriber with ueID %s", ueID)
	return err
}
