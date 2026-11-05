// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/canonical/sqlair"
	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	SMSCsTableName              = "sms_smscs"
	DiameterPeersTableName      = "diameter_peers"
	SMSCServiceCentresTableName = "sms_smsc_service_centres"
)

const smscNodesSchema = 26

const (
	MaxSMSCs                     = 16
	MaxSMSCNodes                 = 8
	MaxSMSCServiceCentres        = 16
	maxDiameterIdentityLength    = 255
	maxDiameterIdentityLabelSize = 63
	maxSMSCNameLength            = 255
)

const (
	listSMSCsStmt                = "SELECT &smscRow.* FROM %s ORDER BY id"
	listSMSCNodesStmt            = "SELECT &smscNodeRow.* FROM %s WHERE role='smsc' ORDER BY id"
	listSMSCServiceCentresStmt   = "SELECT &smscServiceCentreRow.* FROM %s ORDER BY serviceCentre"
	insertSMSCStmt               = "INSERT INTO %s (id, name) VALUES ($smscRow.id, $smscRow.name)"
	deleteSMSCStmt               = "DELETE FROM %s WHERE id==$smscRow.id"
	insertSMSCNodeStmt           = "INSERT INTO %s (id, role, diameterIdentity, address, port, smscId) VALUES ($smscNodeRow.id, 'smsc', $smscNodeRow.diameterIdentity, $smscNodeRow.address, $smscNodeRow.port, $smscNodeRow.smscId)"
	deleteSMSCNodesStmt          = "DELETE FROM %s WHERE smscId==$smscNodeRow.smscId AND role='smsc'"
	insertSMSCServiceCentreStmt  = "INSERT INTO %s (serviceCentre, smscId) VALUES ($smscServiceCentreRow.serviceCentre, $smscServiceCentreRow.smscId)"
	deleteSMSCServiceCentresStmt = "DELETE FROM %s WHERE smscId==$smscServiceCentreRow.smscId"
)

var ErrSMSCConflict = errors.New("SMSC conflict")

type SMSC struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	ServiceCentres []string   `json:"service_centres"`
	Nodes          []SMSCNode `json:"nodes"`
}

type SMSCNode struct {
	ID               string `json:"id"`
	DiameterIdentity string `json:"diameter_identity"`
	Address          string `json:"address"`
	Port             int    `json:"port"`
}

type smscRow struct {
	ID   string `db:"id"`
	Name string `db:"name"`
}

type smscNodeRow struct {
	ID               string `db:"id"`
	DiameterIdentity string `db:"diameterIdentity"`
	Address          string `db:"address"`
	Port             int    `db:"port"`
	SMSCID           string `db:"smscId"`
}

type smscServiceCentreRow struct {
	ServiceCentre string `db:"serviceCentre"`
	SMSCID        string `db:"smscId"`
}

