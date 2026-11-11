// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	IMSRegistrationsTableName    = "ims_registrations"
	IMSRegistrationSetsTableName = "ims_registration_sets"
)

const imsRegistrationsSchema = 25

const (
	getIMSRegistrationSQL         = "SELECT imsi, serverName, originHost, originRealm, privateIdentity FROM " + IMSRegistrationsTableName + " WHERE imsi==?"
	listAllIMSRegistrationsSQL    = "SELECT imsi, serverName, originHost, originRealm, privateIdentity FROM " + IMSRegistrationsTableName + " ORDER BY imsi ASC"
	upsertIMSRegistrationSQL      = "INSERT INTO " + IMSRegistrationsTableName + " (imsi, serverName, originHost, originRealm, privateIdentity) VALUES (?, ?, ?, ?, ?) ON CONFLICT(imsi) DO UPDATE SET serverName=excluded.serverName, originHost=excluded.originHost, originRealm=excluded.originRealm, privateIdentity=excluded.privateIdentity"
	deleteIMSRegistrationSQL      = "DELETE FROM " + IMSRegistrationsTableName + " WHERE imsi==?"
	listIMSRegistrationSetsSQL    = "SELECT imsi, registrationSet, state, authPending, defaultIdentity, profileDigest, emergency FROM " + IMSRegistrationSetsTableName + " WHERE imsi==? ORDER BY registrationSet ASC"
	listAllIMSRegistrationSetsSQL = "SELECT imsi, registrationSet, state, authPending, defaultIdentity, profileDigest, emergency FROM " + IMSRegistrationSetsTableName + " ORDER BY imsi ASC, registrationSet ASC"
	insertIMSRegistrationSetSQL   = "INSERT INTO " + IMSRegistrationSetsTableName + " (imsi, registrationSet, state, authPending, defaultIdentity, profileDigest, emergency) VALUES (?, ?, ?, ?, ?, ?, ?)"
	deleteIMSRegistrationSetsSQL  = "DELETE FROM " + IMSRegistrationSetsTableName + " WHERE imsi==?"
	countIMSRegistrationsSQL      = "SELECT COUNT(DISTINCT imsi) FROM " + IMSRegistrationSetsTableName + " WHERE state==?"
)

type IMSRegistrationState int

const (
	IMSNotRegistered IMSRegistrationState = 0
	IMSRegistered    IMSRegistrationState = 1
	IMSUnregistered  IMSRegistrationState = 2
)

type IMSRegistrationSet string

const (
	IMSRegistrationSetMMTel IMSRegistrationSet = "mmtel"
	IMSRegistrationSetMCPTT IMSRegistrationSet = "mcptt"
)

var IMSRegistrationSets = []IMSRegistrationSet{IMSRegistrationSetMMTel, IMSRegistrationSetMCPTT}

func (s IMSRegistrationSet) Validate() error {
	if !slices.Contains(IMSRegistrationSets, s) {
		return fmt.Errorf("unknown IMS registration set %q", s)
	}

	return nil
}

type IMSRegistration struct {
	IMSI            string                                    `json:"imsi"`
	ServerName      string                                    `json:"serverName"`
	OriginHost      string                                    `json:"originHost"`
	OriginRealm     string                                    `json:"originRealm"`
	PrivateIdentity string                                    `json:"privateIdentity"`
	Sets            map[IMSRegistrationSet]IMSSetRegistration `json:"sets"`
}

type IMSSetRegistration struct {
	State           IMSRegistrationState `json:"state"`
	AuthPending     bool                 `json:"authPending"`
	DefaultIdentity string               `json:"defaultIdentity"`
	ProfileDigest   string               `json:"profileDigest"`
	Emergency       bool                 `json:"emergency"`
}

type imsRegistrationRow struct {
	Imsi            string
	ServerName      string
	OriginHost      string
	OriginRealm     string
	PrivateIdentity string
}

type imsRegistrationSetRow struct {
	Imsi            string
	RegistrationSet string
	State           int
	AuthPending     bool
	DefaultIdentity string
	ProfileDigest   string
	Emergency       bool
}

type imsRegistrationCASPayload struct {
	IMSI     string           `json:"imsi"`
	Expected *IMSRegistration `json:"expected,omitempty"`
	Next     *IMSRegistration `json:"next,omitempty"`
}

