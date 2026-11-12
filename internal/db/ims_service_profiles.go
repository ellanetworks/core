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

	"github.com/google/uuid"
)

const (
	IMSServiceProfilesTableName           = "ims_service_profiles"
	IMSServiceProfileAssignmentsTableName = "ims_service_profile_assignments"
)

const (
	listIMSServiceProfilesSQL            = "SELECT id, name FROM " + IMSServiceProfilesTableName + " ORDER BY name"
	getIMSServiceProfileSQL              = "SELECT id, name FROM " + IMSServiceProfilesTableName + " WHERE name==?"
	insertIMSServiceProfileSQL           = "INSERT INTO " + IMSServiceProfilesTableName + " (id, name) VALUES (?, ?)"
	deleteIMSServiceProfileSQL           = "DELETE FROM " + IMSServiceProfilesTableName + " WHERE name==?"
	listProfileIMSServiceProfilesSQL     = "SELECT profileID, registrationSet, serviceProfileID FROM " + IMSServiceProfileAssignmentsTableName + " WHERE profileID==? ORDER BY registrationSet"
	listAllIMSServiceProfileAssignsSQL   = "SELECT profileID, registrationSet, serviceProfileID FROM " + IMSServiceProfileAssignmentsTableName + " ORDER BY profileID, registrationSet"
	insertIMSServiceProfileAssignSQL     = "INSERT INTO " + IMSServiceProfileAssignmentsTableName + " (profileID, registrationSet, serviceProfileID) VALUES (?, ?, ?)"
	deleteProfileIMSServiceProfilesSQL   = "DELETE FROM " + IMSServiceProfileAssignmentsTableName + " WHERE profileID==?"
	listProfilesOfIMSServiceProfileSQL   = "SELECT DISTINCT profileID FROM " + IMSServiceProfileAssignmentsTableName + " WHERE serviceProfileID==?"
	countIMSServiceProfileAssignmentsSQL = "SELECT COUNT(*) FROM " + IMSServiceProfileAssignmentsTableName + " WHERE serviceProfileID==?"
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
	ID   string
	Name string
}

type imsServiceProfileAssignmentRow struct {
	ProfileID        string
	RegistrationSet  string
	ServiceProfileID string
}

func (sp IMSServiceProfile) Validate() error {
	if strings.TrimSpace(sp.Name) == "" {
		return errors.New("service profile name is required")
	}

	return ValidateIMSFilterCriteria(sp.InitialFilterCriteria)
}

