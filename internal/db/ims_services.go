// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"encoding/json"
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
	IMSServicesTableName              = "ims_services"
	IMSInitialFilterCriteriaTableName = "ims_initial_filter_criteria"
	ProfileIMSServicesTableName       = "profile_ims_services"
)

const (
	listIMSServicesStmt          = "SELECT &imsServiceRow.* FROM %s ORDER BY name"
	getIMSServiceStmt            = "SELECT &imsServiceRow.* FROM %s WHERE name==$imsServiceRow.name"
	insertIMSServiceStmt         = "INSERT INTO %s (id, name, kind, applicationServerID) VALUES ($imsServiceRow.id, $imsServiceRow.name, $imsServiceRow.kind, $imsServiceRow.applicationServerID)"
	updateIMSServiceStmt         = "UPDATE %s SET kind=$imsServiceRow.kind, applicationServerID=$imsServiceRow.applicationServerID WHERE id==$imsServiceRow.id"
	deleteIMSServiceStmt         = "DELETE FROM %s WHERE name==$imsServiceRow.name"
	listIMSIFCsStmt              = "SELECT &imsIFCRow.* FROM %s ORDER BY serviceID, position"
	listIMSIFCsByServiceStmt     = "SELECT &imsIFCRow.* FROM %s WHERE serviceID==$imsIFCRow.serviceID ORDER BY position"
	insertIMSIFCStmt             = "INSERT INTO %s (serviceID, position, triggerPoint, defaultHandling, includeRegisterRequest, includeRegisterResponse) VALUES ($imsIFCRow.serviceID, $imsIFCRow.position, $imsIFCRow.triggerPoint, $imsIFCRow.defaultHandling, $imsIFCRow.includeRegisterRequest, $imsIFCRow.includeRegisterResponse)"
	deleteIMSIFCsByServiceStmt   = "DELETE FROM %s WHERE serviceID==$imsIFCRow.serviceID"
	listProfileIMSServicesStmt   = "SELECT s.* AS &imsServiceRow.* FROM %s AS s JOIN %s AS p ON p.serviceID = s.id WHERE p.profileID==$profileIMSServiceRow.profileID ORDER BY p.position"
	insertProfileIMSServiceStmt  = "INSERT INTO %s (profileID, serviceID, position) VALUES ($profileIMSServiceRow.profileID, $profileIMSServiceRow.serviceID, $profileIMSServiceRow.position)"
	deleteProfileIMSServicesStmt = "DELETE FROM %s WHERE profileID==$profileIMSServiceRow.profileID"
)

type IMSServiceKind string

const (
	IMSServiceKindMCPTT  IMSServiceKind = "mcptt"
	IMSServiceKindCustom IMSServiceKind = "custom"
)

type IMSDefaultHandling string

const (
	IMSDefaultHandlingSessionContinued  IMSDefaultHandling = "SESSION_CONTINUED"
	IMSDefaultHandlingSessionTerminated IMSDefaultHandling = "SESSION_TERMINATED"
)

type IMSSessionCase string

const (
	IMSSessionCaseOriginatingRegistered   IMSSessionCase = "ORIGINATING_REGISTERED"
	IMSSessionCaseTerminatingRegistered   IMSSessionCase = "TERMINATING_REGISTERED"
	IMSSessionCaseTerminatingUnregistered IMSSessionCase = "TERMINATING_UNREGISTERED"
	IMSSessionCaseOriginatingUnregistered IMSSessionCase = "ORIGINATING_UNREGISTERED"
	IMSSessionCaseOriginatingCDIV         IMSSessionCase = "ORIGINATING_CDIV"
)

type IMSRegistrationType string

const (
	IMSRegistrationTypeInitial        IMSRegistrationType = "INITIAL_REGISTRATION"
	IMSRegistrationTypeReRegistration IMSRegistrationType = "RE-REGISTRATION"
	IMSRegistrationTypeDeregistration IMSRegistrationType = "DE-REGISTRATION"
)

