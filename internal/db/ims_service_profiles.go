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
	"strings"

	"github.com/canonical/sqlair"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	IMSServiceProfilesTableName           = "ims_service_profiles"
	IMSServiceProfileAssignmentsTableName = "ims_service_profile_assignments"
)

const (
	listIMSServiceProfilesStmt            = "SELECT &imsServiceProfileRow.* FROM %s ORDER BY name"
	getIMSServiceProfileStmt              = "SELECT &imsServiceProfileRow.* FROM %s WHERE name==$imsServiceProfileRow.name"
	insertIMSServiceProfileStmt           = "INSERT INTO %s (id, name) VALUES ($imsServiceProfileRow.id, $imsServiceProfileRow.name)"
	deleteIMSServiceProfileStmt           = "DELETE FROM %s WHERE name==$imsServiceProfileRow.name"
	listProfileIMSServiceProfilesStmt     = "SELECT &imsServiceProfileAssignmentRow.* FROM %s WHERE profileID==$imsServiceProfileAssignmentRow.profileID ORDER BY registrationSet"
	listAllIMSServiceProfileAssignsStmt   = "SELECT &imsServiceProfileAssignmentRow.* FROM %s ORDER BY profileID, registrationSet"
	insertIMSServiceProfileAssignStmt     = "INSERT INTO %s (profileID, registrationSet, serviceProfileID) VALUES ($imsServiceProfileAssignmentRow.profileID, $imsServiceProfileAssignmentRow.registrationSet, $imsServiceProfileAssignmentRow.serviceProfileID)"
	deleteProfileIMSServiceProfilesStmt   = "DELETE FROM %s WHERE profileID==$imsServiceProfileAssignmentRow.profileID"
	listProfilesOfIMSServiceProfileStmt   = "SELECT DISTINCT &imsServiceProfileAssignmentRow.profileID FROM %s WHERE serviceProfileID==$imsServiceProfileAssignmentRow.serviceProfileID"
	countIMSServiceProfileAssignmentsStmt = "SELECT COUNT(*) AS &NumItems.count FROM %s WHERE serviceProfileID==$imsServiceProfileAssignmentRow.serviceProfileID"
)

type IMSServiceProfile struct {
	ID                    string                      `json:"id"`
	Name                  string                      `json:"name"`
	InitialFilterCriteria []IMSInitialFilterCriterion `json:"initial_filter_criteria"`
}

type ProfileIMSServiceProfiles struct {
	ProfileID       string                        `json:"profile_id"`
	ServiceProfiles map[IMSRegistrationSet]string `json:"service_profiles"`
}

type imsServiceProfileRow struct {
	ID   string `db:"id"`
	Name string `db:"name"`
}

type imsServiceProfileAssignmentRow struct {
	ProfileID        string `db:"profileID"`
	RegistrationSet  string `db:"registrationSet"`
	ServiceProfileID string `db:"serviceProfileID"`
}

func (sp IMSServiceProfile) Validate() error {
	if strings.TrimSpace(sp.Name) == "" {
		return errors.New("service profile name is required")
	}

	return ValidateIMSFilterCriteria(sp.InitialFilterCriteria)
}

func (db *Database) ListIMSServiceProfiles(ctx context.Context) ([]IMSServiceProfile, error) {
	ctx, span := startIMSServiceProfilesSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSServiceProfilesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSServiceProfilesTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return nil, nil
	}

	var (
		rows    []imsServiceProfileRow
		ifcRows []imsIFCRow
	)

	if err := db.conn().Query(ctx, db.listIMSServiceProfilesStmt).GetAll(&rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if err := db.conn().Query(ctx, db.listAllIMSIFCsStmt).GetAll(&ifcRows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)
		return nil, fmt.Errorf("query failed: %w", err)
	}

	ifcs := make(map[string][]IMSInitialFilterCriterion, len(rows))

	for _, r := range ifcRows {
		ifc, err := imsIFCOf(r)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}

		ifcs[r.ServiceProfileID] = append(ifcs[r.ServiceProfileID], ifc)
	}

	profiles := make([]IMSServiceProfile, 0, len(rows))

	for _, r := range rows {
		criteria := ifcs[r.ID]
		if criteria == nil {
			criteria = []IMSInitialFilterCriterion{}
		}

		profiles = append(profiles, IMSServiceProfile{ID: r.ID, Name: r.Name, InitialFilterCriteria: criteria})
	}

	return profiles, nil
}

func (db *Database) GetIMSServiceProfile(ctx context.Context, name string) (*IMSServiceProfile, error) {
	ctx, span := startIMSServiceProfilesSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSServiceProfilesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSServiceProfilesTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return nil, ErrNotFound
	}

	sp, err := db.getIMSServiceProfile(ctx, db.conn(), name)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return sp, nil
}

