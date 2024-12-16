package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/canonical/sqlair"
)

const SubscribersTableName = "subscribers"

const QueryCreateSubscribersTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		ueId TEXT NOT NULL,
		plmnID TEXT NOT NULL,
		sst INTEGER,
		sd TEXT,
		dnn TEXT,

		sequenceNumber TEXT NOT NULL,
		permanentKeyValue TEXT NOT NULL,
		opcValue TEXT NOT NULL,

		uplink TEXT,
		downlink TEXT,
		var5qi INTEGER,
		priorityLevel INTEGER
)`

// UpdateSubscriberProfile should include the following fields:
// dnn, sd, sst, plmnId, bitRateUplink, bitRateDownlink, var5qi, priorityLevel

const (
	listSubscribersStmt            = "SELECT &Subscriber.* from %s"
	getSubscriberStmt              = "SELECT &Subscriber.* from %s WHERE id==$Subscriber.id or ueId==$Subscriber.ueId"
	createSubscriberStmt           = "INSERT INTO %s (ueId, plmnID, sequenceNumber, permanentKeyValue, opcValue) VALUES ($Subscriber.ueId, $Subscriber.plmnID, $Subscriber.sequenceNumber, $Subscriber.permanentKeyValue, $Subscriber.opcValue)"
	updateSubscriberSequenceNumber = "UPDATE %s SET sequenceNumber=$Subscriber.sequenceNumber WHERE id==$Subscriber.id"
	updateSubscriberProfile        = "UPDATE %s SET dnn=$Subscriber.dnn, sd=$Subscriber.sd, sst=$Subscriber.sst, plmnID=$Subscriber.plmnID, uplink=$Subscriber.uplink, downlink=$Subscriber.downlink, var5qi=$Subscriber.var5qi, priorityLevel=$Subscriber.priorityLevel WHERE id==$Subscriber.id"
	deleteSubscriberStmt           = "DELETE FROM %s WHERE id==$Subscriber.id"
)

type Subscriber struct {
	ID int `db:"id"`

	UeId   string `db:"ueId"`
	PlmnID string `db:"plmnID"`
	Sst    int32  `db:"sst"`
	Sd     string `db:"sd"`
	Dnn    string `db:"dnn"`

	SequenceNumber    string `db:"sequenceNumber"`
	PermanentKeyValue string `db:"permanentKeyValue"`
	OpcValue          string `db:"opcValue"`

	BitRateUplink   string `db:"uplink"`
	BitRateDownlink string `db:"downlink"`
	Var5qi          int32  `db:"var5qi"`
	PriorityLevel   int32  `db:"priorityLevel"`
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

// GetSubscriberByID retrieves the name, password and the permission level of a user.
func (db *Database) GetSubscriberByID(id int) (*Subscriber, error) {
	row := Subscriber{
		ID: id,
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

func (db *Database) GetSubscriberByUeID(ueID string) (*Subscriber, error) {
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
	_, err := db.GetSubscriberByUeID(subscriber.UeId)
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

func (db *Database) UpdateSubscriberSequenceNumber(id int, sequenceNumber string) error {
	_, err := db.GetSubscriberByID(id)
	if err != nil {
		return err
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(updateSubscriberSequenceNumber, db.subscribersTable), Subscriber{})
	if err != nil {
		return err
	}
	row := Subscriber{
		ID:             id,
		SequenceNumber: sequenceNumber,
	}
	err = db.conn.Query(context.Background(), stmt, row).Run()
	return err
}

func (db *Database) UpdateSubscriberProfile(id int, dnn string, sd string, sst int32, plmnId string, bitrateUplink string, bitrateDownlink string, var5qi int, priorityLevel int) error {
	_, err := db.GetSubscriberByID(id)
	if err != nil {
		return err
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(updateSubscriberProfile, db.subscribersTable), Subscriber{})
	if err != nil {
		return err
	}
	row := Subscriber{
		ID:              id,
		Dnn:             dnn,
		Sd:              sd,
		Sst:             sst,
		PlmnID:          plmnId,
		BitRateUplink:   bitrateUplink,
		BitRateDownlink: bitrateDownlink,
		Var5qi:          int32(var5qi),
		PriorityLevel:   int32(priorityLevel),
	}
	err = db.conn.Query(context.Background(), stmt, row).Run()
	return err
}

func (db *Database) DeleteSubscriber(id int) error {
	_, err := db.GetSubscriberByID(id)
	if err != nil {
		return err
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(deleteSubscriberStmt, db.subscribersTable), Subscriber{})
	if err != nil {
		return err
	}
	row := Subscriber{
		ID: id,
	}
	err = db.conn.Query(context.Background(), stmt, row).Run()
	return err
}
