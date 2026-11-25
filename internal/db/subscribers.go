// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// SPDX-FileCopyrightText: Ella Networks Inc.

package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
)

const SubscribersTableName = "subscribers"

const MaxNumSubscribers = 100000

var ErrSubscriberLimit = errors.New("maximum number of subscribers reached")

const (
	createSubscriberSQL      = "INSERT INTO " + SubscribersTableName + " (id, imsi, sequenceNumber, permanentKey, opc, profileID, description, msisdn) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
	editSubscriberProfileSQL = "UPDATE " + SubscribersTableName + " SET profileID=?, description=?, msisdn=? WHERE imsi==?"
	editSubscriberSeqNumSQL  = "UPDATE " + SubscribersTableName + " SET sequenceNumber=? WHERE imsi==?"
	casSubscriberSeqNumSQL   = "UPDATE " + SubscribersTableName + " SET sequenceNumber=? WHERE imsi==? AND sequenceNumber==?"
	deleteSubscriberSQL      = "DELETE FROM " + SubscribersTableName + " WHERE imsi==?"
	countSubscribersSQL      = "SELECT COUNT(*) FROM " + SubscribersTableName
)

const subscriberColumns = "id, imsi, sequenceNumber, permanentKey, opc, profileID, description, msisdn"

const (
	getSubscriberSQL         = "SELECT " + subscriberColumns + " from " + SubscribersTableName + " WHERE imsi==?"
	getSubscriberByMSISDNSQL = "SELECT " + subscriberColumns + " from " + SubscribersTableName + " WHERE msisdn==? AND msisdn!=''"
	listIMSIsWithMSISDNSQL   = "SELECT imsi FROM " + SubscribersTableName + " WHERE msisdn!='' AND imsi IN (SELECT value FROM json_each(?))"
	listSubscriberMSISDNsSQL = "SELECT imsi, msisdn FROM " + SubscribersTableName + " WHERE msisdn!=''"
)

const (
	subscriberSearchClause = `
    (?1 IS NULL
     OR imsi LIKE ?1 ESCAPE '\'
     OR (msisdn != '' AND '+' || msisdn LIKE ?1 ESCAPE '\')
     OR description LIKE ?1 ESCAPE '\')`

	subscriberDataNetworkClause = `
    AND (?2 IS NULL
         OR profileID IN (SELECT profileID FROM ` + PoliciesTableName + ` WHERE dataNetworkID = ?2))
    AND (?3 IS NULL
         OR imsi IN (SELECT value FROM json_each(?3)))`
)

const (
	listSubscribersFilteredSQL = `
  SELECT ` + subscriberColumns + `
  FROM ` + SubscribersTableName + `
  WHERE` + subscriberSearchClause + subscriberDataNetworkClause + `
  ORDER BY imsi
  LIMIT ?4 OFFSET ?5`

	countSubscribersFilteredSQL = `
  SELECT COUNT(*)
  FROM ` + SubscribersTableName + `
  WHERE` + subscriberSearchClause + subscriberDataNetworkClause
)

const countSubscribersAndVisitorsSQL = "SELECT COUNT(*) FROM (SELECT imsi FROM " + SubscribersTableName + " UNION ALL SELECT imsi FROM " + VisitorsTableName + ")"

type Subscriber struct {
	ID             string // UUIDv7
	Imsi           string
	SequenceNumber string
	PermanentKey   string
	Opc            string
	ProfileID      string
	Description    string
	Msisdn         string

	RoamingPartnerID *string
}

type SubscriberFilters struct {
	Search        *string
	DataNetworkID *string
	IMSIs         []string
}