func (db *Database) ListIMSServiceProfiles(ctx context.Context) (_ []IMSServiceProfile, err error) {
	ctx, finish := startDBQuery(ctx, IMSServiceProfilesTableName, "SELECT")
	defer func() { finish(err) }()

	if !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return nil, nil
	}

	rows, err := queryAll(ctx, db.reader(), listIMSServiceProfilesSQL, scanIMSServiceProfileRow)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	ifcRows, err := queryAll(ctx, db.reader(), listAllIMSIFCsSQL, scanIMSIFCRow)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	ifcs := make(map[string][]IMSInitialFilterCriterion, len(rows))

	for _, r := range ifcRows {
		ifc, err := imsIFCOf(r)
		if err != nil {
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

func (db *Database) GetIMSServiceProfile(ctx context.Context, name string) (_ *IMSServiceProfile, err error) {
	ctx, finish := startDBQuery(ctx, IMSServiceProfilesTableName, "SELECT")
	defer func() { finish(err) }()

	if !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return nil, ErrNotFound
	}

	sp, err := db.getIMSServiceProfile(ctx, db.reader(), name)
	if err != nil {
		return nil, err
	}

	return sp, nil
}

func (db *Database) getIMSServiceProfile(ctx context.Context, q querier, name string) (*IMSServiceProfile, error) {
	row, err := scanIMSServiceProfileRow(q.QueryRowContext(ctx, getIMSServiceProfileSQL, name))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	ifcRows, err := queryAll(ctx, q, listServiceProfileIMSIFCsSQL, scanIMSIFCRow, row.ID)
	if err != nil {
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

func (db *Database) writeIMSServiceProfile(ctx context.Context, operation string, op *ChangesetOp[IMSServiceProfile, struct{}], sp *IMSServiceProfile) (err error) {
	if err := sp.Validate(); err != nil {
		return err
	}

	ctx, finish := startDBQuery(ctx, IMSServiceProfilesTableName, operation)
	defer func() { finish(err) }()

	if _, err := op.Invoke(ctx, db, sp); err != nil {
		return err
	}

	return nil
}

func (db *Database) applyCreateIMSServiceProfile(ctx context.Context, sp *IMSServiceProfile) (any, error) {
	if _, err := db.runner(ctx).ExecContext(ctx, insertIMSServiceProfileSQL, sp.ID, sp.Name); err != nil {
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

	if _, err := db.runner(ctx).ExecContext(ctx, deleteServiceProfileIMSIFCsSQL, existing.ID); err != nil {
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

		if _, err := db.runner(ctx).ExecContext(ctx, insertIMSIFCSQL, row.ServiceProfileID, row.Position, row.ApplicationServerID, row.TriggerPoint, row.ProfilePart, row.DefaultHandling, row.ServiceInfo, row.IncludeRegisterRequest, row.IncludeRegisterResponse); err != nil {
			if isForeignKeyError(err) {
				return fmt.Errorf("application server %s: %w", ifc.ApplicationServerID, ErrNotFound)
			}

			return fmt.Errorf("query failed: %w", err)
		}
	}

	return nil
}

func (db *Database) DeleteIMSServiceProfile(ctx context.Context, name string) (err error) {
	ctx, finish := startDBQuery(ctx, IMSServiceProfilesTableName, "DELETE")
	defer func() { finish(err) }()

	if _, err := opDeleteIMSServiceProfile.Invoke(ctx, db, &stringPayload{Value: name}); err != nil {
		return err
	}

	return nil
}

func (db *Database) applyDeleteIMSServiceProfile(ctx context.Context, p *stringPayload) (any, error) {
	existing, err := db.getIMSServiceProfile(ctx, db.runner(ctx), p.Value)
	if err != nil {
		return nil, err
	}

	assigned, err := queryCount(ctx, db.runner(ctx), countIMSServiceProfileAssignmentsSQL, existing.ID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if assigned > 0 {
		return nil, ErrInUse
	}

	if _, err := db.runner(ctx).ExecContext(ctx, deleteServiceProfileIMSIFCsSQL, existing.ID); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if _, err := db.runner(ctx).ExecContext(ctx, deleteIMSServiceProfileSQL, p.Value); err != nil {
		if isForeignKeyError(err) {
			return nil, ErrInUse
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) GetProfileIMSServiceProfiles(ctx context.Context, profileID string) (_ map[IMSRegistrationSet]string, err error) {
	ctx, finish := startDBQuery(ctx, IMSServiceProfilesTableName, "SELECT")
	defer func() { finish(err) }()

	out := make(map[IMSRegistrationSet]string)

	if !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return out, nil
	}

	rows, err := queryAll(ctx, db.reader(), listProfileIMSServiceProfilesSQL, scanIMSServiceProfileAssignmentRow, profileID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	for _, r := range rows {
		out[IMSRegistrationSet(r.RegistrationSet)] = r.ServiceProfileID
	}

	return out, nil
}

func (db *Database) ListIMSServiceProfileAssignments(ctx context.Context) (_ []ProfileIMSServiceProfiles, err error) {
	ctx, finish := startDBQuery(ctx, IMSServiceProfilesTableName, "SELECT")
	defer func() { finish(err) }()

	if !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return nil, nil
	}

	rows, err := queryAll(ctx, db.reader(), listAllIMSServiceProfileAssignsSQL, scanIMSServiceProfileAssignmentRow)
	if err != nil {
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

func (db *Database) SetProfileIMSServiceProfiles(ctx context.Context, profileID string, serviceProfiles map[IMSRegistrationSet]string) (err error) {
	for set, id := range serviceProfiles {
		if err := set.Validate(); err != nil {
			return err
		}

		if id == "" {
			return fmt.Errorf("registration set %s: service profile is required", set)
		}
	}

	ctx, finish := startDBQuery(ctx, IMSServiceProfilesTableName, "UPDATE")
	defer func() { finish(err) }()

	if _, err := opSetProfileIMSServiceProfiles.Invoke(ctx, db, &ProfileIMSServiceProfiles{ProfileID: profileID, ServiceProfiles: serviceProfiles}); err != nil {
		return err
	}

	return nil
}

func (db *Database) applySetProfileIMSServiceProfiles(ctx context.Context, p *ProfileIMSServiceProfiles) (any, error) {
	if _, err := db.runner(ctx).ExecContext(ctx, deleteProfileIMSServiceProfilesSQL, p.ProfileID); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	for _, set := range slices.Sorted(maps.Keys(p.ServiceProfiles)) {
		row := imsServiceProfileAssignmentRow{ProfileID: p.ProfileID, RegistrationSet: string(set), ServiceProfileID: p.ServiceProfiles[set]}

		if _, err := db.runner(ctx).ExecContext(ctx, insertIMSServiceProfileAssignSQL, row.ProfileID, row.RegistrationSet, row.ServiceProfileID); err != nil {
			if isForeignKeyError(err) {
				return nil, fmt.Errorf("profile %s or service profile %s: %w", p.ProfileID, row.ServiceProfileID, ErrNotFound)
			}

			return nil, fmt.Errorf("query failed: %w", err)
		}
	}

	return nil, nil
}

func (db *Database) ProfilesUsingIMSServiceProfile(ctx context.Context, serviceProfileID string) (_ []string, err error) {
	ctx, finish := startDBQuery(ctx, IMSServiceProfilesTableName, "SELECT")
	defer func() { finish(err) }()

	if !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return nil, nil
	}

	profileIDs, err := queryAll(ctx, db.reader(), listProfilesOfIMSServiceProfileSQL, scanValue[string], serviceProfileID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if profileIDs == nil {
		return []string{}, nil
	}

	return profileIDs, nil
}

func scanIMSServiceProfileRow(row scanner) (imsServiceProfileRow, error) {
	var v imsServiceProfileRow

	if err := row.Scan(&v.ID, &v.Name); err != nil {
		return imsServiceProfileRow{}, err
	}

	return v, nil
}

func scanIMSServiceProfileAssignmentRow(row scanner) (imsServiceProfileAssignmentRow, error) {
	var v imsServiceProfileAssignmentRow

	if err := row.Scan(&v.ProfileID, &v.RegistrationSet, &v.ServiceProfileID); err != nil {
		return imsServiceProfileAssignmentRow{}, err
	}

	return v, nil
}
