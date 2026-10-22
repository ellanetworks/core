// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"

	"github.com/ellanetworks/core/diameter/cx"
	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const IMSInitialFilterCriteriaTableName = "ims_initial_filter_criteria"

const (
	listProfileIMSIFCsStmt          = "SELECT &imsIFCRow.* FROM %s WHERE profileID==$imsIFCRow.profileID ORDER BY position"
	insertIMSIFCStmt                = "INSERT INTO %s (profileID, position, applicationServerID, triggerPoint, profilePart, defaultHandling, serviceInfo, includeRegisterRequest, includeRegisterResponse) VALUES ($imsIFCRow.profileID, $imsIFCRow.position, $imsIFCRow.applicationServerID, $imsIFCRow.triggerPoint, $imsIFCRow.profilePart, $imsIFCRow.defaultHandling, $imsIFCRow.serviceInfo, $imsIFCRow.includeRegisterRequest, $imsIFCRow.includeRegisterResponse)"
	deleteProfileIMSIFCsStmt        = "DELETE FROM %s WHERE profileID==$imsIFCRow.profileID"
	listProfilesOfIMSASStmt         = "SELECT DISTINCT &imsIFCRow.profileID FROM %s WHERE applicationServerID==$imsIFCRow.applicationServerID"
	listIMSRegisteredOfProfilesStmt = "SELECT s.imsi AS &subscriberIMSI.imsi FROM %s AS s JOIN %s AS r ON r.imsi = s.imsi WHERE r.state != $imsRegistrationRow.state AND s.profileID IN ($SliceIDs[:]) ORDER BY s.imsi"
	listSubscriberIMSIFCsStmt       = "SELECT f.* AS &imsIFCRow.*, a.serverName AS &imsApplicationServerRow.serverName FROM %s AS f JOIN %s AS s ON s.profileID = f.profileID JOIN %s AS a ON a.id = f.applicationServerID WHERE s.imsi = $subscriberIMSI.imsi ORDER BY f.position"
)

type IMSProfilePart string

