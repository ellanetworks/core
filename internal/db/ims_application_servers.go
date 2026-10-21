// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/internal/diameternode"
	"github.com/ellanetworks/core/internal/hss/psi"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	IMSApplicationServersTableName      = "ims_application_servers"
	IMSPublicServiceIdentitiesTableName = "ims_public_service_identities"
)

const imsServicesSchema = 25

const (
	listIMSApplicationServersStmt   = "SELECT &imsApplicationServerRow.* FROM %s ORDER BY name"
	getIMSApplicationServerStmt     = "SELECT &imsApplicationServerRow.* FROM %s WHERE name==$imsApplicationServerRow.name"
	getIMSApplicationServerByIDStmt = "SELECT &imsApplicationServerRow.* FROM %s WHERE id==$imsApplicationServerRow.id"
	insertIMSApplicationServerStmt  = "INSERT INTO %s (id, name, serverName) VALUES ($imsApplicationServerRow.id, $imsApplicationServerRow.name, $imsApplicationServerRow.serverName)"
	updateIMSApplicationServerStmt  = "UPDATE %s SET serverName=$imsApplicationServerRow.serverName WHERE id==$imsApplicationServerRow.id"
	deleteIMSApplicationServerStmt  = "DELETE FROM %s WHERE name==$imsApplicationServerRow.name"
	listIMSPSIsStmt                 = "SELECT &imsPSIRow.* FROM %s ORDER BY identity"
	listIMSPSIsByASStmt             = "SELECT &imsPSIRow.* FROM %s WHERE applicationServerID==$imsPSIRow.applicationServerID ORDER BY identity"
	getIMSPSIStmt                   = "SELECT &imsPSIRow.* FROM %s WHERE identity==$imsPSIRow.identity"
	insertIMSPSIStmt                = "INSERT INTO %s (identity, applicationServerID) VALUES ($imsPSIRow.identity, $imsPSIRow.applicationServerID)"
	deleteIMSPSIsByASStmt           = "DELETE FROM %s WHERE applicationServerID==$imsPSIRow.applicationServerID"
)

var ErrPSIConflict = errors.New("public service identity conflict")

type IMSApplicationServer struct {
	ID                      string   `json:"id"`
	Name                    string   `json:"name"`
	ServerName              string   `json:"server_name"`
	PublicServiceIdentities []string `json:"public_service_identities"`
}

type imsApplicationServerRow struct {
	ID         string `db:"id"`
	Name       string `db:"name"`
	ServerName string `db:"serverName"`
}

type imsPSIRow struct {
	Identity            string `db:"identity"`
	ApplicationServerID string `db:"applicationServerID"`
}

func (as IMSApplicationServer) Validate() error {
	_, err := as.canonical()

	return err
}

func (as IMSApplicationServer) canonical() (IMSApplicationServer, error) {
	if as.Name == "" {
		return as, errors.New("application server name is required")
	}

	serverName, err := psi.CanonicalServerName(as.ServerName)
	if err != nil {
		return as, fmt.Errorf("application server name: %w", err)
	}

	if err := cx.ValidateURI("application server name", serverName); err != nil {
		return as, err
	}

	identities := make([]string, 0, len(as.PublicServiceIdentities))

	for _, identity := range as.PublicServiceIdentities {
		canonical, err := psi.Canonical(identity)
		if err != nil {
			return as, fmt.Errorf("public service identity: %w", err)
		}

		if err := cx.ValidateURI("public service identity", canonical); err != nil {
			return as, err
		}

		if slices.Contains(identities, canonical) {
			return as, fmt.Errorf("public service identity %s is listed twice", canonical)
		}

		identities = append(identities, canonical)
	}

	as.ServerName, as.PublicServiceIdentities = serverName, identities

	return as, nil
}

func (db *Database) ListIMSApplicationServers(ctx context.Context) ([]IMSApplicationServer, error) {
	ctx, span := startIMSApplicationServersSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSApplicationServersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSApplicationServersTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return nil, nil
	}

	var rows []imsApplicationServerRow

	if err := db.conn().Query(ctx, db.listIMSApplicationServersStmt).GetAll(&rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)
		return nil, fmt.Errorf("query failed: %w", err)
	}

	var psis []imsPSIRow

	if err := db.conn().Query(ctx, db.listIMSPSIsStmt).GetAll(&psis); err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)
		return nil, fmt.Errorf("query failed: %w", err)
	}

	servers := make([]IMSApplicationServer, 0, len(rows))

	for _, r := range rows {
		as := IMSApplicationServer{ID: r.ID, Name: r.Name, ServerName: r.ServerName, PublicServiceIdentities: []string{}}

		for _, psi := range psis {
			if psi.ApplicationServerID == r.ID {
				as.PublicServiceIdentities = append(as.PublicServiceIdentities, psi.Identity)
			}
		}

		servers = append(servers, as)
	}

	return servers, nil
}