const maxRegistrationTypesPerSPT = 2

type IMSService struct {
	ID                    string                      `json:"id"`
	Name                  string                      `json:"name"`
	Kind                  IMSServiceKind              `json:"kind"`
	ApplicationServerID   string                      `json:"application_server_id"`
	InitialFilterCriteria []IMSInitialFilterCriterion `json:"initial_filter_criteria,omitempty"`
}

type IMSInitialFilterCriterion struct {
	TriggerPoint            IMSTriggerPoint    `json:"trigger_point"`
	DefaultHandling         IMSDefaultHandling `json:"default_handling"`
	IncludeRegisterRequest  bool               `json:"include_register_request,omitempty"`
	IncludeRegisterResponse bool               `json:"include_register_response,omitempty"`
}

type IMSTriggerPoint struct {
	ConditionTypeCNF     bool                     `json:"condition_type_cnf"`
	ServicePointTriggers []IMSServicePointTrigger `json:"spt"`
}

type IMSServicePointTrigger struct {
	ConditionNegated   bool                   `json:"condition_negated,omitempty"`
	Groups             []int32                `json:"group"`
	RequestURI         *string                `json:"request_uri,omitempty"`
	Method             *string                `json:"method,omitempty"`
	SIPHeader          *IMSHeaderMatch        `json:"sip_header,omitempty"`
	SessionCase        *IMSSessionCase        `json:"session_case,omitempty"`
	SessionDescription *IMSSessionDescription `json:"session_description,omitempty"`
	RegistrationTypes  []IMSRegistrationType  `json:"registration_type,omitempty"`
}

type IMSHeaderMatch struct {
	Header  string  `json:"header"`
	Content *string `json:"content,omitempty"`
}

type IMSSessionDescription struct {
	Line    string  `json:"line"`
	Content *string `json:"content,omitempty"`
}

type imsServiceRow struct {
	ID                  string `db:"id"`
	Name                string `db:"name"`
	Kind                string `db:"kind"`
	ApplicationServerID string `db:"applicationServerID"`
}

type imsIFCRow struct {
	ServiceID               string `db:"serviceID"`
	Position                int    `db:"position"`
	TriggerPoint            string `db:"triggerPoint"`
	DefaultHandling         string `db:"defaultHandling"`
	IncludeRegisterRequest  bool   `db:"includeRegisterRequest"`
	IncludeRegisterResponse bool   `db:"includeRegisterResponse"`
}

type profileIMSServiceRow struct {
	ProfileID string `db:"profileID"`
	ServiceID string `db:"serviceID"`
	Position  int    `db:"position"`
}

type profileIMSServicesPayload struct {
	ProfileID  string   `json:"profile_id"`
	ServiceIDs []string `json:"service_ids"`
}

func (s IMSService) Validate() error {
	if s.Name == "" {
		return errors.New("IMS service name is required")
	}

	if s.ApplicationServerID == "" {
		return errors.New("IMS service application server is required")
	}

	switch s.Kind {
	case IMSServiceKindMCPTT:
		if len(s.InitialFilterCriteria) > 0 {
			return fmt.Errorf("an IMS service of kind %s takes no initial filter criteria", s.Kind)
		}
	case IMSServiceKindCustom:
		if len(s.InitialFilterCriteria) == 0 {
			return fmt.Errorf("an IMS service of kind %s needs at least one initial filter criterion", s.Kind)
		}
	default:
		return fmt.Errorf("unknown IMS service kind %q", s.Kind)
	}

	for i, ifc := range s.InitialFilterCriteria {
		if err := ifc.Validate(); err != nil {
			return fmt.Errorf("initial filter criterion %d: %w", i, err)
		}
	}

	return nil
}

func (c IMSInitialFilterCriterion) Validate() error {
	switch c.DefaultHandling {
	case IMSDefaultHandlingSessionContinued, IMSDefaultHandlingSessionTerminated:
	default:
		return fmt.Errorf("unknown default handling %q", c.DefaultHandling)
	}

	return c.TriggerPoint.Validate()
}

