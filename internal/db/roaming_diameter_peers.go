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
	DiameterPeerRoleSMSC    = "smsc"
	DiameterPeerRoleRoaming = "roaming"
)

const (
	listDiameterPeersStmt         = "SELECT &diameterPeerRow.* FROM %s ORDER BY id"
	getDiameterPeerStmt           = "SELECT &diameterPeerRow.* FROM %s WHERE id==$diameterPeerRow.id"
	insertRoamingDiameterPeerStmt = "INSERT INTO %s (id, role, diameterIdentity, address, port) VALUES ($diameterPeerRow.id, 'roaming', $diameterPeerRow.diameterIdentity, $diameterPeerRow.address, $diameterPeerRow.port)"
	updateRoamingDiameterPeerStmt = "UPDATE %s SET diameterIdentity=$diameterPeerRow.diameterIdentity, address=$diameterPeerRow.address, port=$diameterPeerRow.port WHERE id==$diameterPeerRow.id AND role='roaming'"
	deleteRoamingDiameterPeerStmt = "DELETE FROM %s WHERE id==$diameterPeerRow.id AND role='roaming'"
)

type RoamingDiameterPeer struct {
	ID               string `json:"id"`
	DiameterIdentity string `json:"diameter_identity"`
	Address          string `json:"address"`
	Port             int    `json:"port"`
}

type diameterPeerRow struct {
	ID               string `db:"id"`
	Role             string `db:"role"`
	DiameterIdentity string `db:"diameterIdentity"`
	Address          string `db:"address"`
	Port             int    `db:"port"`
}

func (p RoamingDiameterPeer) Validate() error {
	if p.ID == "" {
		return errors.New("roaming Diameter peer ID is required")
	}

	if p.DiameterIdentity != "" && !IsValidDiameterIdentity(p.DiameterIdentity) {
		return fmt.Errorf("roaming Diameter identity must be a fully qualified domain name, got %q", p.DiameterIdentity)
	}

	addr, err := netip.ParseAddr(p.Address)
	if err != nil || addr.Zone() != "" || addr.IsUnspecified() || addr.Unmap().String() != p.Address {
		return fmt.Errorf("roaming Diameter peer address must be an IPv4 or IPv6 address in canonical form, got %q", p.Address)
	}

	if p.Port < 1 || p.Port > 65535 {
		return fmt.Errorf("roaming Diameter peer port must be between 1 and 65535, got %d", p.Port)
	}

	return nil
}

func validateDiameterPeerEndpoints(peers []diameterPeerRow) error {
	for i, p := range peers {
		for _, other := range peers[:i] {
			if p.ID == other.ID {
				return fmt.Errorf("another Diameter peer has the ID %s", p.ID)
			}

			if p.DiameterIdentity != "" && strings.EqualFold(p.DiameterIdentity, other.DiameterIdentity) {
				return fmt.Errorf("another Diameter peer has the Diameter identity %s", p.DiameterIdentity)
			}

			if p.Address == other.Address && p.Port == other.Port {
				return fmt.Errorf("another Diameter peer has the address %s", net.JoinHostPort(p.Address, strconv.Itoa(p.Port)))
			}

			if p.DiameterIdentity == "" && other.DiameterIdentity == "" && p.Address == other.Address {
				return fmt.Errorf("another Diameter peer without a Diameter identity has the address %s", p.Address)
			}
		}
	}

	return nil
}

func (db *Database) listDiameterPeers(ctx context.Context, runner *sqlair.DB) ([]diameterPeerRow, error) {
	var rows []diameterPeerRow

	if err := runner.Query(ctx, db.listDiameterPeersStmt).GetAll(&rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return rows, nil
}

func (db *Database) checkDiameterPeerEndpoint(ctx context.Context, candidate diameterPeerRow) error {
	peers, err := db.listDiameterPeers(ctx, db.runner(ctx))
	if err != nil {
		return err
	}

	peers = slices.DeleteFunc(peers, func(p diameterPeerRow) bool { return p.ID == candidate.ID })

	return validateDiameterPeerEndpoints(append(peers, candidate))
}

func (db *Database) checkSMSCNodeEndpoints(ctx context.Context, s *SMSC) error {
	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil
	}

	peers, err := db.listDiameterPeers(ctx, db.runner(ctx))
	if err != nil {
		return err
	}

	peers = slices.DeleteFunc(peers, func(p diameterPeerRow) bool { return p.Role == DiameterPeerRoleSMSC })

	for _, n := range s.Nodes {
		peers = append(peers, diameterPeerRow{ID: n.ID, Role: DiameterPeerRoleSMSC, DiameterIdentity: n.DiameterIdentity, Address: n.Address, Port: n.Port})
	}

	return validateDiameterPeerEndpoints(peers)
}

