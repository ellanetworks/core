// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/internal/hss/psi"
	"github.com/google/uuid"
)

const (
	IMSApplicationServersTableName      = "ims_application_servers"
	IMSPublicServiceIdentitiesTableName = "ims_public_service_identities"
)

const imsServicesSchema = 25

const (
	listIMSApplicationServersSQL  = "SELECT id, name, serverName FROM " + IMSApplicationServersTableName + " ORDER BY name"
	getIMSApplicationServerSQL    = "SELECT id, name, serverName FROM " + IMSApplicationServersTableName + " WHERE name==?"
	insertIMSApplicationServerSQL = "INSERT INTO " + IMSApplicationServersTableName + " (id, name, serverName) VALUES (?, ?, ?)"
	updateIMSApplicationServerSQL = "UPDATE " + IMSApplicationServersTableName + " SET serverName=? WHERE id==?"
	deleteIMSApplicationServerSQL = "DELETE FROM " + IMSApplicationServersTableName + " WHERE name==?"
	listIMSPSIsSQL                = "SELECT identity, applicationServerID FROM " + IMSPublicServiceIdentitiesTableName + " ORDER BY identity"
	listIMSPSIsByASSQL            = "SELECT identity, applicationServerID FROM " + IMSPublicServiceIdentitiesTableName + " WHERE applicationServerID==? ORDER BY identity"
	insertIMSPSISQL               = "INSERT INTO " + IMSPublicServiceIdentitiesTableName + " (identity, applicationServerID) VALUES (?, ?)"
	deleteIMSPSIsByASSQL          = "DELETE FROM " + IMSPublicServiceIdentitiesTableName + " WHERE applicationServerID==?"
)

const imsMCPTTUserPrefix = "mcptt-"

var ErrPSIConflict = errors.New("public service identity conflict")

type IMSApplicationServer struct {
	ID                      string   `json:"id"`
	Name                    string   `json:"name"`
	ServerName              string   `json:"server_name"`
	PublicServiceIdentities []string `json:"public_service_identities"`
}

type imsApplicationServerRow struct {
	ID         string
	Name       string
	ServerName string
}

type imsPSIRow struct {
	Identity            string
	ApplicationServerID string
}

func (as IMSApplicationServer) Validate() error {
	_, err := as.canonical()

	return err
}

func (as IMSApplicationServer) canonical() (IMSApplicationServer, error) {
	if as.Name == "" {
		return as, errors.New("application server name is required")
	}

	serverName, err := psi.CanonicalServerName(as.ServerName)
	if err != nil {
		return as, fmt.Errorf("application server name: %w", err)
	}

	if err := cx.ValidateURI("application server name", serverName); err != nil {
		return as, err
	}

	identities := make([]string, 0, len(as.PublicServiceIdentities))

	for _, identity := range as.PublicServiceIdentities {
		canonical, err := psi.Canonical(identity)
		if err != nil {
			return as, fmt.Errorf("public service identity: %w", err)
		}

		if err := cx.ValidateURI("public service identity", canonical); err != nil {
			return as, err
		}

		if slices.Contains(identities, canonical) {
			return as, fmt.Errorf("public service identity %s is listed twice", canonical)
		}

		identities = append(identities, canonical)
	}

	as.ServerName, as.PublicServiceIdentities = serverName, identities

	return as, nil
}

func (db *Database) ListIMSApplicationServers(ctx context.Context) (_ []IMSApplicationServer, err error) {
	ctx, finish := startDBQuery(ctx, IMSApplicationServersTableName, "SELECT")
	defer func() { finish(err) }()

	if !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return nil, nil
	}

	rows, err := queryAll(ctx, db.reader(), listIMSApplicationServersSQL, scanIMSApplicationServerRow)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	psis, err := queryAll(ctx, db.reader(), listIMSPSIsSQL, scanIMSPSIRow)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	servers := make([]IMSApplicationServer, 0, len(rows))

	for _, r := range rows {
		as := IMSApplicationServer{ID: r.ID, Name: r.Name, ServerName: r.ServerName, PublicServiceIdentities: []string{}}

		for _, psi := range psis {
			if psi.ApplicationServerID == r.ID {
				as.PublicServiceIdentities = append(as.PublicServiceIdentities, psi.Identity)
			}
		}

		servers = append(servers, as)
	}

	return servers, nil
}

