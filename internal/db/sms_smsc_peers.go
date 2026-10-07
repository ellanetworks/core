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
	DiameterPeersTableName      = "diameter_peers"
	SMSCServiceCentresTableName = "sms_smsc_service_centres"
)

const (
	MaxSMSCPeers                 = 16
	MaxSMSCPeerServiceCentres    = 16
	maxDiameterIdentityLength    = 255
	maxDiameterIdentityLabelSize = 63
)

const (
	listSMSCPeersStmt            = "SELECT &smscPeerRow.* FROM %s WHERE role='smsc' ORDER BY id"
	listSMSCServiceCentresStmt   = "SELECT &smscServiceCentreRow.* FROM %s ORDER BY serviceCentre"
	insertSMSCPeerStmt           = "INSERT INTO %s (id, role, diameterIdentity, address, port) VALUES ($smscPeerRow.id, 'smsc', $smscPeerRow.diameterIdentity, $smscPeerRow.address, $smscPeerRow.port)"
	updateSMSCPeerStmt           = "UPDATE %s SET diameterIdentity=$smscPeerRow.diameterIdentity, address=$smscPeerRow.address, port=$smscPeerRow.port WHERE id==$smscPeerRow.id AND role='smsc'"
	deleteSMSCPeerStmt           = "DELETE FROM %s WHERE id==$smscPeerRow.id AND role='smsc'"
	insertSMSCServiceCentreStmt  = "INSERT INTO %s (serviceCentre, peerId) VALUES ($smscServiceCentreRow.serviceCentre, $smscServiceCentreRow.peerId)"
	deleteSMSCServiceCentresStmt = "DELETE FROM %s WHERE peerId==$smscServiceCentreRow.peerId"
)

var ErrSMSCPeerConflict = errors.New("SMSC peer conflict")

type SMSCPeer struct {
	ID               string   `json:"id"`
	DiameterIdentity string   `json:"diameter_identity"`
	Address          string   `json:"address"`
	Port             int      `json:"port"`
	ServiceCentres   []string `json:"service_centres"`
}

type smscPeerRow struct {
	ID               string `db:"id"`
	DiameterIdentity string `db:"diameterIdentity"`
	Address          string `db:"address"`
	Port             int    `db:"port"`
}

type smscServiceCentreRow struct {
	ServiceCentre string `db:"serviceCentre"`
	PeerID        string `db:"peerId"`
}

func (p SMSCPeer) Validate() error {
	if p.ID == "" {
		return errors.New("SMSC peer ID is required")
	}

	if !IsValidDiameterIdentity(p.DiameterIdentity) {
		return fmt.Errorf("SMSC Diameter identity must be a fully qualified domain name, got %q", p.DiameterIdentity)
	}

	addr, err := netip.ParseAddr(p.Address)
	if err != nil || addr.Zone() != "" || addr.IsUnspecified() || addr.Unmap().String() != p.Address {
		return fmt.Errorf("SMSC address must be an IPv4 or IPv6 address in canonical form, got %q", p.Address)
	}

	if p.Port < 1 || p.Port > 65535 {
		return fmt.Errorf("SMSC port must be between 1 and 65535, got %d", p.Port)
	}

	if len(p.ServiceCentres) == 0 {
		return errors.New("SMSC peer needs at least one service centre number")
	}

	if len(p.ServiceCentres) > MaxSMSCPeerServiceCentres {
		return fmt.Errorf("SMSC peer serves at most %d service centre numbers", MaxSMSCPeerServiceCentres)
	}

	for i, sc := range p.ServiceCentres {
		if !IsValidMSISDN(sc) {
			return fmt.Errorf("service centre number must be 1 to %d digits in E.164 international format, got %q", maxMSISDNDigits, sc)
		}

		if slices.Contains(p.ServiceCentres[:i], sc) {
			return fmt.Errorf("service centre number %s is listed twice", sc)
		}
	}

	return nil
}