func (s SMSC) Validate() error {
	if s.ID == "" {
		return errors.New("SMSC ID is required")
	}

	if s.Name == "" || len(s.Name) > maxSMSCNameLength {
		return fmt.Errorf("SMSC name must be 1 to %d bytes", maxSMSCNameLength)
	}

	if len(s.ServiceCentres) == 0 {
		return errors.New("SMSC needs at least one service centre number")
	}

	if len(s.ServiceCentres) > MaxSMSCServiceCentres {
		return fmt.Errorf("SMSC serves at most %d service centre numbers", MaxSMSCServiceCentres)
	}

	for i, sc := range s.ServiceCentres {
		if !IsValidMSISDN(sc) {
			return fmt.Errorf("service centre number must be 1 to %d digits in E.164 international format, got %q", maxMSISDNDigits, sc)
		}

		if slices.Contains(s.ServiceCentres[:i], sc) {
			return fmt.Errorf("service centre number %s is listed twice", sc)
		}
	}

	if len(s.Nodes) == 0 {
		return errors.New("SMSC needs at least one node")
	}

	if len(s.Nodes) > MaxSMSCNodes {
		return fmt.Errorf("SMSC has at most %d nodes", MaxSMSCNodes)
	}

	for _, n := range s.Nodes {
		if err := n.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (n SMSCNode) Validate() error {
	if n.ID == "" {
		return errors.New("SMSC node ID is required")
	}

	if n.DiameterIdentity != "" && !IsValidDiameterIdentity(n.DiameterIdentity) {
		return fmt.Errorf("SMSC node Diameter identity must be a fully qualified domain name, got %q", n.DiameterIdentity)
	}

	addr, err := netip.ParseAddr(n.Address)
	if err != nil || addr.Zone() != "" || addr.IsUnspecified() || addr.Unmap().String() != n.Address {
		return fmt.Errorf("SMSC node address must be an IPv4 or IPv6 address in canonical form, got %q", n.Address)
	}

	if n.Port < 1 || n.Port > 65535 {
		return fmt.Errorf("SMSC node port must be between 1 and 65535, got %d", n.Port)
	}

	return nil
}

func (s SMSC) Serves(serviceCentre string) bool {
	return slices.Contains(s.ServiceCentres, serviceCentre)
}

func IsValidDiameterIdentity(identity string) bool {
	if len(identity) > maxDiameterIdentityLength {
		return false
	}

	labels := strings.Split(identity, ".")
	if len(labels) < 2 {
		return false
	}

	for _, label := range labels {
		if label == "" || len(label) > maxDiameterIdentityLabelSize || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}

		for i := range len(label) {
			c := label[i]
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '_' {
				return false
			}
		}
	}

	return true
}

func ValidateSMSCs(smscs []SMSC) error {
	if len(smscs) > MaxSMSCs {
		return fmt.Errorf("at most %d SMSCs can be configured", MaxSMSCs)
	}

	var nodes []SMSCNode

	for i, s := range smscs {
		for _, other := range smscs[:i] {
			if strings.EqualFold(s.Name, other.Name) {
				return fmt.Errorf("another SMSC is named %s", s.Name)
			}

			for _, sc := range s.ServiceCentres {
				if other.Serves(sc) {
					return fmt.Errorf("another SMSC serves the service centre number +%s", sc)
				}
			}
		}

		nodes = append(nodes, s.Nodes...)
	}

	for i, n := range nodes {
		for _, other := range nodes[:i] {
			if n.ID == other.ID {
				return fmt.Errorf("SMSC node ID %s is used twice", n.ID)
			}

			if n.DiameterIdentity != "" && strings.EqualFold(n.DiameterIdentity, other.DiameterIdentity) {
				return fmt.Errorf("another SMSC node has the Diameter identity %s", n.DiameterIdentity)
			}

			if n.Address == other.Address && n.Port == other.Port {
				return fmt.Errorf("another SMSC node has the address %s", net.JoinHostPort(n.Address, strconv.Itoa(n.Port)))
			}

			if n.DiameterIdentity == "" && other.DiameterIdentity == "" && n.Address == other.Address {
				return fmt.Errorf("another SMSC node without a Diameter identity has the address %s", n.Address)
			}
		}
	}

	return nil
}

func (db *Database) ListSMSCs(ctx context.Context) ([]SMSC, error) {
	ctx, span := startSMSCsSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SMSCsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SMSCsTableName, "select").Inc()

	smscs, err := db.listSMSCs(ctx, db.conn())
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return smscs, nil
}

func (db *Database) GetSMSC(ctx context.Context, name string) (*SMSC, error) {
	smscs, err := db.ListSMSCs(ctx)
	if err != nil {
		return nil, err
	}

	for _, s := range smscs {
		if s.Name == name {
			return &s, nil
		}
	}

	return nil, ErrNotFound
}

func (db *Database) CreateSMSC(ctx context.Context, smsc *SMSC) error {
	return db.writeSMSC(ctx, "INSERT", opCreateSMSC, smsc)
}

func (db *Database) UpdateSMSC(ctx context.Context, smsc *SMSC) error {
	return db.writeSMSC(ctx, "UPDATE", opUpdateSMSC, smsc)
}