type subscriberFilterArgs struct {
	Search        *string
	DataNetworkID *string
	IMSIs         *string
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`)

func (f *SubscriberFilters) args() (subscriberFilterArgs, error) {
	if f == nil {
		return subscriberFilterArgs{}, nil
	}

	args := subscriberFilterArgs{DataNetworkID: f.DataNetworkID}

	if f.IMSIs != nil {
		encoded, err := json.Marshal(f.IMSIs)
		if err != nil {
			return subscriberFilterArgs{}, fmt.Errorf("encode imsi filter: %w", err)
		}

		imsis := string(encoded)
		args.IMSIs = &imsis
	}

	if f.Search != nil {
		pattern := "%" + likeEscaper.Replace(*f.Search) + "%"
		args.Search = &pattern
	}

	return args, nil
}

func (db *Database) ListSubscribersPage(ctx context.Context, filters *SubscriberFilters, page int, perPage int) (_ []Subscriber, _ int, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+SubscribersTableName+" (paged)", SubscribersTableName, "SELECT", attribute.Int("ella.db.page", page), attribute.Int("ella.db.page_size", perPage))
	defer func() { finish(err) }()

	filterArgs, err := filters.args()
	if err != nil {
		return nil, 0, err
	}

	subs, err := queryAll(ctx, db.reader(), listSubscribersFilteredSQL, scanSubscriber, filterArgs.Search, filterArgs.DataNetworkID, filterArgs.IMSIs, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	var count int

	if filterArgs == (subscriberFilterArgs{}) {
		count, err = db.CountSubscribers(ctx)
	} else {
		count, err = db.countSubscribersFiltered(ctx, filterArgs)
	}

	if err != nil {
		return nil, 0, err
	}

	return subs, count, nil
}

func (db *Database) countSubscribersFiltered(ctx context.Context, filterArgs subscriberFilterArgs) (_ int, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+SubscribersTableName+" (filtered count)", SubscribersTableName, "SELECT")
	defer func() { finish(err) }()

	count, err := queryCount(ctx, db.reader(), countSubscribersFilteredSQL, filterArgs.Search, filterArgs.DataNetworkID, filterArgs.IMSIs)
	if err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

func (db *Database) GetSubscriber(ctx context.Context, imsi string) (_ *Subscriber, err error) {
	ctx, finish := startDBQuery(ctx, SubscribersTableName, "SELECT")
	defer func() { finish(err) }()

	row, err := db.getOwnSubscriber(ctx, db.reader(), imsi)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) getOwnSubscriber(ctx context.Context, q querier, imsi string) (Subscriber, error) {
	return scanSubscriber(q.QueryRowContext(ctx, getSubscriberSQL, imsi))
}

func (db *Database) IMSIsWithMSISDN(ctx context.Context, imsis []string) (_ map[string]bool, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+SubscribersTableName+" (with msisdn)", SubscribersTableName, "SELECT")
	defer func() { finish(err) }()

	found := make(map[string]bool, len(imsis))

	matched, err := queryAll(ctx, db.reader(), listIMSIsWithMSISDNSQL, scanValue[string], jsonArray(imsis))
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	for _, imsi := range matched {
		found[imsi] = true
	}

	return found, nil
}

func (db *Database) GetSubscriberByMSISDN(ctx context.Context, msisdn string) (_ *Subscriber, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+SubscribersTableName+" (by msisdn)", SubscribersTableName, "SELECT")
	defer func() { finish(err) }()

	if msisdn == "" {
		return nil, ErrNotFound
	}

	row, err := scanSubscriber(db.reader().QueryRowContext(ctx, getSubscriberByMSISDNSQL, msisdn))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) CreateSubscriber(ctx context.Context, subscriber *Subscriber) (err error) {
	ctx, finish := startDBQuery(ctx, SubscribersTableName, "INSERT")
	defer func() { finish(err) }()

	if subscriber.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate subscriber id: %w", err)
		}

		subscriber.ID = id.String()
	}

	_, err = opCreateSubscriber.Invoke(ctx, db, subscriber)
	if err != nil {
		return err
	}

	return nil
}

func (db *Database) UpdateSubscriberProfile(ctx context.Context, subscriber *Subscriber) (err error) {
	ctx, finish := startDBQuery(ctx, SubscribersTableName, "UPDATE")
	defer func() { finish(err) }()

	_, err = opUpdateSubscriberProfile.Invoke(ctx, db, subscriber)
	if err != nil {
		return err
	}

	return nil
}

func (db *Database) EditSubscriberSequenceNumber(ctx context.Context, imsi string, sequenceNumber string) (err error) {
	ctx, finish := startDBQueryNamed(ctx, "UPDATE "+SubscribersTableName+" (sequence number)", SubscribersTableName, "UPDATE")
	defer func() { finish(err) }()

	subscriber := &Subscriber{
		Imsi:           imsi,
		SequenceNumber: sequenceNumber,
	}

	_, err = opEditSubscriberSeqNum.Invoke(ctx, db, subscriber)
	if err != nil {
		return err
	}

	return nil
}

func (db *Database) DeleteSubscriber(ctx context.Context, imsi string) (err error) {
	ctx, finish := startDBQuery(ctx, SubscribersTableName, "DELETE")
	defer func() { finish(err) }()

	_, err = opDeleteSubscriber.Invoke(ctx, db, &stringPayload{Value: imsi})
	if err != nil {
		return err
	}

	return nil
}

func (db *Database) CountSubscribers(ctx context.Context) (_ int, err error) {
	ctx, finish := startDBQuery(ctx, SubscribersTableName, "SELECT")
	defer func() { finish(err) }()

	count, err := queryCount(ctx, db.reader(), countSubscribersSQL)
	if err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

func (db *Database) CountSubscribersAndVisitors(ctx context.Context) (_ int, err error) {
	querySummary := fmt.Sprintf("%s %s (with %s)", "SELECT", SubscribersTableName, VisitorsTableName)

	ctx, finish := startDBQueryNamed(ctx, querySummary, SubscribersTableName, "SELECT")
	defer func() { finish(err) }()

	n, err := db.countSubscribersAndVisitors(ctx, db.reader())
	if err != nil {
		return 0, err
	}

	return n, nil
}

func (db *Database) countSubscribersAndVisitors(ctx context.Context, q querier) (int, error) {
	count, err := queryCount(ctx, q, countSubscribersAndVisitorsSQL)
	if err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

func (db *Database) checkSubscriberCapacity(ctx context.Context) error {
	n, err := db.subscriberCount(ctx, db.runner(ctx))
	if err != nil {
		return err
	}

	if n >= db.subscriberCap() {
		return fmt.Errorf("%w (%d)", ErrSubscriberLimit, db.subscriberCap())
	}

	return nil
}

func (db *Database) subscriberCap() int {
	if db.maxSubscribers > 0 {
		return db.maxSubscribers
	}

	return MaxNumSubscribers
}

func scanSubscriber(row scanner) (Subscriber, error) {
	var v Subscriber

	if err := row.Scan(&v.ID, &v.Imsi, &v.SequenceNumber, &v.PermanentKey, &v.Opc, &v.ProfileID, &v.Description, &v.Msisdn); err != nil {
		return Subscriber{}, err
	}

	return v, nil
}

func scanSubscriberMSISDN(row scanner) (Subscriber, error) {
	var v Subscriber

	if err := row.Scan(&v.Imsi, &v.Msisdn); err != nil {
		return Subscriber{}, err
	}

	return v, nil
}