const (
	IMSProfilePartRegistered   IMSProfilePart = "REGISTERED"
	IMSProfilePartUnregistered IMSProfilePart = "UNREGISTERED"
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

type IMSInitialFilterCriterion struct {
	ApplicationServerID     string              `json:"application_server_id"`
	TriggerPoint            *IMSTriggerPoint    `json:"trigger_point,omitempty"`
	ProfilePart             *IMSProfilePart     `json:"profile_part,omitempty"`
	DefaultHandling         *IMSDefaultHandling `json:"default_handling,omitempty"`
	ServiceInfo             *string             `json:"service_info,omitempty"`
	IncludeRegisterRequest  bool                `json:"include_register_request,omitempty"`
	IncludeRegisterResponse bool                `json:"include_register_response,omitempty"`
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

type imsIFCRow struct {
	ProfileID               string  `db:"profileID"`
	Position                int     `db:"position"`
	ApplicationServerID     string  `db:"applicationServerID"`
	TriggerPoint            *string `db:"triggerPoint"`
	ProfilePart             *string `db:"profilePart"`
	DefaultHandling         *string `db:"defaultHandling"`
	ServiceInfo             *string `db:"serviceInfo"`
	IncludeRegisterRequest  bool    `db:"includeRegisterRequest"`
	IncludeRegisterResponse bool    `db:"includeRegisterResponse"`
}

func (c IMSInitialFilterCriterion) Validate() error {
	if c.ApplicationServerID == "" {
		return errors.New("application server is required")
	}

	if c.DefaultHandling != nil && *c.DefaultHandling != IMSDefaultHandlingSessionContinued && *c.DefaultHandling != IMSDefaultHandlingSessionTerminated {
		return fmt.Errorf("unknown default handling %q", *c.DefaultHandling)
	}

	if c.ProfilePart != nil && *c.ProfilePart != IMSProfilePartRegistered && *c.ProfilePart != IMSProfilePartUnregistered {
		return fmt.Errorf("unknown profile part %q", *c.ProfilePart)
	}

	if c.TriggerPoint != nil {
		if err := c.TriggerPoint.Validate(); err != nil {
			return err
		}
	}

	return c.CX(0, "sip:validation.invalid").Validate()
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
	conditions := 0

	for _, set := range []bool{spt.RequestURI != nil, spt.Method != nil, spt.SIPHeader != nil, spt.SessionCase != nil, spt.SessionDescription != nil} {
		if set {
			conditions++
		}
	}

	if conditions != 1 {
		return fmt.Errorf("exactly one of request URI, method, SIP header, session case and session description is required, got %d", conditions)
	}

	switch {
	case spt.Method != nil && *spt.Method == "":
		return errors.New("method must not be empty")
	case spt.SessionCase != nil && !slices.Contains(imsSessionCases, *spt.SessionCase):
		return fmt.Errorf("unknown session case %q", *spt.SessionCase)
	case len(spt.RegistrationTypes) > 0 && (spt.Method == nil || *spt.Method != "REGISTER"):
		return errors.New("registration types only apply to the REGISTER method")
	}

	for i, rt := range spt.RegistrationTypes {
		if !slices.Contains(imsRegistrationTypes, rt) {
			return fmt.Errorf("unknown registration type %q", rt)
		}

		if slices.Contains(spt.RegistrationTypes[:i], rt) {
			return fmt.Errorf("registration type %s is listed twice", rt)
		}
	}

	if err := spt.validateExpressions(); err != nil {
		return err
	}

	return spt.CX().Validate()
}

func (spt IMSServicePointTrigger) validateExpressions() error {
	type expression struct {
		name     string
		value    *string
		required bool
	}

	expressions := []expression{{name: "request URI", value: spt.RequestURI, required: true}}

	if h := spt.SIPHeader; h != nil {
		expressions = append(expressions, expression{name: "SIP header name", value: &h.Header, required: true}, expression{name: "SIP header content", value: h.Content})
	}

	if d := spt.SessionDescription; d != nil {
		expressions = append(expressions, expression{name: "session description line", value: &d.Line, required: true}, expression{name: "session description content", value: d.Content})
	}

	for _, e := range expressions {
		if e.value == nil {
			continue
		}

		if e.required && *e.value == "" {
			return fmt.Errorf("%s must not be empty", e.name)
		}

		if _, err := regexp.CompilePOSIX(*e.value); err != nil {
			return fmt.Errorf("%s %q is not an extended regular expression: %w", e.name, *e.value, err)
		}
	}

	return nil
}

var imsSessionCases = []IMSSessionCase{
	IMSSessionCaseOriginatingRegistered,
	IMSSessionCaseTerminatingRegistered,
	IMSSessionCaseTerminatingUnregistered,
	IMSSessionCaseOriginatingUnregistered,
	IMSSessionCaseOriginatingCDIV,
}

var imsRegistrationTypes = []IMSRegistrationType{
	IMSRegistrationTypeInitial,
	IMSRegistrationTypeReRegistration,
	IMSRegistrationTypeDeregistration,
}

func (t IMSTriggerPoint) CX() *cx.TriggerPoint {
	trigger := &cx.TriggerPoint{ConjunctiveNormalForm: t.ConditionTypeCNF}

	for _, spt := range t.ServicePointTriggers {
		trigger.ServicePointTriggers = append(trigger.ServicePointTriggers, spt.CX())
	}

	return trigger
}

func (spt IMSServicePointTrigger) CX() cx.ServicePointTrigger {
	out := cx.ServicePointTrigger{
		Negated:    spt.ConditionNegated,
		Groups:     slices.Clone(spt.Groups),
		RequestURI: spt.RequestURI,
		Method:     spt.Method,
	}

	if h := spt.SIPHeader; h != nil {
		out.SIPHeader = &cx.HeaderMatch{Header: h.Header, Content: h.Content}
	}

	if d := spt.SessionDescription; d != nil {
		out.SessionDescription = &cx.SessionDescriptionMatch{Line: d.Line, Content: d.Content}
	}

	if spt.SessionCase != nil {
		c := cx.SessionCase(slices.Index(imsSessionCases, *spt.SessionCase))
		out.SessionCase = &c
	}

	for _, rt := range spt.RegistrationTypes {
		out.RegistrationTypes = append(out.RegistrationTypes, cx.RegistrationType(slices.Index(imsRegistrationTypes, rt)))
	}

	return out
}

func (c IMSInitialFilterCriterion) CX(priority int32, serverName string) cx.InitialFilterCriteria {
	out := cx.InitialFilterCriteria{
		Priority: priority,
		ApplicationServer: cx.ApplicationServer{
			ServerName:              serverName,
			ServiceInfo:             c.ServiceInfo,
			IncludeRegisterRequest:  c.IncludeRegisterRequest,
			IncludeRegisterResponse: c.IncludeRegisterResponse,
		},
	}

	if c.TriggerPoint != nil {
		out.Trigger = c.TriggerPoint.CX()
	}

	if c.DefaultHandling != nil {
		handling := cx.DefaultHandlingSessionContinued
		if *c.DefaultHandling == IMSDefaultHandlingSessionTerminated {
			handling = cx.DefaultHandlingSessionTerminated
		}

		out.ApplicationServer.DefaultHandling = &handling
	}

	if c.ProfilePart != nil {
		part := cx.ProfilePartRegistered
		if *c.ProfilePart == IMSProfilePartUnregistered {
			part = cx.ProfilePartUnregistered
		}

		out.ProfilePart = &part
	}

	return out
}

type profileIMSIFCsPayload struct {
	ProfileID             string                      `json:"profile_id"`
	InitialFilterCriteria []IMSInitialFilterCriterion `json:"initial_filter_criteria"`
}

func ValidateIMSFilterCriteria(ifcs []IMSInitialFilterCriterion) error {
	for i, ifc := range ifcs {
		if err := ifc.Validate(); err != nil {
			return fmt.Errorf("initial filter criterion %d: %w", i, err)
		}
	}

	return nil
}

func (db *Database) ListProfileIMSFilterCriteria(ctx context.Context, profileID string) ([]IMSInitialFilterCriterion, error) {
	ctx, span := startIMSFilterCriteriaSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSInitialFilterCriteriaTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSInitialFilterCriteriaTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return nil, nil
	}

	var rows []imsIFCRow

	if err := db.conn().Query(ctx, db.listProfileIMSIFCsStmt, imsIFCRow{ProfileID: profileID}).GetAll(&rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)
		return nil, fmt.Errorf("query failed: %w", err)
	}

	ifcs := make([]IMSInitialFilterCriterion, 0, len(rows))

	for _, r := range rows {
		ifc, err := imsIFCOf(r)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}

		ifcs = append(ifcs, ifc)
	}

	return ifcs, nil
}

