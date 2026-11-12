// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/ellanetworks/core/internal/diameternode"
	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const IMSPeersTableName = "ims_peers"

const (
	MaxIMSPeers          = 32
	MaxIMSPeerAddresses  = 16
	maxIMSPeerNameLength = 255
)

const (
	listIMSPeersSQL  = "SELECT id, name, originHost, realm, addresses FROM " + IMSPeersTableName + " ORDER BY name"
	insertIMSPeerSQL = "INSERT INTO " + IMSPeersTableName + " (id, name, originHost, realm, addresses) VALUES (?, ?, ?, ?, ?)"
	updateIMSPeerSQL = "UPDATE " + IMSPeersTableName + " SET originHost=?, realm=?, addresses=? WHERE id==?"
	deleteIMSPeerSQL = "DELETE FROM " + IMSPeersTableName + " WHERE id==?"
)

var ErrIMSPeerConflict = errors.New("allowed IMS peer conflict")

type IMSPeer struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	OriginHost string   `json:"originHost"`
	Realm      string   `json:"realm"`
	Addresses  []string `json:"addresses"`
}

type imsPeerRow struct {
	ID         string
	Name       string
	OriginHost *string
	Realm      *string
	Addresses  string
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

func (r imsPeerRow) peer() (IMSPeer, error) {
	p := IMSPeer{ID: r.ID, Name: r.Name}

	if r.OriginHost != nil {
		p.OriginHost = *r.OriginHost
	}

	if r.Realm != nil {
		p.Realm = *r.Realm
	}

	if err := json.Unmarshal([]byte(r.Addresses), &p.Addresses); err != nil {
		return IMSPeer{}, fmt.Errorf("decode the addresses of the allowed IMS peer %s: %w", r.Name, err)
	}

	return p, nil
}

func (p IMSPeer) row() (imsPeerRow, error) {
	b, err := json.Marshal(p.Addresses)
	if err != nil {
		return imsPeerRow{}, fmt.Errorf("encode the addresses of the allowed IMS peer %s: %w", p.Name, err)
	}

	return imsPeerRow{ID: p.ID, Name: p.Name, OriginHost: nullIfEmpty(p.OriginHost), Realm: nullIfEmpty(p.Realm), Addresses: string(b)}, nil
}

func NormalizeIMSPeerAddresses(addresses []string) ([]string, error) {
	if len(addresses) > MaxIMSPeerAddresses {
		return nil, fmt.Errorf("an allowed IMS peer can have at most %d addresses", MaxIMSPeerAddresses)
	}

	prefixes := make([]netip.Prefix, 0, len(addresses))

	for _, raw := range addresses {
		prefix, err := parseIMSPeerAddress(strings.TrimSpace(raw))
		if err != nil {
			return nil, err
		}

		prefixes = append(prefixes, prefix)
	}

	slices.SortFunc(prefixes, func(a, b netip.Prefix) int {
		if c := a.Addr().Compare(b.Addr()); c != 0 {
			return c
		}

		return a.Bits() - b.Bits()
	})

	prefixes = slices.Compact(prefixes)

	out := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		out = append(out, prefix.String())
	}

	return out, nil
}

func parseIMSPeerAddress(raw string) (netip.Prefix, error) {
	if strings.Contains(raw, "/") {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil || prefix.Addr().Zone() != "" {
			return netip.Prefix{}, fmt.Errorf("allowed IMS peer address %q is not an IP address or CIDR prefix", raw)
		}

		addr := prefix.Addr()
		bits := prefix.Bits()

		if addr.Is4In6() {
			if bits < 96 {
				return netip.Prefix{}, fmt.Errorf("allowed IMS peer address %q is not an IP address or CIDR prefix", raw)
			}

			addr, bits = addr.Unmap(), bits-96
		}

		return netip.PrefixFrom(addr, bits).Masked(), nil
	}

	addr, err := netip.ParseAddr(raw)
	if err != nil || addr.Zone() != "" {
		return netip.Prefix{}, fmt.Errorf("allowed IMS peer address %q is not an IP address or CIDR prefix", raw)
	}

	addr = addr.Unmap()

	return netip.PrefixFrom(addr, addr.BitLen()), nil
}