func (db *Database) ListRoamingDiameterPeers(ctx context.Context) ([]RoamingDiameterPeer, error) {
	ctx, span := startDiameterPeersSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(DiameterPeersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(DiameterPeersTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, nil
	}

	rows, err := db.listDiameterPeers(ctx, db.conn())
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	peers := make([]RoamingDiameterPeer, 0, len(rows))

	for _, r := range rows {
		if r.Role == DiameterPeerRoleRoaming {
			peers = append(peers, RoamingDiameterPeer{ID: r.ID, DiameterIdentity: r.DiameterIdentity, Address: r.Address, Port: r.Port})
		}
	}

	return peers, nil
}

func (db *Database) GetRoamingDiameterPeer(ctx context.Context, id string) (*RoamingDiameterPeer, error) {
	peers, err := db.ListRoamingDiameterPeers(ctx)
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

func (db *Database) CreateRoamingDiameterPeer(ctx context.Context, peer *RoamingDiameterPeer) error {
	return db.writeRoamingDiameterPeer(ctx, "INSERT", opCreateRoamingDiameterPeer, peer)
}

func (db *Database) UpdateRoamingDiameterPeer(ctx context.Context, peer *RoamingDiameterPeer) error {
	return db.writeRoamingDiameterPeer(ctx, "UPDATE", opUpdateRoamingDiameterPeer, peer)
}

func (db *Database) writeRoamingDiameterPeer(ctx context.Context, operation string, op *ChangesetOp[RoamingDiameterPeer, struct{}], peer *RoamingDiameterPeer) error {
	if err := peer.Validate(); err != nil {
		return err
	}

	ctx, span := startDiameterPeersSpan(ctx, operation)
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

func (db *Database) DeleteRoamingDiameterPeer(ctx context.Context, id string) error {
	ctx, span := startDiameterPeersSpan(ctx, "DELETE")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(DiameterPeersTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(DiameterPeersTableName, "delete").Inc()

	if _, err := opDeleteRoamingDiameterPeer.Invoke(ctx, db, &stringPayload{Value: id}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (p RoamingDiameterPeer) row() diameterPeerRow {
	return diameterPeerRow{ID: p.ID, Role: DiameterPeerRoleRoaming, DiameterIdentity: p.DiameterIdentity, Address: p.Address, Port: p.Port}
}

func (db *Database) applyCreateRoamingDiameterPeer(ctx context.Context, p *RoamingDiameterPeer) (any, error) {
	if err := db.checkDiameterPeerEndpoint(ctx, p.row()); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRoamingConflict, err)
	}

	if err := db.runner(ctx).Query(ctx, db.insertRoamingDiameterPeerStmt, p.row()).Run(); err != nil {
		if isUniqueNameError(err) || isPrimaryKeyError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateRoamingDiameterPeer(ctx context.Context, p *RoamingDiameterPeer) (any, error) {
	existing := diameterPeerRow{ID: p.ID}

	if err := db.runner(ctx).Query(ctx, db.getDiameterPeerStmt, existing).Get(&existing); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	if existing.Role != DiameterPeerRoleRoaming {
		return nil, ErrNotFound
	}

	if err := db.checkDiameterPeerEndpoint(ctx, p.row()); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRoamingConflict, err)
	}

	if err := db.runner(ctx).Query(ctx, db.updateRoamingDiameterPeerStmt, p.row()).Run(); err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteRoamingDiameterPeer(ctx context.Context, p *stringPayload) (any, error) {
	var outcome sqlair.Outcome

	if err := db.runner(ctx).Query(ctx, db.deleteRoamingDiameterPeerStmt, diameterPeerRow{ID: p.Value}).Get(&outcome); err != nil {
		if isForeignKeyRestrictError(err) {
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

func startDiameterPeersSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
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
