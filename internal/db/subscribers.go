package db

import (
	"context"
	"database/sql"
	"fmt"
	"net"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/logger"
)

const SubscribersTableName = "subscribers"

const QueryCreateSubscribersTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		imsi TEXT NOT NULL,

		ipAddress TEXT,

		sequenceNumber TEXT NOT NULL,
		permanentKeyValue TEXT NOT NULL,
		opcValue TEXT NOT NULL,

		profileID INTEGER NOT NULL,
    	FOREIGN KEY (profileID) REFERENCES profiles (id)
)`

const (
	listSubscribersStmt  = "SELECT &Subscriber.* from %s"
	getSubscriberStmt    = "SELECT &Subscriber.* from %s WHERE imsi==$Subscriber.imsi"
	createSubscriberStmt = "INSERT INTO %s (imsi, ipAddress, sequenceNumber, permanentKeyValue, opcValue, profileID) VALUES ($Subscriber.imsi, $Subscriber.ipAddress, $Subscriber.sequenceNumber, $Subscriber.permanentKeyValue, $Subscriber.opcValue, $Subscriber.profileID)"
	editSubscriberStmt   = "UPDATE %s SET ipAddress=$Subscriber.ipAddress, sequenceNumber=$Subscriber.sequenceNumber, permanentKeyValue=$Subscriber.permanentKeyValue, opcValue=$Subscriber.opcValue, profileID=$Subscriber.profileID WHERE imsi==$Subscriber.imsi"
	deleteSubscriberStmt = "DELETE FROM %s WHERE imsi==$Subscriber.imsi"
	checkIPStmt          = "SELECT &Subscriber.* FROM %s WHERE ipAddress=$Subscriber.ipAddress"
	allocateIPStmt       = "UPDATE %s SET ipAddress=$Subscriber.ipAddress WHERE imsi=$Subscriber.imsi"
	releaseIPStmt        = "UPDATE %s SET ipAddress=NULL WHERE imsi=$Subscriber.imsi"
)

type Subscriber struct {
	ID int `db:"id"`

	Imsi string `db:"imsi"`

	IpAddress string `db:"ipAddress"`

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

func (db *Database) UpdateSubscriber(subscriber *Subscriber) error {
	_, err := db.GetSubscriber(subscriber.Imsi)
	if err != nil {
		return err
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(editSubscriberStmt, db.subscribersTable), Subscriber{})
	if err != nil {
		return err
	}
	err = db.conn.Query(context.Background(), stmt, subscriber).Run()
	return err
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

func (db *Database) AllocateIP(imsi string) (net.IP, error) {
	subscriber, err := db.GetSubscriber(imsi)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriber: %v", err)
	}

	profile, err := db.GetProfileByID(subscriber.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile for subscriber %s: %v", imsi, err)
	}

	_, ipNet, err := net.ParseCIDR(profile.UeIpPool)
	if err != nil {
		return nil, fmt.Errorf("invalid IP pool in profile %s: %v", profile.Name, err)
	}

	baseIP := ipNet.IP
	maskBits, totalBits := ipNet.Mask.Size()
	totalIPs := 1 << (totalBits - maskBits)

	for i := 1; i < totalIPs-1; i++ { // Skip network and broadcast addresses
		ip := addOffsetToIP(baseIP, i)
		ipStr := ip.String()

		stmt, err := sqlair.Prepare(fmt.Sprintf(checkIPStmt, SubscribersTableName), Subscriber{})
		if err != nil {
			return nil, fmt.Errorf("failed to prepare IP check statement: %v", err)
		}
		var existing Subscriber
		err = db.conn.Query(context.Background(), stmt, Subscriber{IpAddress: ipStr}).Get(&existing)
		if err == sql.ErrNoRows {
			// IP is not allocated, assign it to the subscriber
			subscriber.IpAddress = ipStr
			stmt, err := sqlair.Prepare(fmt.Sprintf(allocateIPStmt, SubscribersTableName), Subscriber{})
			if err != nil {
				return nil, fmt.Errorf("failed to prepare IP allocation statement: %v", err)
			}
			err = db.conn.Query(context.Background(), stmt, subscriber).Run()
			if err != nil {
				return nil, fmt.Errorf("failed to allocate IP: %v", err)
			}
			return ip, nil
		} else if err != nil {
			return nil, fmt.Errorf("failed to check IP availability: %v", err)
		}
	}

	return nil, fmt.Errorf("no available IP addresses")
}

func (db *Database) ReleaseIP(imsi string) error {
	subscriber, err := db.GetSubscriber(imsi)
	if err != nil {
		return fmt.Errorf("failed to get subscriber: %v", err)
	}

	if subscriber.IpAddress == "" {
		return nil
	}

	stmt, err := sqlair.Prepare(fmt.Sprintf(releaseIPStmt, SubscribersTableName), Subscriber{})
	if err != nil {
		return fmt.Errorf("failed to prepare IP release statement: %v", err)
	}

	err = db.conn.Query(context.Background(), stmt, subscriber).Run()
	if err != nil {
		return fmt.Errorf("failed to release IP: %v", err)
	}

	return nil
}

func addOffsetToIP(baseIP net.IP, offset int) net.IP {
	resultIP := make(net.IP, len(baseIP))
	copy(resultIP, baseIP)

	for i := len(resultIP) - 1; i >= 0; i-- {
		offset += int(resultIP[i])
		resultIP[i] = byte(offset)
		offset >>= 8
	}

	return resultIP
}