func (p IMSPeer) Rule() diameternode.IMSPeer {
	prefixes := make([]netip.Prefix, 0, len(p.Addresses))

	for _, raw := range p.Addresses {
		if prefix, err := parseIMSPeerAddress(raw); err == nil {
			prefixes = append(prefixes, prefix)
		}
	}

	return diameternode.IMSPeer{Name: p.Name, Host: p.OriginHost, Realm: p.Realm, Prefixes: prefixes}
}

func (p IMSPeer) Validate() error {
	if p.ID == "" {
		return errors.New("IMS peer ID is required")
	}

	if p.Name == "" || len(p.Name) > maxIMSPeerNameLength {
		return fmt.Errorf("IMS peer name must be 1 to %d bytes", maxIMSPeerNameLength)
	}

	if p.Name == "." || p.Name == ".." {
		return fmt.Errorf("IMS peer name cannot be %q", p.Name)
	}

	if p.OriginHost == "" && p.Realm == "" {
		return errors.New("IMS peer needs an Origin-Host, a realm, or both")
	}

	if p.OriginHost != "" && (!IsValidDiameterIdentity(p.OriginHost) || isIPLiteral(p.OriginHost) || strings.ToLower(p.OriginHost) != p.OriginHost) {
		return fmt.Errorf("IMS peer Origin-Host must be a lowercase fully qualified domain name, got %q", p.OriginHost)
	}

	if p.Realm != "" && (!IsValidDiameterIdentity(p.Realm) || isIPLiteral(p.Realm) || strings.ToLower(p.Realm) != p.Realm) {
		return fmt.Errorf("IMS peer realm must be a lowercase fully qualified domain name, got %q", p.Realm)
	}

	if len(p.Addresses) == 0 {
		return errors.New("an allowed IMS peer needs at least one source address or CIDR prefix")
	}

	normalized, err := NormalizeIMSPeerAddresses(p.Addresses)
	if err != nil {
		return err
	}

	if !slices.Equal(normalized, p.Addresses) {
		return fmt.Errorf("IMS peer addresses must be sorted, distinct CIDR prefixes, got %v", p.Addresses)
	}

	return nil
}

func (p IMSPeer) sameRule(other IMSPeer) bool {
	return p.OriginHost == other.OriginHost && p.Realm == other.Realm && slices.Equal(p.Addresses, other.Addresses)
}

func ValidateIMSPeers(peers []IMSPeer) error {
	if len(peers) > MaxIMSPeers {
		return fmt.Errorf("at most %d IMS peers can be configured", MaxIMSPeers)
	}

	for i, p := range peers {
		for _, other := range peers[:i] {
			if strings.EqualFold(p.Name, other.Name) {
				return fmt.Errorf("another allowed IMS peer is named %s", p.Name)
			}

			if p.sameRule(other) {
				return fmt.Errorf("the allowed IMS peer %s already has the same Origin-Host, realm and addresses", other.Name)
			}
		}
	}

	return nil
}

func (db *Database) ListIMSPeers(ctx context.Context) ([]IMSPeer, error) {
	ctx, span := startIMSPeersSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSPeersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSPeersTableName, "select").Inc()

	peers, err := db.listIMSPeers(ctx)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return peers, nil
}

func (db *Database) GetIMSPeer(ctx context.Context, name string) (*IMSPeer, error) {
	peers, err := db.ListIMSPeers(ctx)
	if err != nil {
		return nil, err
	}

	for _, p := range peers {
		if strings.EqualFold(p.Name, name) {
			return &p, nil
		}
	}

	return nil, ErrNotFound
}

func (db *Database) CreateIMSPeer(ctx context.Context, peer *IMSPeer) error {
	return db.writeIMSPeer(ctx, "INSERT", opCreateIMSPeer, peer)
}

func (db *Database) UpdateIMSPeer(ctx context.Context, peer *IMSPeer) error {
	return db.writeIMSPeer(ctx, "UPDATE", opUpdateIMSPeer, peer)
}