func (db *Database) GetIMSApplicationServer(ctx context.Context, name string) (_ *IMSApplicationServer, err error) {
	ctx, finish := startDBQuery(ctx, IMSApplicationServersTableName, "SELECT")
	defer func() { finish(err) }()

	if !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return nil, ErrNotFound
	}

	as, err := db.getIMSApplicationServer(ctx, db.reader(), name)
	if err != nil {
		return nil, err
	}

	return as, nil
}

func (db *Database) getIMSApplicationServer(ctx context.Context, q querier, name string) (*IMSApplicationServer, error) {
	row, err := scanIMSApplicationServerRow(q.QueryRowContext(ctx, getIMSApplicationServerSQL, name))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	psis, err := queryAll(ctx, q, listIMSPSIsByASSQL, scanIMSPSIRow, row.ID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	as := &IMSApplicationServer{ID: row.ID, Name: row.Name, ServerName: row.ServerName, PublicServiceIdentities: make([]string, 0, len(psis))}

	for _, psi := range psis {
		as.PublicServiceIdentities = append(as.PublicServiceIdentities, psi.Identity)
	}

	return as, nil
}

func (db *Database) CreateIMSApplicationServer(ctx context.Context, as *IMSApplicationServer) error {
	if as.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate application server id: %w", err)
		}

		as.ID = id.String()
	}

	return db.writeIMSApplicationServer(ctx, "INSERT", opCreateIMSApplicationServer, as)
}

func (db *Database) UpdateIMSApplicationServer(ctx context.Context, as *IMSApplicationServer) error {
	return db.writeIMSApplicationServer(ctx, "UPDATE", opUpdateIMSApplicationServer, as)
}

func (db *Database) writeIMSApplicationServer(ctx context.Context, operation string, op *ChangesetOp[IMSApplicationServer, struct{}], as *IMSApplicationServer) (err error) {
	canonical, err := as.canonical()
	if err != nil {
		return err
	}

	*as = canonical

	ctx, finish := startDBQuery(ctx, IMSApplicationServersTableName, operation)
	defer func() { finish(err) }()

	if _, err := op.Invoke(ctx, db, as); err != nil {
		return err
	}

	return nil
}

func (db *Database) DeleteIMSApplicationServer(ctx context.Context, name string) (err error) {
	ctx, finish := startDBQuery(ctx, IMSApplicationServersTableName, "DELETE")
	defer func() { finish(err) }()

	if _, err := opDeleteIMSApplicationServer.Invoke(ctx, db, &stringPayload{Value: name}); err != nil {
		return err
	}

	return nil
}

func (db *Database) applyCreateIMSApplicationServer(ctx context.Context, as *IMSApplicationServer) (any, error) {
	if err := db.checkPSIsAvailable(ctx, as.ID, as.PublicServiceIdentities); err != nil {
		return nil, err
	}

	row := imsApplicationServerRow{ID: as.ID, Name: as.Name, ServerName: as.ServerName}

	if _, err := db.runner(ctx).ExecContext(ctx, insertIMSApplicationServerSQL, row.ID, row.Name, row.ServerName); err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, db.insertIMSPSIs(ctx, as.ID, as.PublicServiceIdentities)
}

func (db *Database) applyUpdateIMSApplicationServer(ctx context.Context, as *IMSApplicationServer) (any, error) {
	existing, err := db.getIMSApplicationServer(ctx, db.runner(ctx), as.Name)
	if err != nil {
		return nil, err
	}

	if err := db.checkPSIsAvailable(ctx, existing.ID, as.PublicServiceIdentities); err != nil {
		return nil, err
	}

	row := imsApplicationServerRow{ID: existing.ID, ServerName: as.ServerName}

	if _, err := db.runner(ctx).ExecContext(ctx, updateIMSApplicationServerSQL, row.ServerName, row.ID); err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	if _, err := db.runner(ctx).ExecContext(ctx, deleteIMSPSIsByASSQL, existing.ID); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, db.insertIMSPSIs(ctx, existing.ID, as.PublicServiceIdentities)
}

