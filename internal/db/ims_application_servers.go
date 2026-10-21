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
	listIMSApplicationServersStmt  = "SELECT &imsApplicationServerRow.* FROM %s ORDER BY name"
	getIMSApplicationServerStmt    = "SELECT &imsApplicationServerRow.* FROM %s WHERE name==$imsApplicationServerRow.name"
	insertIMSApplicationServerStmt = "INSERT INTO %s (id, name, serverName) VALUES ($imsApplicationServerRow.id, $imsApplicationServerRow.name, $imsApplicationServerRow.serverName)"
	updateIMSApplicationServerStmt = "UPDATE %s SET serverName=$imsApplicationServerRow.serverName WHERE id==$imsApplicationServerRow.id"
	deleteIMSApplicationServerStmt = "DELETE FROM %s WHERE name==$imsApplicationServerRow.name"
	listIMSPSIsStmt                = "SELECT &imsPSIRow.* FROM %s ORDER BY identity"
	listIMSPSIsByASStmt            = "SELECT &imsPSIRow.* FROM %s WHERE applicationServerID==$imsPSIRow.applicationServerID ORDER BY identity"
	insertIMSPSIStmt               = "INSERT INTO %s (identity, applicationServerID) VALUES ($imsPSIRow.identity, $imsPSIRow.applicationServerID)"
	deleteIMSPSIsByASStmt          = "DELETE FROM %s WHERE applicationServerID==$imsPSIRow.applicationServerID"
)

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
	if as.Name == "" {
		return errors.New("application server name is required")
	}

	if !hasURIScheme(as.ServerName, "sip:", "sips:") {
		return fmt.Errorf("application server name must be a SIP URI, got %q", as.ServerName)
	}

	for i, psi := range as.PublicServiceIdentities {
		if !hasURIScheme(psi, "sip:", "sips:", "tel:") {
			return fmt.Errorf("public service identity must be a SIP or tel URI, got %q", psi)
		}

		if slices.Contains(as.PublicServiceIdentities[:i], psi) {
			return fmt.Errorf("public service identity %s is listed twice", psi)
		}
	}

	return nil
}

func hasURIScheme(uri string, schemes ...string) bool {
	for _, scheme := range schemes {
		if len(uri) > len(scheme) && strings.EqualFold(uri[:len(scheme)], scheme) {
			return true
		}
	}

	return false
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

func (db *Database) getIMSApplicationServer(ctx context.Context, runner *sqlair.DB, name string) (*IMSApplicationServer, error) {
	row := imsApplicationServerRow{Name: name}

	if err := runner.Query(ctx, db.getIMSApplicationServerStmt, row).Get(&row); err != nil {
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
	if err := as.Validate(); err != nil {
		return err
	}

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

func (db *Database) insertIMSPSIs(ctx context.Context, applicationServerID string, psis []string) error {
	for _, psi := range psis {
		if err := db.runner(ctx).Query(ctx, db.insertIMSPSIStmt, imsPSIRow{Identity: psi, ApplicationServerID: applicationServerID}).Run(); err != nil {
			if isPrimaryKeyError(err) {
				return fmt.Errorf("public service identity %s: %w", psi, ErrAlreadyExists)
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
