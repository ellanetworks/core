// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
)

const (
	listLegacySMSCPeersSQL            = "SELECT id, diameterIdentity, address, port FROM " + DiameterPeersTableName + " WHERE role='smsc' ORDER BY id"
	listLegacySMSCServiceCentresSQL   = "SELECT serviceCentre, peerId FROM " + SMSCServiceCentresTableName + " ORDER BY serviceCentre"
	insertLegacySMSCPeerSQL           = "INSERT INTO " + DiameterPeersTableName + " (id, role, diameterIdentity, address, port) VALUES (?, 'smsc', ?, ?, ?)"
	updateLegacySMSCPeerSQL           = "UPDATE " + DiameterPeersTableName + " SET diameterIdentity=?, address=?, port=? WHERE id==? AND role='smsc'"
	deleteLegacySMSCPeerSQL           = "DELETE FROM " + DiameterPeersTableName + " WHERE id==? AND role='smsc'"
	insertLegacySMSCServiceCentreSQL  = "INSERT INTO " + SMSCServiceCentresTableName + " (serviceCentre, peerId) VALUES (?, ?)"
	deleteLegacySMSCServiceCentresSQL = "DELETE FROM " + SMSCServiceCentresTableName + " WHERE peerId==?"
)

type legacySMSCPeer struct {
	ID               string   `json:"id"`
	DiameterIdentity string   `json:"diameter_identity"`
	Address          string   `json:"address"`
	Port             int      `json:"port"`
	ServiceCentres   []string `json:"service_centres"`
}

type smscPeerRow struct {
	ID               string
	DiameterIdentity string
	Address          string
	Port             int
}

type legacySMSCServiceCentreRow struct {
	ServiceCentre string
	PeerID        string
}

func (p legacySMSCPeer) smsc(name string) SMSC {
	return SMSC{
		ID:             p.ID,
		Name:           name,
		ServiceCentres: p.ServiceCentres,
		Nodes:          []SMSCNode{{ID: p.ID, DiameterIdentity: p.DiameterIdentity, Address: p.Address, Port: p.Port}},
	}
}

func (db *Database) listLegacySMSCPeers(ctx context.Context, q querier) ([]legacySMSCPeer, error) {
	rows, err := queryAll(ctx, q, listLegacySMSCPeersSQL, scanSMSCPeerRow)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	centres, err := queryAll(ctx, q, listLegacySMSCServiceCentresSQL, scanLegacySMSCServiceCentreRow)
	if err != nil {
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

	if _, err := db.runner(ctx).ExecContext(ctx, insertLegacySMSCPeerSQL, row.ID, row.DiameterIdentity, row.Address, row.Port); err != nil {
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

	if _, err := db.runner(ctx).ExecContext(ctx, updateLegacySMSCPeerSQL, row.DiameterIdentity, row.Address, row.Port, row.ID); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if _, err := db.runner(ctx).ExecContext(ctx, deleteLegacySMSCServiceCentresSQL, p.ID); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, db.insertLegacySMSCServiceCentres(ctx, p)
}

func (db *Database) insertLegacySMSCServiceCentres(ctx context.Context, p *legacySMSCPeer) error {
	for _, sc := range p.ServiceCentres {
		if _, err := db.runner(ctx).ExecContext(ctx, insertLegacySMSCServiceCentreSQL, sc, p.ID); err != nil {
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

	if _, err := db.runner(ctx).ExecContext(ctx, deleteLegacySMSCServiceCentresSQL, p.Value); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if _, err := db.runner(ctx).ExecContext(ctx, deleteLegacySMSCPeerSQL, p.Value); err != nil {
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

func scanSMSCPeerRow(row scanner) (smscPeerRow, error) {
	var v smscPeerRow

	if err := row.Scan(&v.ID, &v.DiameterIdentity, &v.Address, &v.Port); err != nil {
		return smscPeerRow{}, err
	}

	return v, nil
}

func scanLegacySMSCServiceCentreRow(row scanner) (legacySMSCServiceCentreRow, error) {
	var v legacySMSCServiceCentreRow

	if err := row.Scan(&v.ServiceCentre, &v.PeerID); err != nil {
		return legacySMSCServiceCentreRow{}, err
	}

	return v, nil
}