func (db *Database) getIMSServiceProfile(ctx context.Context, runner *sqlair.DB, name string) (*IMSServiceProfile, error) {
	row := imsServiceProfileRow{Name: name}

	if err := runner.Query(ctx, db.getIMSServiceProfileStmt, row).Get(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	var ifcRows []imsIFCRow

	if err := runner.Query(ctx, db.listServiceProfileIMSIFCsStmt, imsIFCRow{ServiceProfileID: row.ID}).GetAll(&ifcRows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	sp := &IMSServiceProfile{ID: row.ID, Name: row.Name, InitialFilterCriteria: make([]IMSInitialFilterCriterion, 0, len(ifcRows))}

	for _, r := range ifcRows {
		ifc, err := imsIFCOf(r)
		if err != nil {
			return nil, err
		}

		sp.InitialFilterCriteria = append(sp.InitialFilterCriteria, ifc)
	}

	return sp, nil
}

func (db *Database) CreateIMSServiceProfile(ctx context.Context, sp *IMSServiceProfile) error {
	if sp.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate service profile id: %w", err)
		}

		sp.ID = id.String()
	}

	return db.writeIMSServiceProfile(ctx, "INSERT", opCreateIMSServiceProfile, sp)
}

func (db *Database) UpdateIMSServiceProfile(ctx context.Context, sp *IMSServiceProfile) error {
	return db.writeIMSServiceProfile(ctx, "UPDATE", opUpdateIMSServiceProfile, sp)
}

func (db *Database) writeIMSServiceProfile(ctx context.Context, operation string, op *ChangesetOp[IMSServiceProfile, struct{}], sp *IMSServiceProfile) error {
	if err := sp.Validate(); err != nil {
		return err
	}

	ctx, span := startIMSServiceProfilesSpan(ctx, operation)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSServiceProfilesTableName, strings.ToLower(operation)))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSServiceProfilesTableName, strings.ToLower(operation)).Inc()

	if _, err := op.Invoke(ctx, db, sp); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) applyCreateIMSServiceProfile(ctx context.Context, sp *IMSServiceProfile) (any, error) {
	if err := db.runner(ctx).Query(ctx, db.insertIMSServiceProfileStmt, imsServiceProfileRow{ID: sp.ID, Name: sp.Name}).Run(); err != nil {
		if isUniqueNameError(err) || isPrimaryKeyError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, db.insertIMSIFCs(ctx, sp.ID, sp.InitialFilterCriteria)
}

func (db *Database) applyUpdateIMSServiceProfile(ctx context.Context, sp *IMSServiceProfile) (any, error) {
	existing, err := db.getIMSServiceProfile(ctx, db.runner(ctx), sp.Name)
	if err != nil {
		return nil, err
	}

	if err := db.runner(ctx).Query(ctx, db.deleteServiceProfileIMSIFCsStmt, imsIFCRow{ServiceProfileID: existing.ID}).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, db.insertIMSIFCs(ctx, existing.ID, sp.InitialFilterCriteria)
}

func (db *Database) insertIMSIFCs(ctx context.Context, serviceProfileID string, ifcs []IMSInitialFilterCriterion) error {
	for i, ifc := range ifcs {
		row, err := imsIFCRowOf(serviceProfileID, i, ifc)
		if err != nil {
			return err
		}

		if err := db.runner(ctx).Query(ctx, db.insertIMSIFCStmt, row).Run(); err != nil {
			if isForeignKeyError(err) {
				return fmt.Errorf("application server %s: %w", ifc.ApplicationServerID, ErrNotFound)
			}

			return fmt.Errorf("query failed: %w", err)
		}
	}

	return nil
}

func (db *Database) DeleteIMSServiceProfile(ctx context.Context, name string) error {
	ctx, span := startIMSServiceProfilesSpan(ctx, "DELETE")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSServiceProfilesTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSServiceProfilesTableName, "delete").Inc()

	if _, err := opDeleteIMSServiceProfile.Invoke(ctx, db, &stringPayload{Value: name}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) applyDeleteIMSServiceProfile(ctx context.Context, p *stringPayload) (any, error) {
	existing, err := db.getIMSServiceProfile(ctx, db.runner(ctx), p.Value)
	if err != nil {
		return nil, err
	}

	var assigned NumItems

	if err := db.runner(ctx).Query(ctx, db.countIMSServiceProfileAssignmentsStmt, imsServiceProfileAssignmentRow{ServiceProfileID: existing.ID}).Get(&assigned); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if assigned.Count > 0 {
		return nil, ErrInUse
	}

	if err := db.runner(ctx).Query(ctx, db.deleteServiceProfileIMSIFCsStmt, imsIFCRow{ServiceProfileID: existing.ID}).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if err := db.runner(ctx).Query(ctx, db.deleteIMSServiceProfileStmt, imsServiceProfileRow{Name: p.Value}).Run(); err != nil {
		if isForeignKeyError(err) {
			return nil, ErrInUse
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) GetProfileIMSServiceProfiles(ctx context.Context, profileID string) (map[IMSRegistrationSet]string, error) {
	ctx, span := startIMSServiceProfilesSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSServiceProfileAssignmentsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSServiceProfileAssignmentsTableName, "select").Inc()

	out := make(map[IMSRegistrationSet]string)

	if !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return out, nil
	}

	var rows []imsServiceProfileAssignmentRow

	if err := db.conn().Query(ctx, db.listProfileIMSServiceProfilesStmt, imsServiceProfileAssignmentRow{ProfileID: profileID}).GetAll(&rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)
		return nil, fmt.Errorf("query failed: %w", err)
	}

	for _, r := range rows {
		out[IMSRegistrationSet(r.RegistrationSet)] = r.ServiceProfileID
	}

	return out, nil
}