func (t IMSTriggerPoint) Validate() error {
	if len(t.ServicePointTriggers) == 0 {
		return errors.New("trigger point needs at least one service point trigger")
	}

	for i, spt := range t.ServicePointTriggers {
		if err := spt.Validate(); err != nil {
			return fmt.Errorf("service point trigger %d: %w", i, err)
		}
	}

	return nil
}

func (spt IMSServicePointTrigger) Validate() error {
	if len(spt.Groups) == 0 {
		return errors.New("at least one group is required")
	}

	for _, g := range spt.Groups {
		if g < 0 {
			return fmt.Errorf("group must not be negative, got %d", g)
		}
	}

	conditions := 0

	if spt.RequestURI != nil {
		conditions++

		if *spt.RequestURI == "" {
			return errors.New("request URI must not be empty")
		}
	}

	if spt.Method != nil {
		conditions++

		if *spt.Method == "" {
			return errors.New("method must not be empty")
		}
	}

	if spt.SIPHeader != nil {
		conditions++

		if spt.SIPHeader.Header == "" {
			return errors.New("SIP header name must not be empty")
		}
	}

	if spt.SessionCase != nil {
		conditions++

		switch *spt.SessionCase {
		case IMSSessionCaseOriginatingRegistered, IMSSessionCaseTerminatingRegistered, IMSSessionCaseTerminatingUnregistered,
			IMSSessionCaseOriginatingUnregistered, IMSSessionCaseOriginatingCDIV:
		default:
			return fmt.Errorf("unknown session case %q", *spt.SessionCase)
		}
	}

	if spt.SessionDescription != nil {
		conditions++

		if spt.SessionDescription.Line == "" {
			return errors.New("session description line must not be empty")
		}
	}

	if conditions != 1 {
		return fmt.Errorf("exactly one of request URI, method, SIP header, session case and session description is required, got %d", conditions)
	}

	if len(spt.RegistrationTypes) == 0 {
		return nil
	}

	if spt.Method == nil || *spt.Method != "REGISTER" {
		return errors.New("registration types only apply to the REGISTER method")
	}

	if len(spt.RegistrationTypes) > maxRegistrationTypesPerSPT {
		return fmt.Errorf("at most %d registration types are allowed, got %d", maxRegistrationTypesPerSPT, len(spt.RegistrationTypes))
	}

	for i, rt := range spt.RegistrationTypes {
		switch rt {
		case IMSRegistrationTypeInitial, IMSRegistrationTypeReRegistration, IMSRegistrationTypeDeregistration:
		default:
			return fmt.Errorf("unknown registration type %q", rt)
		}

		if slices.Contains(spt.RegistrationTypes[:i], rt) {
			return fmt.Errorf("registration type %s is listed twice", rt)
		}
	}

	return nil
}

func (db *Database) ListIMSServices(ctx context.Context) ([]IMSService, error) {
	ctx, span := startIMSServicesSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSServicesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSServicesTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return nil, nil
	}

	var rows []imsServiceRow

	if err := db.conn().Query(ctx, db.listIMSServicesStmt).GetAll(&rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)
		return nil, fmt.Errorf("query failed: %w", err)
	}

	var ifcs []imsIFCRow

	if err := db.conn().Query(ctx, db.listIMSIFCsStmt).GetAll(&ifcs); err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)
		return nil, fmt.Errorf("query failed: %w", err)
	}

	services := make([]IMSService, 0, len(rows))

	for _, r := range rows {
		var own []imsIFCRow

		for _, ifc := range ifcs {
			if ifc.ServiceID == r.ID {
				own = append(own, ifc)
			}
		}

		svc, err := imsServiceOf(r, own)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}

		services = append(services, svc)
	}

	return services, nil
}