type imsRegistrationCASResult struct {
	Swapped bool             `json:"swapped"`
	Current *IMSRegistration `json:"current,omitempty"`
}

func imsRegistrationOf(r imsRegistrationRow, sets []imsRegistrationSetRow) *IMSRegistration {
	reg := &IMSRegistration{
		IMSI:            r.Imsi,
		ServerName:      r.ServerName,
		OriginHost:      r.OriginHost,
		OriginRealm:     r.OriginRealm,
		PrivateIdentity: r.PrivateIdentity,
		Sets:            make(map[IMSRegistrationSet]IMSSetRegistration, len(sets)),
	}

	for _, s := range sets {
		reg.Sets[IMSRegistrationSet(s.RegistrationSet)] = IMSSetRegistration{
			State:           IMSRegistrationState(s.State),
			AuthPending:     s.AuthPending,
			DefaultIdentity: s.DefaultIdentity,
			ProfileDigest:   s.ProfileDigest,
			Emergency:       s.Emergency,
		}
	}

	return reg
}

func sameIMSRegistration(a, b *IMSRegistration) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	return a.ServerName == b.ServerName &&
		a.OriginHost == b.OriginHost &&
		a.OriginRealm == b.OriginRealm &&
		a.PrivateIdentity == b.PrivateIdentity &&
		maps.Equal(a.Sets, b.Sets)
}

func validateIMSRegistration(r *IMSRegistration) error {
	if len(r.Sets) == 0 {
		return errors.New("an IMS registration needs at least one registration set")
	}

	for set, s := range r.Sets {
		if err := set.Validate(); err != nil {
			return err
		}

		if s.State != IMSNotRegistered && r.PrivateIdentity == "" {
			return fmt.Errorf("registration set %s is in state %d without a private identity", set, s.State)
		}
	}

	return nil
}

func (db *Database) ListAllIMSRegistrations(ctx context.Context) ([]IMSRegistration, error) {
	ctx, span := startIMSRegistrationsSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSRegistrationsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSRegistrationsTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, imsRegistrationsSchema) {
		return nil, nil
	}

	tx, err := db.reader().BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("begin transaction failed: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	rows, err := queryAll(ctx, tx, listAllIMSRegistrationsSQL, scanIMSRegistrationRow)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	setRows, err := queryAll(ctx, tx, listAllIMSRegistrationSetsSQL, scanIMSRegistrationSetRow)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	sets := make(map[string][]imsRegistrationSetRow, len(rows))
	for _, s := range setRows {
		sets[s.Imsi] = append(sets[s.Imsi], s)
	}

	regs := make([]IMSRegistration, 0, len(rows))

	for _, r := range rows {
		regs = append(regs, *imsRegistrationOf(r, sets[r.Imsi]))
	}

	return regs, nil
}

func (db *Database) GetIMSRegistration(ctx context.Context, imsi string) (*IMSRegistration, error) {
	ctx, span := startIMSRegistrationsSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSRegistrationsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSRegistrationsTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, imsRegistrationsSchema) {
		return nil, nil
	}

	tx, err := db.reader().BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("begin transaction failed: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	reg, err := db.getIMSRegistration(ctx, tx, imsi)
	if err != nil {
		recordSpanError(span, err)

		return nil, err
	}

	return reg, nil
}

func (db *Database) CountIMSRegistrations(ctx context.Context, state IMSRegistrationState) (int, error) {
	ctx, span := startIMSRegistrationsSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSRegistrationsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSRegistrationsTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, imsRegistrationsSchema) {
		return 0, nil
	}

	count, err := queryCount(ctx, db.reader(), countIMSRegistrationsSQL, int(state))
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

func (db *Database) CompareAndSwapIMSRegistration(ctx context.Context, imsi string, expected, next *IMSRegistration) (*IMSRegistration, bool, error) {
	ctx, span := startIMSRegistrationsSpan(ctx, "UPDATE")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSRegistrationsTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSRegistrationsTableName, "update").Inc()

	result, err := opCompareAndSwapIMSRegistration.Invoke(ctx, db, &imsRegistrationCASPayload{IMSI: imsi, Expected: expected, Next: next})
	if err != nil {
		recordSpanError(span, err)

		return nil, false, err
	}

	if result == nil {
		err = fmt.Errorf("compare and swap IMS registration for subscriber %s: leader returned no result", imsi)
		recordSpanError(span, err)

		return nil, false, err
	}

	return result.Current, result.Swapped, nil
}