func (db *Database) DeleteIMSPeer(ctx context.Context, id string) error {
	ctx, span := startIMSPeersSpan(ctx, "DELETE")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSPeersTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSPeersTableName, "delete").Inc()

	if _, err := opDeleteIMSPeer.Invoke(ctx, db, &stringPayload{Value: id}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) writeIMSPeer(ctx context.Context, operation string, op *ChangesetOp[IMSPeer, struct{}], peer *IMSPeer) error {
	addresses, err := NormalizeIMSPeerAddresses(peer.Addresses)
	if err != nil {
		return err
	}

	peer.Addresses = addresses

	if err := peer.Validate(); err != nil {
		return err
	}

	ctx, span := startIMSPeersSpan(ctx, operation)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSPeersTableName, strings.ToLower(operation)))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSPeersTableName, strings.ToLower(operation)).Inc()

	if _, err := op.Invoke(ctx, db, peer); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) listIMSPeers(ctx context.Context) ([]IMSPeer, error) {
	if !db.appliedSchemaAtLeast(ctx, imsIntegrationsSchema) {
		return []IMSPeer{}, nil
	}

	rows, err := queryAll(ctx, db.readQuerier(ctx), listIMSPeersSQL, scanIMSPeerRow)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	peers := make([]IMSPeer, 0, len(rows))

	for _, row := range rows {
		p, err := row.peer()
		if err != nil {
			return nil, err
		}

		peers = append(peers, p)
	}

	return peers, nil
}

func (db *Database) applyCreateIMSPeer(ctx context.Context, p *IMSPeer) (any, error) {
	peers, err := db.listIMSPeers(ctx)
	if err != nil {
		return nil, err
	}

	if slices.ContainsFunc(peers, func(other IMSPeer) bool { return other.ID == p.ID || strings.EqualFold(other.Name, p.Name) }) {
		return nil, ErrAlreadyExists
	}

	if err := p.Validate(); err != nil {
		return nil, err
	}

	if err := ValidateIMSPeers(append(peers, *p)); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrIMSPeerConflict, err)
	}

	row, err := p.row()
	if err != nil {
		return nil, err
	}

	if _, err := db.runner(ctx).ExecContext(ctx, insertIMSPeerSQL, row.ID, row.Name, row.OriginHost, row.Realm, row.Addresses); err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateIMSPeer(ctx context.Context, p *IMSPeer) (any, error) {
	peers, err := db.listIMSPeers(ctx)
	if err != nil {
		return nil, err
	}

	i := slices.IndexFunc(peers, func(other IMSPeer) bool { return other.ID == p.ID })
	if i < 0 || peers[i].Name != p.Name {
		return nil, ErrNotFound
	}

	if err := p.Validate(); err != nil {
		return nil, err
	}

	peers[i] = *p

	if err := ValidateIMSPeers(peers); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrIMSPeerConflict, err)
	}

	row, err := p.row()
	if err != nil {
		return nil, err
	}

	if _, err := db.runner(ctx).ExecContext(ctx, updateIMSPeerSQL, row.OriginHost, row.Realm, row.Addresses, row.ID); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteIMSPeer(ctx context.Context, p *stringPayload) (any, error) {
	peers, err := db.listIMSPeers(ctx)
	if err != nil {
		return nil, err
	}

	if !slices.ContainsFunc(peers, func(other IMSPeer) bool { return other.ID == p.Value }) {
		return nil, ErrNotFound
	}

	if _, err := db.runner(ctx).ExecContext(ctx, deleteIMSPeerSQL, p.Value); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func startIMSPeersSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	querySummary := fmt.Sprintf("%s %s", operation, IMSPeersTableName)

	return tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(IMSPeersTableName),
		),
	)
}

func isIPLiteral(s string) bool {
	_, err := netip.ParseAddr(s)

	return err == nil
}

func scanIMSPeerRow(row scanner) (imsPeerRow, error) {
	var v imsPeerRow

	if err := row.Scan(&v.ID, &v.Name, &v.OriginHost, &v.Realm, &v.Addresses); err != nil {
		return imsPeerRow{}, err
	}

	return v, nil
}
