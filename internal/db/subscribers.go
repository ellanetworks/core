// Copyright 2024 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
	"net"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const SubscribersTableName = "subscribers"

const QueryCreateSubscribersTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		imsi TEXT NOT NULL,

		ipAddress TEXT,

		sequenceNumber TEXT NOT NULL,
		permanentKey TEXT NOT NULL,
		opc TEXT NOT NULL,

		profileID INTEGER NOT NULL,
    	FOREIGN KEY (profileID) REFERENCES profiles (id)
)`

const (
	listSubscribersStmt   = "SELECT &Subscriber.* from %s"
	getSubscriberStmt     = "SELECT &Subscriber.* from %s WHERE imsi==$Subscriber.imsi"
	createSubscriberStmt  = "INSERT INTO %s (imsi, ipAddress, sequenceNumber, permanentKey, opc, profileID) VALUES ($Subscriber.imsi, $Subscriber.ipAddress, $Subscriber.sequenceNumber, $Subscriber.permanentKey, $Subscriber.opc, $Subscriber.profileID)"
	editSubscriberStmt    = "UPDATE %s SET ipAddress=$Subscriber.ipAddress, sequenceNumber=$Subscriber.sequenceNumber, permanentKey=$Subscriber.permanentKey, opc=$Subscriber.opc, profileID=$Subscriber.profileID WHERE imsi==$Subscriber.imsi"
	deleteSubscriberStmt  = "DELETE FROM %s WHERE imsi==$Subscriber.imsi"
	getNumSubscribersStmt = "SELECT COUNT(*) AS &NumSubscribers.count FROM %s"
	checkIPStmt           = "SELECT &Subscriber.* FROM %s WHERE ipAddress=$Subscriber.ipAddress"
	allocateIPStmt        = "UPDATE %s SET ipAddress=$Subscriber.ipAddress WHERE imsi=$Subscriber.imsi"
	releaseIPStmt         = "UPDATE %s SET ipAddress=NULL WHERE imsi=$Subscriber.imsi"
)

type NumSubscribers struct {
	Count int `db:"count"`
}

type Subscriber struct {
	ID             int    `db:"id"`
	Imsi           string `db:"imsi"`
	IPAddress      string `db:"ipAddress"`
	SequenceNumber string `db:"sequenceNumber"`
	PermanentKey   string `db:"permanentKey"`
	Opc            string `db:"opc"`
	ProfileID      int    `db:"profileID"`
}

// ListSubscribers returns all of the subscribers and their fields available in the database.
func (db *Database) ListSubscribers(ctx context.Context) ([]Subscriber, error) {
	ctx, span := tracer.Start(ctx, "ListSubscribers")
	defer span.End()
	stmt, err := sqlair.Prepare(fmt.Sprintf(listSubscribersStmt, db.subscribersTable), Subscriber{})
	if err != nil {
		return nil, err
	}
	var subscribers []Subscriber
	err = db.conn.Query(ctx, stmt).GetAll(&subscribers)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return subscribers, nil
}

func (db *Database) GetSubscriber(imsi string, ctx context.Context) (*Subscriber, error) {
	ctx, span := tracer.Start(ctx, "GetSubscriber")
	defer span.End()
	row := Subscriber{
		Imsi: imsi,
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(getSubscriberStmt, db.subscribersTable), Subscriber{})
	if err != nil {
		return nil, err
	}
	err = db.conn.Query(ctx, stmt, row).Get(&row)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (db *Database) CreateSubscriber(subscriber *Subscriber, ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "CreateSubscriber")
	defer span.End()
	_, err := db.GetSubscriber(subscriber.Imsi, ctx)
	if err == nil {
		return fmt.Errorf("subscriber with imsi %s already exists", subscriber.Imsi)
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(createSubscriberStmt, db.subscribersTable), Subscriber{})
	if err != nil {
		return err
	}
	err = db.conn.Query(ctx, stmt, subscriber).Run()
	return err
}

func (db *Database) UpdateSubscriber(subscriber *Subscriber, ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "UpdateSubscriber")
	defer span.End()
	_, err := db.GetSubscriber(subscriber.Imsi, ctx)
	if err != nil {
		return err
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(editSubscriberStmt, db.subscribersTable), Subscriber{})
	if err != nil {
		return err
	}
	err = db.conn.Query(ctx, stmt, subscriber).Run()
	return err
}

func (db *Database) DeleteSubscriber(imsi string, ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "DeleteSubscriber")
	defer span.End()
	_, err := db.GetSubscriber(imsi, ctx)
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
	err = db.conn.Query(ctx, stmt, row).Run()
	logger.DBLog.Info("Deleted subscriber", zap.String("imsi", imsi))
	return err
}

func (db *Database) SubscribersInProfile(name string, ctx context.Context) (bool, error) {
	ctx, span := tracer.Start(ctx, "SubscribersInProfile")
	defer span.End()
	profile, err := db.GetProfile(name, ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get profile with name %s: %v", name, err)
	}

	allSubscribers, err := db.ListSubscribers(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to list subscribers: %v", err)
	}

	for _, subscriber := range allSubscribers {
		if subscriber.ProfileID == profile.ID {
			return true, nil
		}
	}

	return false, nil
}

func (db *Database) AllocateIP(imsi string, ctx context.Context) (net.IP, error) {
	ctx, span := tracer.Start(ctx, "AllocateIP")
	defer span.End()
	subscriber, err := db.GetSubscriber(imsi, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriber: %v", err)
	}

	profile, err := db.GetProfileByID(subscriber.ProfileID, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile for subscriber %s: %v", imsi, err)
	}

	_, ipNet, err := net.ParseCIDR(profile.UeIPPool)
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
		err = db.conn.Query(ctx, stmt, Subscriber{IPAddress: ipStr}).Get(&existing)
		if err == sql.ErrNoRows {
			// IP is not allocated, assign it to the subscriber
			subscriber.IPAddress = ipStr
			stmt, err := sqlair.Prepare(fmt.Sprintf(allocateIPStmt, SubscribersTableName), Subscriber{})
			if err != nil {
				return nil, fmt.Errorf("failed to prepare IP allocation statement: %v", err)
			}
			err = db.conn.Query(ctx, stmt, subscriber).Run()
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

func (db *Database) ReleaseIP(imsi string, ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "ReleaseIP")
	defer span.End()
	subscriber, err := db.GetSubscriber(imsi, ctx)
	if err != nil {
		return fmt.Errorf("failed to get subscriber: %v", err)
	}

	if subscriber.IPAddress == "" {
		return nil
	}

	stmt, err := sqlair.Prepare(fmt.Sprintf(releaseIPStmt, SubscribersTableName), Subscriber{})
	if err != nil {
		return fmt.Errorf("failed to prepare IP release statement: %v", err)
	}

	err = db.conn.Query(ctx, stmt, subscriber).Run()
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

func (db *Database) NumSubscribers(ctx context.Context) (int, error) {
	ctx, span := tracer.Start(ctx, "NumSubscribers")
	defer span.End()
	stmt, err := sqlair.Prepare(fmt.Sprintf(getNumSubscribersStmt, db.subscribersTable), NumSubscribers{})
	if err != nil {
		return 0, fmt.Errorf("failed to prepare statement: %v", err)
	}
	result := NumSubscribers{}
	err = db.conn.Query(ctx, stmt).Get(&result)
	if err != nil {
		return 0, fmt.Errorf("failed to get number of subscribers: %v", err)
	}
	return result.Count, nil
}