func (db *Database) ListSubscriberIMSFilterCriteria(ctx context.Context, imsi string) ([]cx.InitialFilterCriteria, error) {
	ctx, span := startIMSFilterCriteriaSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSInitialFilterCriteriaTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSInitialFilterCriteriaTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return nil, nil
	}

	var (
		rows    []imsIFCRow
		servers []imsApplicationServerRow
	)

	if err := db.conn().Query(ctx, db.listSubscriberIMSIFCsStmt, subscriberIMSI{Imsi: imsi}).GetAll(&rows, &servers); err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if len(rows) > math.MaxInt32 {
		return nil, fmt.Errorf("subscriber %s has too many initial filter criteria", imsi)
	}

	ifcs := make([]cx.InitialFilterCriteria, 0, len(rows))

	for i, r := range rows {
		ifc, err := imsIFCOf(r)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}

		ifcs = append(ifcs, ifc.CX(int32(i), servers[i].ServerName))
	}

	return ifcs, nil
}

func imsIFCOf(r imsIFCRow) (IMSInitialFilterCriterion, error) {
	ifc := IMSInitialFilterCriterion{
		ApplicationServerID:     r.ApplicationServerID,
		ServiceInfo:             r.ServiceInfo,
		IncludeRegisterRequest:  r.IncludeRegisterRequest,
		IncludeRegisterResponse: r.IncludeRegisterResponse,
	}

	if r.DefaultHandling != nil {
		ifc.DefaultHandling = new(IMSDefaultHandling(*r.DefaultHandling))
	}

	if r.ProfilePart != nil {
		ifc.ProfilePart = new(IMSProfilePart(*r.ProfilePart))
	}

	if r.TriggerPoint != nil {
		ifc.TriggerPoint = new(IMSTriggerPoint)

		if err := json.Unmarshal([]byte(*r.TriggerPoint), ifc.TriggerPoint); err != nil {
			return IMSInitialFilterCriterion{}, fmt.Errorf("decode the trigger point of initial filter criterion %d of profile %s: %w", r.Position, r.ProfileID, err)
		}
	}

	return ifc, nil
}