func (db *Database) GetIMSService(ctx context.Context, name string) (*IMSService, error) {
	ctx, span := startIMSServicesSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSServicesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSServicesTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return nil, ErrNotFound
	}

	row := imsServiceRow{Name: name}

	if err := db.conn().Query(ctx, db.getIMSServiceStmt, row).Get(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	svc, err := db.loadIMSService(ctx, row)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return &svc, nil
}

func (db *Database) ListProfileIMSServices(ctx context.Context, profileID string) ([]IMSService, error) {
	ctx, span := startIMSServicesSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ProfileIMSServicesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ProfileIMSServicesTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return nil, nil
	}

	var rows []imsServiceRow

	if err := db.conn().Query(ctx, db.listProfileIMSServicesStmt, profileIMSServiceRow{ProfileID: profileID}).GetAll(&rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)
		return nil, fmt.Errorf("query failed: %w", err)
	}

	services := make([]IMSService, 0, len(rows))

	for _, r := range rows {
		svc, err := db.loadIMSService(ctx, r)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}

		services = append(services, svc)
	}

	return services, nil
}

func (db *Database) loadIMSService(ctx context.Context, row imsServiceRow) (IMSService, error) {
	var ifcs []imsIFCRow

	if err := db.conn().Query(ctx, db.listIMSIFCsByServiceStmt, imsIFCRow{ServiceID: row.ID}).GetAll(&ifcs); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return IMSService{}, fmt.Errorf("query failed: %w", err)
	}

	return imsServiceOf(row, ifcs)
}

func imsServiceOf(row imsServiceRow, ifcs []imsIFCRow) (IMSService, error) {
	svc := IMSService{ID: row.ID, Name: row.Name, Kind: IMSServiceKind(row.Kind), ApplicationServerID: row.ApplicationServerID}

	for _, r := range ifcs {
		ifc := IMSInitialFilterCriterion{
			DefaultHandling:         IMSDefaultHandling(r.DefaultHandling),
			IncludeRegisterRequest:  r.IncludeRegisterRequest,
			IncludeRegisterResponse: r.IncludeRegisterResponse,
		}

		if err := json.Unmarshal([]byte(r.TriggerPoint), &ifc.TriggerPoint); err != nil {
			return IMSService{}, fmt.Errorf("decode trigger point of IMS service %s: %w", row.Name, err)
		}

		svc.InitialFilterCriteria = append(svc.InitialFilterCriteria, ifc)
	}

	return svc, nil
}

func (db *Database) CreateIMSService(ctx context.Context, svc *IMSService) error {
	if svc.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate IMS service id: %w", err)
		}

		svc.ID = id.String()
	}

	return db.writeIMSService(ctx, "INSERT", opCreateIMSService, svc)
}

func (db *Database) UpdateIMSService(ctx context.Context, svc *IMSService) error {
	return db.writeIMSService(ctx, "UPDATE", opUpdateIMSService, svc)
}