func (db *Database) checkPSIsAvailable(ctx context.Context, applicationServerID string, identities []string) error {
	stored, err := queryAll(ctx, db.runner(ctx), listIMSPSIsSQL, scanIMSPSIRow)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	domains, err := db.imsHomeDomains(ctx)
	if err != nil {
		return err
	}

	accepted := domains.Accepted()

	for _, identity := range identities {
		for _, domain := range accepted {
			if collidesWithSubscriberIdentity(identity, domain) {
				return fmt.Errorf("%w: %s has the form of a subscriber identity in %s", ErrPSIConflict, identity, domain)
			}
		}

		for _, other := range stored {
			if other.ApplicationServerID == applicationServerID {
				continue
			}

			if other.Identity == identity {
				return fmt.Errorf("%w: %s is hosted by another application server", ErrPSIConflict, identity)
			}

			if psi.Overlap(identity, other.Identity) {
				return fmt.Errorf("%w: %s overlaps %s on another application server", ErrPSIConflict, identity, other.Identity)
			}
		}

		if err := db.checkPSIFreeOfSubscribers(ctx, identity, accepted); err != nil {
			return err
		}
	}

	return nil
}

func (db *Database) checkPSIFreeOfSubscribers(ctx context.Context, identity string, imsDomains []string) error {
	inIMSDomain := strings.HasPrefix(identity, "sip:") && slices.Contains(imsDomains, psi.Host(identity))
	if !strings.HasPrefix(identity, "tel:+") && !inIMSDomain {
		return nil
	}

	if !psi.Wildcarded(identity) {
		msisdn, ok := strings.CutPrefix(identity, "tel:+")
		if inIMSDomain {
			user := psi.User(identity)
			if msisdn, ok = strings.CutPrefix(user, "+"); !ok {
				msisdn, ok = strings.CutPrefix(user, imsMCPTTUserPrefix)
			}
		}

		if !ok || strings.ContainsRune(msisdn, ';') {
			return nil
		}

		holder, err := scanSubscriber(db.runner(ctx).QueryRowContext(ctx, getSubscriberByMSISDNSQL, msisdn))
		if err == nil {
			return fmt.Errorf("%w: %s is the public user identity of subscriber %s", ErrPSIConflict, identity, holder.Imsi)
		}

		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("query failed: %w", err)
		}

		return nil
	}

	holders, err := queryAll(ctx, db.runner(ctx), listSubscriberMSISDNsSQL, scanSubscriberMSISDN)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	m := psi.NewMatcher(map[string]string{identity: ""})

	for _, holder := range holders {
		for _, impu := range msisdnIdentities(holder.Msisdn, imsDomains) {
			if _, ok := m.Match(impu); ok {
				return fmt.Errorf("%w: %s covers %s, a public user identity of subscriber %s", ErrPSIConflict, identity, impu, holder.Imsi)
			}
		}
	}

	return nil
}

func (db *Database) imsDomain(ctx context.Context) (string, error) {
	domains, err := db.imsHomeDomains(ctx)
	if err != nil {
		return "", err
	}

	return domains.Primary, nil
}