func (db *Database) writeSMSC(ctx context.Context, operation string, op *ChangesetOp[SMSC, struct{}], smsc *SMSC) error {
	if err := smsc.Validate(); err != nil {
		return err
	}

	ctx, span := startSMSCsSpan(ctx, operation)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SMSCsTableName, strings.ToLower(operation)))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SMSCsTableName, strings.ToLower(operation)).Inc()

	if _, err := op.Invoke(ctx, db, smsc); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) DeleteSMSC(ctx context.Context, id string) error {
	ctx, span := startSMSCsSpan(ctx, "DELETE")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SMSCsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SMSCsTableName, "delete").Inc()

	if _, err := opDeleteSMSC.Invoke(ctx, db, &stringPayload{Value: id}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) listSMSCs(ctx context.Context, runner *sqlair.DB) ([]SMSC, error) {
	if !db.appliedSchemaAtLeast(ctx, smsSchema) {
		return nil, nil
	}

	if !db.appliedSchemaAtLeast(ctx, smscNodesSchema) {
		peers, err := db.listLegacySMSCPeers(ctx, runner)
		if err != nil {
			return nil, err
		}

		return legacySMSCs(peers), nil
	}

	var rows []smscRow

	if err := runner.Query(ctx, db.listSMSCsStmt).GetAll(&rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	var nodes []smscNodeRow

	if err := runner.Query(ctx, db.listSMSCNodesStmt).GetAll(&nodes); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	var centres []smscServiceCentreRow

	if err := runner.Query(ctx, db.listSMSCServiceCentresStmt).GetAll(&centres); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	smscs := make([]SMSC, 0, len(rows))

	for _, r := range rows {
		s := SMSC{ID: r.ID, Name: r.Name, ServiceCentres: []string{}, Nodes: []SMSCNode{}}

		for _, n := range nodes {
			if n.SMSCID == r.ID {
				s.Nodes = append(s.Nodes, SMSCNode{ID: n.ID, DiameterIdentity: n.DiameterIdentity, Address: n.Address, Port: n.Port})
			}
		}

		for _, c := range centres {
			if c.SMSCID == r.ID {
				s.ServiceCentres = append(s.ServiceCentres, c.ServiceCentre)
			}
		}

		smscs = append(smscs, s)
	}

	return smscs, nil
}

func (db *Database) applyCreateSMSC(ctx context.Context, s *SMSC) (any, error) {
	smscs, err := db.listSMSCs(ctx, db.runner(ctx))
	if err != nil {
		return nil, err
	}

	if slices.ContainsFunc(smscs, func(other SMSC) bool { return other.ID == s.ID || strings.EqualFold(other.Name, s.Name) }) {
		return nil, ErrAlreadyExists
	}

	if err := ValidateSMSCs(append(smscs, *s)); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSMSCConflict, err)
	}

	if err := db.runner(ctx).Query(ctx, db.insertSMSCStmt, smscRow{ID: s.ID, Name: s.Name}).Run(); err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, db.insertSMSCChildren(ctx, s)
}

func (db *Database) applyUpdateSMSC(ctx context.Context, s *SMSC) (any, error) {
	smscs, err := db.listSMSCs(ctx, db.runner(ctx))
	if err != nil {
		return nil, err
	}

	i := slices.IndexFunc(smscs, func(other SMSC) bool { return other.ID == s.ID })
	if i < 0 || smscs[i].Name != s.Name {
		return nil, ErrNotFound
	}

	smscs[i] = *s

	if err := ValidateSMSCs(smscs); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSMSCConflict, err)
	}

	if err := db.deleteSMSCChildren(ctx, s.ID); err != nil {
		return nil, err
	}

	return nil, db.insertSMSCChildren(ctx, s)
}

func (db *Database) applyDeleteSMSC(ctx context.Context, p *stringPayload) (any, error) {
	smscs, err := db.listSMSCs(ctx, db.runner(ctx))
	if err != nil {
		return nil, err
	}

	if !slices.ContainsFunc(smscs, func(other SMSC) bool { return other.ID == p.Value }) {
		return nil, ErrNotFound
	}

	if err := db.deleteSMSCChildren(ctx, p.Value); err != nil {
		return nil, err
	}

	if err := db.runner(ctx).Query(ctx, db.deleteSMSCStmt, smscRow{ID: p.Value}).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) deleteSMSCChildren(ctx context.Context, id string) error {
	if err := db.runner(ctx).Query(ctx, db.deleteSMSCServiceCentresStmt, smscServiceCentreRow{SMSCID: id}).Run(); err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	if err := db.runner(ctx).Query(ctx, db.deleteSMSCNodesStmt, smscNodeRow{SMSCID: id}).Run(); err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) insertSMSCChildren(ctx context.Context, s *SMSC) error {
	for _, n := range s.Nodes {
		row := smscNodeRow{ID: n.ID, DiameterIdentity: n.DiameterIdentity, Address: n.Address, Port: n.Port, SMSCID: s.ID}

		if err := db.runner(ctx).Query(ctx, db.insertSMSCNodeStmt, row).Run(); err != nil {
			if isUniqueNameError(err) {
				return fmt.Errorf("%w: SMSC node %s or its Diameter identity is already in use", ErrSMSCConflict, n.ID)
			}

			return fmt.Errorf("query failed: %w", err)
		}
	}

	for _, sc := range s.ServiceCentres {
		if err := db.runner(ctx).Query(ctx, db.insertSMSCServiceCentreStmt, smscServiceCentreRow{ServiceCentre: sc, SMSCID: s.ID}).Run(); err != nil {
			if isUniqueNameError(err) {
				return fmt.Errorf("%w: another SMSC serves the service centre number +%s", ErrSMSCConflict, sc)
			}

			return fmt.Errorf("query failed: %w", err)
		}
	}

	return nil
}

func startSMSCsSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	querySummary := fmt.Sprintf("%s %s", operation, SMSCsTableName)

	return tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(SMSCsTableName),
		),
	)
}