func (db *Database) GetIMSApplicationServer(ctx context.Context, name string) (*IMSApplicationServer, error) {
	ctx, span := startIMSApplicationServersSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSApplicationServersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSApplicationServersTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return nil, ErrNotFound
	}

	as, err := db.getIMSApplicationServer(ctx, db.conn(), name)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return as, nil
}

func (db *Database) GetIMSApplicationServerByID(ctx context.Context, id string) (*IMSApplicationServer, error) {
	ctx, span := startIMSApplicationServersSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSApplicationServersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSApplicationServersTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return nil, ErrNotFound
	}

	as, err := db.loadIMSApplicationServer(ctx, db.conn(), db.getIMSApplicationServerByIDStmt, imsApplicationServerRow{ID: id})
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return as, nil
}

func (db *Database) getIMSApplicationServer(ctx context.Context, runner *sqlair.DB, name string) (*IMSApplicationServer, error) {
	return db.loadIMSApplicationServer(ctx, runner, db.getIMSApplicationServerStmt, imsApplicationServerRow{Name: name})
}

func (db *Database) loadIMSApplicationServer(ctx context.Context, runner *sqlair.DB, stmt *sqlair.Statement, row imsApplicationServerRow) (*IMSApplicationServer, error) {
	if err := runner.Query(ctx, stmt, row).Get(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	var psis []imsPSIRow

	if err := runner.Query(ctx, db.listIMSPSIsByASStmt, imsPSIRow{ApplicationServerID: row.ID}).GetAll(&psis); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	as := &IMSApplicationServer{ID: row.ID, Name: row.Name, ServerName: row.ServerName, PublicServiceIdentities: make([]string, 0, len(psis))}

	for _, psi := range psis {
		as.PublicServiceIdentities = append(as.PublicServiceIdentities, psi.Identity)
	}

	return as, nil
}

func (db *Database) CreateIMSApplicationServer(ctx context.Context, as *IMSApplicationServer) error {
	if as.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate application server id: %w", err)
		}

		as.ID = id.String()
	}

	return db.writeIMSApplicationServer(ctx, "INSERT", opCreateIMSApplicationServer, as)
}

func (db *Database) UpdateIMSApplicationServer(ctx context.Context, as *IMSApplicationServer) error {
	return db.writeIMSApplicationServer(ctx, "UPDATE", opUpdateIMSApplicationServer, as)
}

func (db *Database) writeIMSApplicationServer(ctx context.Context, operation string, op *ChangesetOp[IMSApplicationServer, struct{}], as *IMSApplicationServer) error {
	canonical, err := as.canonical()
	if err != nil {
		return err
	}

	*as = canonical

	ctx, span := startIMSApplicationServersSpan(ctx, operation)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSApplicationServersTableName, strings.ToLower(operation)))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSApplicationServersTableName, strings.ToLower(operation)).Inc()

	if _, err := op.Invoke(ctx, db, as); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) DeleteIMSApplicationServer(ctx context.Context, name string) error {
	ctx, span := startIMSApplicationServersSpan(ctx, "DELETE")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSApplicationServersTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSApplicationServersTableName, "delete").Inc()

	if _, err := opDeleteIMSApplicationServer.Invoke(ctx, db, &stringPayload{Value: name}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) applyCreateIMSApplicationServer(ctx context.Context, as *IMSApplicationServer) (any, error) {
	if err := db.checkPSIsAvailable(ctx, as.ID, as.PublicServiceIdentities); err != nil {
		return nil, err
	}

	row := imsApplicationServerRow{ID: as.ID, Name: as.Name, ServerName: as.ServerName}

	if err := db.runner(ctx).Query(ctx, db.insertIMSApplicationServerStmt, row).Run(); err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, db.insertIMSPSIs(ctx, as.ID, as.PublicServiceIdentities)
}