func (db *Database) ListIMSServiceProfileAssignments(ctx context.Context) ([]ProfileIMSServiceProfiles, error) {
	ctx, span := startIMSServiceProfilesSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSServiceProfileAssignmentsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSServiceProfileAssignmentsTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return nil, nil
	}

	var rows []imsServiceProfileAssignmentRow

	if err := db.conn().Query(ctx, db.listAllIMSServiceProfileAssignsStmt).GetAll(&rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)
		return nil, fmt.Errorf("query failed: %w", err)
	}

	var out []ProfileIMSServiceProfiles

	for _, r := range rows {
		if len(out) == 0 || out[len(out)-1].ProfileID != r.ProfileID {
			out = append(out, ProfileIMSServiceProfiles{ProfileID: r.ProfileID, ServiceProfiles: make(map[IMSRegistrationSet]string)})
		}

		out[len(out)-1].ServiceProfiles[IMSRegistrationSet(r.RegistrationSet)] = r.ServiceProfileID
	}

	return out, nil
}

func (db *Database) SetProfileIMSServiceProfiles(ctx context.Context, profileID string, serviceProfiles map[IMSRegistrationSet]string) error {
	for set, id := range serviceProfiles {
		if err := set.Validate(); err != nil {
			return err
		}

		if id == "" {
			return fmt.Errorf("registration set %s: service profile is required", set)
		}
	}

	ctx, span := startIMSServiceProfilesSpan(ctx, "UPDATE")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSServiceProfileAssignmentsTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSServiceProfileAssignmentsTableName, "update").Inc()

	if _, err := opSetProfileIMSServiceProfiles.Invoke(ctx, db, &ProfileIMSServiceProfiles{ProfileID: profileID, ServiceProfiles: serviceProfiles}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) applySetProfileIMSServiceProfiles(ctx context.Context, p *ProfileIMSServiceProfiles) (any, error) {
	if err := db.runner(ctx).Query(ctx, db.deleteProfileIMSServiceProfilesStmt, imsServiceProfileAssignmentRow{ProfileID: p.ProfileID}).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	for _, set := range slices.Sorted(maps.Keys(p.ServiceProfiles)) {
		row := imsServiceProfileAssignmentRow{ProfileID: p.ProfileID, RegistrationSet: string(set), ServiceProfileID: p.ServiceProfiles[set]}

		if err := db.runner(ctx).Query(ctx, db.insertIMSServiceProfileAssignStmt, row).Run(); err != nil {
			if isForeignKeyError(err) {
				return nil, fmt.Errorf("profile %s or service profile %s: %w", p.ProfileID, row.ServiceProfileID, ErrNotFound)
			}

			return nil, fmt.Errorf("query failed: %w", err)
		}
	}

	return nil, nil
}

func (db *Database) ProfilesUsingIMSServiceProfile(ctx context.Context, serviceProfileID string) ([]string, error) {
	ctx, span := startIMSServiceProfilesSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSServiceProfileAssignmentsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSServiceProfileAssignmentsTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return nil, nil
	}

	var rows []imsServiceProfileAssignmentRow

	if err := db.conn().Query(ctx, db.listProfilesOfIMSServiceProfileStmt, imsServiceProfileAssignmentRow{ServiceProfileID: serviceProfileID}).GetAll(&rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)
		return nil, fmt.Errorf("query failed: %w", err)
	}

	profileIDs := make([]string, 0, len(rows))
	for _, r := range rows {
		profileIDs = append(profileIDs, r.ProfileID)
	}

	return profileIDs, nil
}

func startIMSServiceProfilesSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	querySummary := fmt.Sprintf("%s %s", operation, IMSServiceProfilesTableName)

	return tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(IMSServiceProfilesTableName),
		),
	)
}