func (db *Database) applyCompareAndSwapIMSRegistration(ctx context.Context, payload *imsRegistrationCASPayload) (any, error) {
	current, err := db.getIMSRegistration(ctx, db.runner(ctx), payload.IMSI)
	if err != nil {
		return nil, err
	}

	if !sameIMSRegistration(current, payload.Expected) {
		return &imsRegistrationCASResult{Current: current}, nil
	}

	if !sameIMSRegistration(current, payload.Next) {
		if err := db.writeIMSRegistration(ctx, payload.IMSI, payload.Next); err != nil {
			return nil, err
		}
	}

	next := payload.Next
	if next != nil {
		stored := *next
		stored.IMSI = payload.IMSI
		next = &stored
	}

	return &imsRegistrationCASResult{Swapped: true, Current: next}, nil
}

func (db *Database) applyRetiredCompareAndSwapIMSRegistration(context.Context, *struct{}) (any, error) {
	return nil, ErrRetiredOperation
}

func (db *Database) writeIMSRegistration(ctx context.Context, imsi string, next *IMSRegistration) error {
	if next != nil {
		if err := validateIMSRegistration(next); err != nil {
			return err
		}
	}

	if _, err := db.runner(ctx).ExecContext(ctx, deleteIMSRegistrationSetsSQL, imsi); err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	if next == nil {
		if _, err := db.runner(ctx).ExecContext(ctx, deleteIMSRegistrationSQL, imsi); err != nil {
			return fmt.Errorf("query failed: %w", err)
		}

		return nil
	}

	row := imsRegistrationRow{
		Imsi:            imsi,
		ServerName:      next.ServerName,
		OriginHost:      next.OriginHost,
		OriginRealm:     next.OriginRealm,
		PrivateIdentity: next.PrivateIdentity,
	}

	if _, err := db.runner(ctx).ExecContext(ctx, upsertIMSRegistrationSQL, row.Imsi, row.ServerName, row.OriginHost, row.OriginRealm, row.PrivateIdentity); err != nil {
		if isForeignKeyError(err) {
			return ErrNotFound
		}

		return fmt.Errorf("query failed: %w", err)
	}

	for _, set := range slices.Sorted(maps.Keys(next.Sets)) {
		s := next.Sets[set]

		setRow := imsRegistrationSetRow{
			Imsi:            imsi,
			RegistrationSet: string(set),
			State:           int(s.State),
			AuthPending:     s.AuthPending,
			DefaultIdentity: s.DefaultIdentity,
			ProfileDigest:   s.ProfileDigest,
			Emergency:       s.Emergency,
		}

		if _, err := db.runner(ctx).ExecContext(ctx, insertIMSRegistrationSetSQL, setRow.Imsi, setRow.RegistrationSet, setRow.State, setRow.AuthPending, setRow.DefaultIdentity, setRow.ProfileDigest, setRow.Emergency); err != nil {
			return fmt.Errorf("query failed: %w", err)
		}
	}

	return nil
}

func (db *Database) getIMSRegistration(ctx context.Context, q querier, imsi string) (*IMSRegistration, error) {
	row, err := scanIMSRegistrationRow(q.QueryRowContext(ctx, getIMSRegistrationSQL, imsi))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	sets, err := queryAll(ctx, q, listIMSRegistrationSetsSQL, scanIMSRegistrationSetRow, imsi)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return imsRegistrationOf(row, sets), nil
}

func startIMSRegistrationsSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	querySummary := fmt.Sprintf("%s %s", operation, IMSRegistrationsTableName)

	return tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(IMSRegistrationsTableName),
		),
	)
}

func scanIMSRegistrationRow(row scanner) (imsRegistrationRow, error) {
	var v imsRegistrationRow

	if err := row.Scan(&v.Imsi, &v.ServerName, &v.OriginHost, &v.OriginRealm, &v.PrivateIdentity); err != nil {
		return imsRegistrationRow{}, err
	}

	return v, nil
}

func scanIMSRegistrationSetRow(row scanner) (imsRegistrationSetRow, error) {
	var v imsRegistrationSetRow

	if err := row.Scan(&v.Imsi, &v.RegistrationSet, &v.State, &v.AuthPending, &v.DefaultIdentity, &v.ProfileDigest, &v.Emergency); err != nil {
		return imsRegistrationSetRow{}, err
	}

	return v, nil
}