func (db *Database) applyUpdateIMSApplicationServer(ctx context.Context, as *IMSApplicationServer) (any, error) {
	existing, err := db.getIMSApplicationServer(ctx, db.runner(ctx), as.Name)
	if err != nil {
		return nil, err
	}

	if err := db.checkPSIsAvailable(ctx, existing.ID, as.PublicServiceIdentities); err != nil {
		return nil, err
	}

	row := imsApplicationServerRow{ID: existing.ID, ServerName: as.ServerName}

	if err := db.runner(ctx).Query(ctx, db.updateIMSApplicationServerStmt, row).Run(); err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	if err := db.runner(ctx).Query(ctx, db.deleteIMSPSIsByASStmt, imsPSIRow{ApplicationServerID: existing.ID}).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, db.insertIMSPSIs(ctx, existing.ID, as.PublicServiceIdentities)
}

func (db *Database) checkPSIsAvailable(ctx context.Context, applicationServerID string, identities []string) error {
	var stored []imsPSIRow

	if err := db.runner(ctx).Query(ctx, db.listIMSPSIsStmt).GetAll(&stored); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("query failed: %w", err)
	}

	imsDomain, err := db.imsDomain(ctx)
	if err != nil {
		return err
	}

	for _, identity := range identities {
		if collidesWithSubscriberIdentity(identity, imsDomain) {
			return fmt.Errorf("%w: %s has the form of a subscriber identity in %s", ErrPSIConflict, identity, imsDomain)
		}

		for _, other := range stored {
			if other.ApplicationServerID == applicationServerID {
				continue
			}

			if other.Identity == identity {
				return fmt.Errorf("%w: %s is hosted by another application server", ErrPSIConflict, identity)
			}

			if psi.Overlap(identity, other.Identity) {
				return fmt.Errorf("%w: %s overlaps %s on another application server", ErrPSIConflict, identity, other.Identity)
			}
		}

		msisdn, ok := strings.CutPrefix(identity, "tel:+")
		if !ok || psi.Wildcarded(identity) || strings.ContainsRune(msisdn, ';') {
			continue
		}

		holder := Subscriber{Msisdn: msisdn}

		err := db.runner(ctx).Query(ctx, db.getSubscriberByMSISDNStmt, holder).Get(&holder)
		if err == nil {
			return fmt.Errorf("%w: %s is the public user identity of subscriber %s", ErrPSIConflict, identity, holder.Imsi)
		}

		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("query failed: %w", err)
		}
	}

	return nil
}

func (db *Database) imsDomain(ctx context.Context) (string, error) {
	var op Operator

	if err := db.runner(ctx).Query(ctx, db.getOperatorStmt).Get(&op); err != nil {
		return "", fmt.Errorf("get operator: %w", err)
	}

	domain, err := diameternode.IMSRealm(op.Mcc, op.Mnc)
	if err != nil {
		return "", nil
	}

	return domain, nil
}

func collidesWithSubscriberIdentity(identity, imsDomain string) bool {
	if imsDomain == "" || psi.Host(identity) != imsDomain {
		return false
	}

	if !psi.Wildcarded(identity) {
		user := psi.User(identity)

		return len(user) >= 6 && len(user) <= 15 && strings.Trim(user, "0123456789") == ""
	}

	_, ok := psi.NewMatcher(map[string]string{identity: ""}).Match("sip:" + sampleIMSI + "@" + imsDomain)

	return ok
}

const sampleIMSI = "001010123456789"

func (db *Database) msisdnIsPSI(ctx context.Context, msisdn string) (bool, error) {
	if msisdn == "" || !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return false, nil
	}

	row := imsPSIRow{Identity: "tel:+" + msisdn}

	err := db.runner(ctx).Query(ctx, db.getIMSPSIStmt, row).Get(&row)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("query failed: %w", err)
	}

	return true, nil
}

func (db *Database) insertIMSPSIs(ctx context.Context, applicationServerID string, psis []string) error {
	for _, identity := range psis {
		if err := db.runner(ctx).Query(ctx, db.insertIMSPSIStmt, imsPSIRow{Identity: identity, ApplicationServerID: applicationServerID}).Run(); err != nil {
			if isPrimaryKeyError(err) {
				return fmt.Errorf("%w: %s is hosted by another application server", ErrPSIConflict, identity)
			}

			return fmt.Errorf("query failed: %w", err)
		}
	}

	return nil
}

func (db *Database) applyDeleteIMSApplicationServer(ctx context.Context, p *stringPayload) (any, error) {
	var outcome sqlair.Outcome

	if err := db.runner(ctx).Query(ctx, db.deleteIMSApplicationServerStmt, imsApplicationServerRow{Name: p.Value}).Get(&outcome); err != nil {
		if isForeignKeyError(err) {
			return nil, ErrInUse
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func startIMSApplicationServersSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	querySummary := fmt.Sprintf("%s %s", operation, IMSApplicationServersTableName)

	return tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(IMSApplicationServersTableName),
		),
	)
}
