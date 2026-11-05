// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/canonical/sqlair"
)

const (
	listLegacySMSCPeersStmt            = "SELECT &smscPeerRow.* FROM %s WHERE role='smsc' ORDER BY id"
	listLegacySMSCServiceCentresStmt   = "SELECT &legacySMSCServiceCentreRow.* FROM %s ORDER BY serviceCentre"
	insertLegacySMSCPeerStmt           = "INSERT INTO %s (id, role, diameterIdentity, address, port) VALUES ($smscPeerRow.id, 'smsc', $smscPeerRow.diameterIdentity, $smscPeerRow.address, $smscPeerRow.port)"
	updateLegacySMSCPeerStmt           = "UPDATE %s SET diameterIdentity=$smscPeerRow.diameterIdentity, address=$smscPeerRow.address, port=$smscPeerRow.port WHERE id==$smscPeerRow.id AND role='smsc'"
	deleteLegacySMSCPeerStmt           = "DELETE FROM %s WHERE id==$smscPeerRow.id AND role='smsc'"
	insertLegacySMSCServiceCentreStmt  = "INSERT INTO %s (serviceCentre, peerId) VALUES ($legacySMSCServiceCentreRow.serviceCentre, $legacySMSCServiceCentreRow.peerId)"
	deleteLegacySMSCServiceCentresStmt = "DELETE FROM %s WHERE peerId==$legacySMSCServiceCentreRow.peerId"
)

type legacySMSCPeer struct {
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

type legacySMSCServiceCentreRow struct {
	ServiceCentre string `db:"serviceCentre"`
	PeerID        string `db:"peerId"`
}

func (p legacySMSCPeer) smsc(name string) SMSC {
	return SMSC{
		ID:             p.ID,
		Name:           name,
		ServiceCentres: p.ServiceCentres,
		Nodes:          []SMSCNode{{ID: p.ID, DiameterIdentity: p.DiameterIdentity, Address: p.Address, Port: p.Port}},
	}
}

func (db *Database) listLegacySMSCPeers(ctx context.Context, runner *sqlair.DB) ([]legacySMSCPeer, error) {
	var rows []smscPeerRow

	if err := runner.Query(ctx, db.listLegacySMSCPeersStmt).GetAll(&rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	var centres []legacySMSCServiceCentreRow

	if err := runner.Query(ctx, db.listLegacySMSCServiceCentresStmt).GetAll(&centres); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	peers := make([]legacySMSCPeer, 0, len(rows))

	for _, r := range rows {
		p := legacySMSCPeer{ID: r.ID, DiameterIdentity: r.DiameterIdentity, Address: r.Address, Port: r.Port, ServiceCentres: []string{}}

		for _, c := range centres {
			if c.PeerID == r.ID {
				p.ServiceCentres = append(p.ServiceCentres, c.ServiceCentre)
			}
		}

		peers = append(peers, p)
	}

	return peers, nil
}

func legacySMSCs(peers []legacySMSCPeer) []SMSC {
	smscs := make([]SMSC, 0, len(peers))
	names := make([]string, 0, len(peers))

	for _, p := range peers {
		name := legacySMSCName(p.DiameterIdentity, p.Address, names)
		names = append(names, name)
		smscs = append(smscs, p.smsc(name))
	}

	return smscs
}

func (db *Database) legacySMSCPeerWritable(ctx context.Context) error {
	if db.appliedSchemaAtLeast(ctx, smscNodesSchema) {
		return fmt.Errorf("%w: SMSC peers are now SMSC nodes", ErrRetiredOperation)
	}

	return nil
}

func validateLegacySMSCPeers(peers []legacySMSCPeer) error {
	smscs := legacySMSCs(peers)

	for _, s := range smscs {
		if err := s.Validate(); err != nil {
			return err
		}
	}

	return ValidateSMSCs(smscs)
}

func (db *Database) applyCreateSMSCPeer(ctx context.Context, p *legacySMSCPeer) (any, error) {
	if err := db.legacySMSCPeerWritable(ctx); err != nil {
		return nil, err
	}

	peers, err := db.listLegacySMSCPeers(ctx, db.runner(ctx))
	if err != nil {
		return nil, err
	}

	if err := validateLegacySMSCPeers(append(peers, *p)); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSMSCConflict, err)
	}

	row := smscPeerRow{ID: p.ID, DiameterIdentity: p.DiameterIdentity, Address: p.Address, Port: p.Port}

	if err := db.runner(ctx).Query(ctx, db.insertLegacySMSCPeerStmt, row).Run(); err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, db.insertLegacySMSCServiceCentres(ctx, p)
}

func (db *Database) applyUpdateSMSCPeer(ctx context.Context, p *legacySMSCPeer) (any, error) {
	if err := db.legacySMSCPeerWritable(ctx); err != nil {
		return nil, err
	}

	peers, err := db.listLegacySMSCPeers(ctx, db.runner(ctx))
	if err != nil {
		return nil, err
	}

	i := slices.IndexFunc(peers, func(other legacySMSCPeer) bool { return other.ID == p.ID })
	if i < 0 {
		return nil, ErrNotFound
	}

	peers[i] = *p

	if err := validateLegacySMSCPeers(peers); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSMSCConflict, err)
	}

	row := smscPeerRow{ID: p.ID, DiameterIdentity: p.DiameterIdentity, Address: p.Address, Port: p.Port}

	if err := db.runner(ctx).Query(ctx, db.updateLegacySMSCPeerStmt, row).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if err := db.runner(ctx).Query(ctx, db.deleteLegacySMSCServiceCentresStmt, legacySMSCServiceCentreRow{PeerID: p.ID}).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, db.insertLegacySMSCServiceCentres(ctx, p)
}

func (db *Database) insertLegacySMSCServiceCentres(ctx context.Context, p *legacySMSCPeer) error {
	for _, sc := range p.ServiceCentres {
		if err := db.runner(ctx).Query(ctx, db.insertLegacySMSCServiceCentreStmt, legacySMSCServiceCentreRow{ServiceCentre: sc, PeerID: p.ID}).Run(); err != nil {
			if isUniqueNameError(err) {
				return fmt.Errorf("%w: another SMSC serves the service centre number +%s", ErrSMSCConflict, sc)
			}

			return fmt.Errorf("query failed: %w", err)
		}
	}

	return nil
}

func (db *Database) applyDeleteSMSCPeer(ctx context.Context, p *stringPayload) (any, error) {
	if err := db.legacySMSCPeerWritable(ctx); err != nil {
		return nil, err
	}

	peers, err := db.listLegacySMSCPeers(ctx, db.runner(ctx))
	if err != nil {
		return nil, err
	}

	if !slices.ContainsFunc(peers, func(other legacySMSCPeer) bool { return other.ID == p.Value }) {
		return nil, ErrNotFound
	}

	if err := db.runner(ctx).Query(ctx, db.deleteLegacySMSCServiceCentresStmt, legacySMSCServiceCentreRow{PeerID: p.Value}).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if err := db.runner(ctx).Query(ctx, db.deleteLegacySMSCPeerStmt, smscPeerRow{ID: p.Value}).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func legacySMSCName(identity, address string, taken []string) string {
	base := cmp.Or(identity, address)
	name := base

	for i := 2; slices.ContainsFunc(taken, func(t string) bool { return strings.EqualFold(t, name) }); i++ {
		name = fmt.Sprintf("%s (%d)", base, i)
	}

	return name
}
