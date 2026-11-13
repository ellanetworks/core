// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/diameter"
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
	listSMSCsSQL                = "SELECT id, name FROM " + SMSCsTableName + " ORDER BY id"
	listSMSCNodesSQL            = "SELECT id, diameterIdentity, address, port, COALESCE(smscId, '') FROM " + DiameterPeersTableName + " WHERE role='smsc' ORDER BY id"
	listSMSCServiceCentresSQL   = "SELECT serviceCentre, smscId FROM " + SMSCServiceCentresTableName + " ORDER BY serviceCentre"
	insertSMSCSQL               = "INSERT INTO " + SMSCsTableName + " (id, name) VALUES (?, ?)"
	deleteSMSCSQL               = "DELETE FROM " + SMSCsTableName + " WHERE id==?"
	insertSMSCNodeSQL           = "INSERT INTO " + DiameterPeersTableName + " (id, role, diameterIdentity, address, port, smscId) VALUES (?, 'smsc', ?, ?, ?, ?)"
	deleteSMSCNodesSQL          = "DELETE FROM " + DiameterPeersTableName + " WHERE smscId==? AND role='smsc'"
	insertSMSCServiceCentreSQL  = "INSERT INTO " + SMSCServiceCentresTableName + " (serviceCentre, smscId) VALUES (?, ?)"
	deleteSMSCServiceCentresSQL = "DELETE FROM " + SMSCServiceCentresTableName + " WHERE smscId==?"
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
	ID   string
	Name string
}

type smscNodeRow struct {
	ID               string
	DiameterIdentity string
	Address          string
	Port             int
	SMSCID           string
}

type smscServiceCentreRow struct {
	ServiceCentre string
	SMSCID        string
}

func (s SMSC) Validate() error {
	if s.ID == "" {
		return errors.New("SMSC ID is required")
	}

	if s.Name == "" || len(s.Name) > maxSMSCNameLength {
		return fmt.Errorf("SMSC name must be 1 to %d bytes", maxSMSCNameLength)
	}

	if s.Name == "." || s.Name == ".." {
		return fmt.Errorf("SMSC name cannot be %q", s.Name)
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

	if !IsCanonicalNodeAddress(n.Address) {
		return fmt.Errorf("SMSC node address must be an IPv4 or IPv6 address or a lowercase host name, got %q", n.Address)
	}

	if n.Port < 1 || n.Port > 65535 {
		return fmt.Errorf("SMSC node port must be between 1 and 65535, got %d", n.Port)
	}

	return nil
}

func IsCanonicalNodeAddress(address string) bool {
	if addr, err := netip.ParseAddr(address); err == nil {
		return addr.Zone() == "" && !addr.IsUnspecified() && addr.Unmap().String() == address
	}

	return diameter.IsValidHostname(address) && strings.ToLower(address) == address
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

func (db *Database) ListSMSCs(ctx context.Context) (_ []SMSC, err error) {
	ctx, finish := startDBQuery(ctx, SMSCsTableName, "SELECT")
	defer func() { finish(err) }()

	smscs, err := db.listSMSCs(ctx, db.reader())
	if err != nil {
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
		if strings.EqualFold(s.Name, name) {
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

func (db *Database) writeSMSC(ctx context.Context, operation string, op *ChangesetOp[SMSC, struct{}], smsc *SMSC) (err error) {
	if err := smsc.Validate(); err != nil {
		return err
	}

	ctx, finish := startDBQuery(ctx, SMSCsTableName, operation)
	defer func() { finish(err) }()

	if _, err := op.Invoke(ctx, db, smsc); err != nil {
		return err
	}

	return nil
}

func (db *Database) DeleteSMSC(ctx context.Context, id string) (err error) {
	ctx, finish := startDBQuery(ctx, SMSCsTableName, "DELETE")
	defer func() { finish(err) }()

	if _, err := opDeleteSMSC.Invoke(ctx, db, &stringPayload{Value: id}); err != nil {
		return err
	}

	return nil
}

func (db *Database) listSMSCs(ctx context.Context, q querier) ([]SMSC, error) {
	if !db.appliedSchemaAtLeast(ctx, smsSchema) {
		return nil, nil
	}

	if !db.appliedSchemaAtLeast(ctx, smscNodesSchema) {
		peers, err := db.listLegacySMSCPeers(ctx, q)
		if err != nil {
			return nil, err
		}

		return legacySMSCs(peers), nil
	}

	rows, err := queryAll(ctx, q, listSMSCsSQL, scanSMSCRow)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	nodes, err := queryAll(ctx, q, listSMSCNodesSQL, scanSMSCNodeRow)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	centres, err := queryAll(ctx, q, listSMSCServiceCentresSQL, scanSMSCServiceCentreRow)
	if err != nil {
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

	if err := db.checkSMSCNodeEndpoints(ctx, s); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSMSCConflict, err)
	}

	if _, err := db.runner(ctx).ExecContext(ctx, insertSMSCSQL, s.ID, s.Name); err != nil {
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

	if err := db.checkSMSCNodeEndpoints(ctx, s); err != nil {
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

	if _, err := db.runner(ctx).ExecContext(ctx, deleteSMSCSQL, p.Value); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) deleteSMSCChildren(ctx context.Context, id string) error {
	if _, err := db.runner(ctx).ExecContext(ctx, deleteSMSCServiceCentresSQL, id); err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	if _, err := db.runner(ctx).ExecContext(ctx, deleteSMSCNodesSQL, id); err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) insertSMSCChildren(ctx context.Context, s *SMSC) error {
	for _, n := range s.Nodes {
		row := smscNodeRow{ID: n.ID, DiameterIdentity: n.DiameterIdentity, Address: n.Address, Port: n.Port, SMSCID: s.ID}

		if _, err := db.runner(ctx).ExecContext(ctx, insertSMSCNodeSQL, row.ID, row.DiameterIdentity, row.Address, row.Port, row.SMSCID); err != nil {
			if isUniqueNameError(err) {
				return fmt.Errorf("%w: SMSC node %s or its Diameter identity is already in use", ErrSMSCConflict, n.ID)
			}

			return fmt.Errorf("query failed: %w", err)
		}
	}

	for _, sc := range s.ServiceCentres {
		if _, err := db.runner(ctx).ExecContext(ctx, insertSMSCServiceCentreSQL, sc, s.ID); err != nil {
			if isUniqueNameError(err) {
				return fmt.Errorf("%w: another SMSC serves the service centre number +%s", ErrSMSCConflict, sc)
			}

			return fmt.Errorf("query failed: %w", err)
		}
	}

	return nil
}

func scanSMSCRow(row scanner) (smscRow, error) {
	var v smscRow

	if err := row.Scan(&v.ID, &v.Name); err != nil {
		return smscRow{}, err
	}

	return v, nil
}

func scanSMSCNodeRow(row scanner) (smscNodeRow, error) {
	var v smscNodeRow

	if err := row.Scan(&v.ID, &v.DiameterIdentity, &v.Address, &v.Port, &v.SMSCID); err != nil {
		return smscNodeRow{}, err
	}

	return v, nil
}

func scanSMSCServiceCentreRow(row scanner) (smscServiceCentreRow, error) {
	var v smscServiceCentreRow

	if err := row.Scan(&v.ServiceCentre, &v.SMSCID); err != nil {
		return smscServiceCentreRow{}, err
	}

	return v, nil
}