func (db *Database) SetProfileIMSFilterCriteria(ctx context.Context, profileID string, ifcs []IMSInitialFilterCriterion) error {
	if err := ValidateIMSFilterCriteria(ifcs); err != nil {
		return err
	}

	ctx, span := startIMSFilterCriteriaSpan(ctx, "UPDATE")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSInitialFilterCriteriaTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSInitialFilterCriteriaTableName, "update").Inc()

	if _, err := opSetProfileIMSFilterCriteria.Invoke(ctx, db, &profileIMSIFCsPayload{ProfileID: profileID, InitialFilterCriteria: ifcs}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) applySetProfileIMSFilterCriteria(ctx context.Context, p *profileIMSIFCsPayload) (any, error) {
	if err := db.runner(ctx).Query(ctx, db.deleteProfileIMSIFCsStmt, imsIFCRow{ProfileID: p.ProfileID}).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	for i, ifc := range p.InitialFilterCriteria {
		row := imsIFCRow{
			ProfileID:               p.ProfileID,
			Position:                i,
			ApplicationServerID:     ifc.ApplicationServerID,
			ServiceInfo:             ifc.ServiceInfo,
			IncludeRegisterRequest:  ifc.IncludeRegisterRequest,
			IncludeRegisterResponse: ifc.IncludeRegisterResponse,
		}

		if ifc.TriggerPoint != nil {
			trigger, err := json.Marshal(ifc.TriggerPoint)
			if err != nil {
				return nil, fmt.Errorf("encode trigger point: %w", err)
			}

			row.TriggerPoint = new(string(trigger))
		}

		if ifc.DefaultHandling != nil {
			row.DefaultHandling = new(string(*ifc.DefaultHandling))
		}

		if ifc.ProfilePart != nil {
			row.ProfilePart = new(string(*ifc.ProfilePart))
		}

		if err := db.runner(ctx).Query(ctx, db.insertIMSIFCStmt, row).Run(); err != nil {
			if isForeignKeyError(err) {
				return nil, fmt.Errorf("profile %s or application server %s: %w", p.ProfileID, ifc.ApplicationServerID, ErrNotFound)
			}

			return nil, fmt.Errorf("query failed: %w", err)
		}
	}

	return nil, nil
}

func (db *Database) ProfilesUsingIMSApplicationServer(ctx context.Context, applicationServerID string) ([]string, error) {
	ctx, span := startIMSFilterCriteriaSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSInitialFilterCriteriaTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSInitialFilterCriteriaTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return nil, nil
	}

	var rows []imsIFCRow

	if err := db.conn().Query(ctx, db.listProfilesOfIMSASStmt, imsIFCRow{ApplicationServerID: applicationServerID}).GetAll(&rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)
		return nil, fmt.Errorf("query failed: %w", err)
	}

	profileIDs := make([]string, 0, len(rows))
	for _, r := range rows {
		profileIDs = append(profileIDs, r.ProfileID)
	}

	return profileIDs, nil
}

func (db *Database) IMSRegisteredSubscribersOfProfiles(ctx context.Context, profileIDs []string) ([]string, error) {
	ctx, span := startIMSFilterCriteriaSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSRegistrationsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSRegistrationsTableName, "select").Inc()

	if len(profileIDs) == 0 || !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return nil, nil
	}

	var rows []subscriberIMSI

	if err := db.conn().Query(ctx, db.listIMSRegisteredOfProfilesStmt, imsRegistrationRow{State: int(IMSNotRegistered)}, SliceIDs(profileIDs)).GetAll(&rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)
		return nil, fmt.Errorf("query failed: %w", err)
	}

	imsis := make([]string, 0, len(rows))
	for _, r := range rows {
		imsis = append(imsis, r.Imsi)
	}

	return imsis, nil
}

func startIMSFilterCriteriaSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	querySummary := fmt.Sprintf("%s %s", operation, IMSInitialFilterCriteriaTableName)

	return tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(IMSInitialFilterCriteriaTableName),
		),
	)
}