func (p SMSCPeer) Serves(serviceCentre string) bool {
	return slices.Contains(p.ServiceCentres, serviceCentre)
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

func ValidateSMSCPeers(peers []SMSCPeer) error {
	if len(peers) > MaxSMSCPeers {
		return fmt.Errorf("at most %d SMSC peers can be configured", MaxSMSCPeers)
	}

	for i, p := range peers {
		for _, other := range peers[:i] {
			if strings.EqualFold(p.DiameterIdentity, other.DiameterIdentity) {
				return fmt.Errorf("another SMSC peer has the Diameter identity %s", p.DiameterIdentity)
			}

			if p.Address == other.Address && p.Port == other.Port {
				return fmt.Errorf("another SMSC peer has the address %s", net.JoinHostPort(p.Address, strconv.Itoa(p.Port)))
			}

			for _, sc := range p.ServiceCentres {
				if other.Serves(sc) {
					return fmt.Errorf("another SMSC peer serves the service centre number +%s", sc)
				}
			}
		}
	}

	return nil
}

func (db *Database) ListSMSCPeers(ctx context.Context) ([]SMSCPeer, error) {
	ctx, span := startSMSCPeersSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(DiameterPeersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(DiameterPeersTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, smsSchema) {
		return nil, nil
	}

	peers, err := db.listSMSCPeers(ctx, db.conn())
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return peers, nil
}

func (db *Database) GetSMSCPeer(ctx context.Context, id string) (*SMSCPeer, error) {
	peers, err := db.ListSMSCPeers(ctx)
	if err != nil {
		return nil, err
	}

	for _, p := range peers {
		if p.ID == id {
			return &p, nil
		}
	}

	return nil, ErrNotFound
}

func (db *Database) CreateSMSCPeer(ctx context.Context, peer *SMSCPeer) error {
	return db.writeSMSCPeer(ctx, "INSERT", opCreateSMSCPeer, peer)
}

func (db *Database) UpdateSMSCPeer(ctx context.Context, peer *SMSCPeer) error {
	return db.writeSMSCPeer(ctx, "UPDATE", opUpdateSMSCPeer, peer)
}

func (db *Database) writeSMSCPeer(ctx context.Context, operation string, op *ChangesetOp[SMSCPeer, struct{}], peer *SMSCPeer) error {
	if err := peer.Validate(); err != nil {
		return err
	}

	ctx, span := startSMSCPeersSpan(ctx, operation)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(DiameterPeersTableName, strings.ToLower(operation)))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(DiameterPeersTableName, strings.ToLower(operation)).Inc()

	if _, err := op.Invoke(ctx, db, peer); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) DeleteSMSCPeer(ctx context.Context, id string) error {
	ctx, span := startSMSCPeersSpan(ctx, "DELETE")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(DiameterPeersTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(DiameterPeersTableName, "delete").Inc()

	if _, err := opDeleteSMSCPeer.Invoke(ctx, db, &stringPayload{Value: id}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) listSMSCPeers(ctx context.Context, runner *sqlair.DB) ([]SMSCPeer, error) {
	var rows []smscPeerRow

	if err := runner.Query(ctx, db.listSMSCPeersStmt).GetAll(&rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	var centres []smscServiceCentreRow

	if err := runner.Query(ctx, db.listSMSCServiceCentresStmt).GetAll(&centres); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	peers := make([]SMSCPeer, 0, len(rows))

	for _, r := range rows {
		p := SMSCPeer{ID: r.ID, DiameterIdentity: r.DiameterIdentity, Address: r.Address, Port: r.Port, ServiceCentres: []string{}}

		for _, c := range centres {
			if c.PeerID == r.ID {
				p.ServiceCentres = append(p.ServiceCentres, c.ServiceCentre)
			}
		}

		peers = append(peers, p)
	}

	return peers, nil
}

func (db *Database) applyCreateSMSCPeer(ctx context.Context, p *SMSCPeer) (any, error) {
	peers, err := db.listSMSCPeers(ctx, db.runner(ctx))
	if err != nil {
		return nil, err
	}

	if err := ValidateSMSCPeers(append(peers, *p)); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSMSCPeerConflict, err)
	}

	row := smscPeerRow{ID: p.ID, DiameterIdentity: p.DiameterIdentity, Address: p.Address, Port: p.Port}

	if err := db.runner(ctx).Query(ctx, db.insertSMSCPeerStmt, row).Run(); err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, db.insertSMSCServiceCentres(ctx, p)
}

func (db *Database) applyUpdateSMSCPeer(ctx context.Context, p *SMSCPeer) (any, error) {
	peers, err := db.listSMSCPeers(ctx, db.runner(ctx))
	if err != nil {
		return nil, err
	}

	i := slices.IndexFunc(peers, func(other SMSCPeer) bool { return other.ID == p.ID })
	if i < 0 {
		return nil, ErrNotFound
	}

	peers[i] = *p

	if err := ValidateSMSCPeers(peers); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSMSCPeerConflict, err)
	}

	row := smscPeerRow{ID: p.ID, DiameterIdentity: p.DiameterIdentity, Address: p.Address, Port: p.Port}

	if err := db.runner(ctx).Query(ctx, db.updateSMSCPeerStmt, row).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if err := db.runner(ctx).Query(ctx, db.deleteSMSCServiceCentresStmt, smscServiceCentreRow{PeerID: p.ID}).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, db.insertSMSCServiceCentres(ctx, p)
}

func (db *Database) insertSMSCServiceCentres(ctx context.Context, p *SMSCPeer) error {
	for _, sc := range p.ServiceCentres {
		if err := db.runner(ctx).Query(ctx, db.insertSMSCServiceCentreStmt, smscServiceCentreRow{ServiceCentre: sc, PeerID: p.ID}).Run(); err != nil {
			if isUniqueNameError(err) {
				return fmt.Errorf("%w: another SMSC peer serves the service centre number +%s", ErrSMSCPeerConflict, sc)
			}

			return fmt.Errorf("query failed: %w", err)
		}
	}

	return nil
}

func (db *Database) applyDeleteSMSCPeer(ctx context.Context, p *stringPayload) (any, error) {
	peers, err := db.listSMSCPeers(ctx, db.runner(ctx))
	if err != nil {
		return nil, err
	}

	if !slices.ContainsFunc(peers, func(other SMSCPeer) bool { return other.ID == p.Value }) {
		return nil, ErrNotFound
	}

	if err := db.runner(ctx).Query(ctx, db.deleteSMSCServiceCentresStmt, smscServiceCentreRow{PeerID: p.Value}).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if err := db.runner(ctx).Query(ctx, db.deleteSMSCPeerStmt, smscPeerRow{ID: p.Value}).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func startSMSCPeersSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	querySummary := fmt.Sprintf("%s %s", operation, DiameterPeersTableName)

	return tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(DiameterPeersTableName),
		),
	)
}