func (db *Database) writeIMSService(ctx context.Context, operation string, op *ChangesetOp[IMSService, struct{}], svc *IMSService) error {
	if err := svc.Validate(); err != nil {
		return err
	}

	ctx, span := startIMSServicesSpan(ctx, operation)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSServicesTableName, strings.ToLower(operation)))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSServicesTableName, strings.ToLower(operation)).Inc()

	if _, err := op.Invoke(ctx, db, svc); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) DeleteIMSService(ctx context.Context, name string) error {
	ctx, span := startIMSServicesSpan(ctx, "DELETE")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSServicesTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSServicesTableName, "delete").Inc()

	if _, err := opDeleteIMSService.Invoke(ctx, db, &stringPayload{Value: name}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) SetProfileIMSServices(ctx context.Context, profileID string, serviceIDs []string) error {
	for i, id := range serviceIDs {
		if slices.Contains(serviceIDs[:i], id) {
			return fmt.Errorf("IMS service %s is listed twice", id)
		}
	}

	ctx, span := startIMSServicesSpan(ctx, "UPDATE")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ProfileIMSServicesTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ProfileIMSServicesTableName, "update").Inc()

	if _, err := opSetProfileIMSServices.Invoke(ctx, db, &profileIMSServicesPayload{ProfileID: profileID, ServiceIDs: serviceIDs}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) applyCreateIMSService(ctx context.Context, svc *IMSService) (any, error) {
	row := imsServiceRow{ID: svc.ID, Name: svc.Name, Kind: string(svc.Kind), ApplicationServerID: svc.ApplicationServerID}

	if err := db.runner(ctx).Query(ctx, db.insertIMSServiceStmt, row).Run(); err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		if isForeignKeyError(err) {
			return nil, fmt.Errorf("application server %s: %w", svc.ApplicationServerID, ErrNotFound)
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, db.insertIMSIFCs(ctx, svc.ID, svc.InitialFilterCriteria)
}

func (db *Database) applyUpdateIMSService(ctx context.Context, svc *IMSService) (any, error) {
	row := imsServiceRow{Name: svc.Name}

	if err := db.runner(ctx).Query(ctx, db.getIMSServiceStmt, row).Get(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	row.Kind = string(svc.Kind)
	row.ApplicationServerID = svc.ApplicationServerID

	if err := db.runner(ctx).Query(ctx, db.updateIMSServiceStmt, row).Run(); err != nil {
		if isForeignKeyError(err) {
			return nil, fmt.Errorf("application server %s: %w", svc.ApplicationServerID, ErrNotFound)
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	if err := db.runner(ctx).Query(ctx, db.deleteIMSIFCsByServiceStmt, imsIFCRow{ServiceID: row.ID}).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, db.insertIMSIFCs(ctx, row.ID, svc.InitialFilterCriteria)
}

func (db *Database) insertIMSIFCs(ctx context.Context, serviceID string, ifcs []IMSInitialFilterCriterion) error {
	for i, ifc := range ifcs {
		trigger, err := json.Marshal(ifc.TriggerPoint)
		if err != nil {
			return fmt.Errorf("encode trigger point: %w", err)
		}

		row := imsIFCRow{
			ServiceID:               serviceID,
			Position:                i,
			TriggerPoint:            string(trigger),
			DefaultHandling:         string(ifc.DefaultHandling),
			IncludeRegisterRequest:  ifc.IncludeRegisterRequest,
			IncludeRegisterResponse: ifc.IncludeRegisterResponse,
		}

		if err := db.runner(ctx).Query(ctx, db.insertIMSIFCStmt, row).Run(); err != nil {
			return fmt.Errorf("query failed: %w", err)
		}
	}

	return nil
}

func (db *Database) applyDeleteIMSService(ctx context.Context, p *stringPayload) (any, error) {
	var outcome sqlair.Outcome

	if err := db.runner(ctx).Query(ctx, db.deleteIMSServiceStmt, imsServiceRow{Name: p.Value}).Get(&outcome); err != nil {
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

func (db *Database) applySetProfileIMSServices(ctx context.Context, p *profileIMSServicesPayload) (any, error) {
	if err := db.runner(ctx).Query(ctx, db.deleteProfileIMSServicesStmt, profileIMSServiceRow{ProfileID: p.ProfileID}).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	for i, serviceID := range p.ServiceIDs {
		row := profileIMSServiceRow{ProfileID: p.ProfileID, ServiceID: serviceID, Position: i}

		if err := db.runner(ctx).Query(ctx, db.insertProfileIMSServiceStmt, row).Run(); err != nil {
			if isForeignKeyError(err) {
				return nil, fmt.Errorf("profile %s or IMS service %s: %w", p.ProfileID, serviceID, ErrNotFound)
			}

			return nil, fmt.Errorf("query failed: %w", err)
		}
	}

	return nil, nil
}

func startIMSServicesSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	querySummary := fmt.Sprintf("%s %s", operation, IMSServicesTableName)

	return tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(IMSServicesTableName),
		),
	)
}