func (db *Database) rehomeIMSApplicationServers(ctx context.Context, from, to string) error {
	if from == "" || to == "" || from == to || !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return nil
	}

	rows, err := queryAll(ctx, db.runner(ctx), listIMSApplicationServersSQL, scanIMSApplicationServerRow)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	stored, err := queryAll(ctx, db.runner(ctx), listIMSPSIsSQL, scanIMSPSIRow)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	for _, row := range rows {
		if serverName, ok := psi.Rehome(row.ServerName, from, to); ok {
			row.ServerName = serverName
			if _, err := db.runner(ctx).ExecContext(ctx, updateIMSApplicationServerSQL, row.ServerName, row.ID); err != nil {
				return fmt.Errorf("query failed: %w", err)
			}
		}

		var identities []string

		rehomed := false

		for _, p := range stored {
			if p.ApplicationServerID != row.ID {
				continue
			}

			identity, ok := psi.Rehome(p.Identity, from, to)
			rehomed = rehomed || ok

			identities = append(identities, identity)
		}

		if !rehomed {
			continue
		}

		if _, err := db.runner(ctx).ExecContext(ctx, deleteIMSPSIsByASSQL, row.ID); err != nil {
			return fmt.Errorf("query failed: %w", err)
		}

		if err := db.insertIMSPSIs(ctx, row.ID, identities); err != nil {
			return err
		}
	}

	return nil
}

func collidesWithSubscriberIdentity(identity, imsDomain string) bool {
	if imsDomain == "" || psi.Host(identity) != imsDomain {
		return false
	}

	if !psi.Wildcarded(identity) {
		user := psi.User(identity)

		return len(user) >= 6 && len(user) <= 15 && strings.Trim(user, "0123456789") == ""
	}

	return psi.Covers(identity, `^sip:[0-9]{6,15}@`+regexp.QuoteMeta(imsDomain)+`$`)
}

func msisdnIdentities(msisdn string, imsDomains []string) []string {
	identities := []string{"tel:+" + msisdn}
	for _, imsDomain := range imsDomains {
		identities = append(identities, "sip:+"+msisdn+"@"+imsDomain, "sip:"+imsMCPTTUserPrefix+msisdn+"@"+imsDomain)
	}

	return identities
}

func (db *Database) msisdnPSI(ctx context.Context, msisdn string) (string, string, error) {
	if msisdn == "" || !db.appliedSchemaAtLeast(ctx, imsServicesSchema) {
		return "", "", nil
	}

	stored, err := queryAll(ctx, db.runner(ctx), listIMSPSIsSQL, scanIMSPSIRow)
	if err != nil {
		return "", "", fmt.Errorf("query failed: %w", err)
	}

	domains, err := db.imsHomeDomains(ctx)
	if err != nil {
		return "", "", err
	}

	identities := make(map[string]string, len(stored))
	for _, row := range stored {
		identities[row.Identity] = row.Identity
	}

	matcher := psi.NewMatcher(identities)

	for _, identity := range msisdnIdentities(msisdn, domains.Accepted()) {
		if m, ok := matcher.Match(identity); ok {
			return identity, m.Owner, nil
		}
	}

	return "", "", nil
}

func (db *Database) insertIMSPSIs(ctx context.Context, applicationServerID string, psis []string) error {
	for _, identity := range psis {
		if _, err := db.runner(ctx).ExecContext(ctx, insertIMSPSISQL, identity, applicationServerID); err != nil {
			if isPrimaryKeyError(err) {
				return fmt.Errorf("%w: %s is hosted by another application server", ErrPSIConflict, identity)
			}

			return fmt.Errorf("query failed: %w", err)
		}
	}

	return nil
}

func (db *Database) applyDeleteIMSApplicationServer(ctx context.Context, p *stringPayload) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, deleteIMSApplicationServerSQL, p.Value)
	if err != nil {
		if isForeignKeyError(err) {
			return nil, ErrInUse
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func scanIMSApplicationServerRow(row scanner) (imsApplicationServerRow, error) {
	var v imsApplicationServerRow

	if err := row.Scan(&v.ID, &v.Name, &v.ServerName); err != nil {
		return imsApplicationServerRow{}, err
	}

	return v, nil
}

func scanIMSPSIRow(row scanner) (imsPSIRow, error) {
	var v imsPSIRow

	if err := row.Scan(&v.Identity, &v.ApplicationServerID); err != nil {
		return imsPSIRow{}, err
	}

	return v, nil
}
