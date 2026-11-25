// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	OIDCSettingsTableName   = "oidc_settings"
	OIDCGroupRolesTableName = "oidc_group_roles"
	OIDCUsedStatesTableName = "oidc_used_states"
)

const (
	getOIDCSettingsSQL = "SELECT enabled, externalURL, issuer, clientID, clientSecret, displayName, scopes, sessionLifetime, trustUnverifiedEmail, caCertificate, groupsClaim FROM " +
		OIDCSettingsTableName + " WHERE singleton=TRUE"
	getOIDCIssuerSQL        = "SELECT enabled, issuer, clientID FROM " + OIDCSettingsTableName + " WHERE singleton=TRUE"
	upsertOIDCSettingsSQL   = "INSERT INTO " + OIDCSettingsTableName + " (singleton, enabled, externalURL, issuer, clientID, clientSecret, displayName, scopes, sessionLifetime, trustUnverifiedEmail, caCertificate, groupsClaim) VALUES (TRUE, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(singleton) DO UPDATE SET enabled=excluded.enabled, externalURL=excluded.externalURL, issuer=excluded.issuer, clientID=excluded.clientID, clientSecret=excluded.clientSecret, displayName=excluded.displayName, scopes=excluded.scopes, sessionLifetime=excluded.sessionLifetime, trustUnverifiedEmail=excluded.trustUnverifiedEmail, caCertificate=excluded.caCertificate, groupsClaim=excluded.groupsClaim" // #nosec: G101
	listOIDCGroupRolesSQL   = "SELECT groupName, roleID FROM " + OIDCGroupRolesTableName + " ORDER BY position"
	deleteOIDCGroupRolesSQL = "DELETE FROM " + OIDCGroupRolesTableName
	insertOIDCGroupRoleSQL  = "INSERT INTO " + OIDCGroupRolesTableName + " (position, groupName, roleID) VALUES (?, ?, ?)"
	deleteOIDCSessionsSQL   = "DELETE FROM " + SessionsTableName + " WHERE method = '" + string(SessionOIDC) + "'"
)

const DefaultOIDCSessionLifetime = 24 * time.Hour

var DefaultOIDCScopes = []string{"openid", "email", "profile"}

type OIDC struct {
	Enabled              bool          `json:"enabled"`
	ExternalURL          string        `json:"externalURL"`
	Issuer               string        `json:"issuer"`
	ClientID             string        `json:"clientID"`
	ClientSecret         string        `json:"clientSecret"`
	DisplayName          string        `json:"displayName"`
	Scopes               []string      `json:"scopes"`
	SessionLifetime      time.Duration `json:"sessionLifetime"`
	TrustUnverifiedEmail bool          `json:"trustUnverifiedEmail"`
	CACertificate        string        `json:"caCertificate"`
	GroupsClaim          string        `json:"groupsClaim"`
	GroupRoles           []GroupRole   `json:"groupRoles"`
}

type GroupRole struct {
	Group  string `json:"group"`
	RoleID RoleID `json:"roleID"`
}

func DefaultOIDC() OIDC {
	return OIDC{
		Scopes:          append([]string{}, DefaultOIDCScopes...),
		SessionLifetime: DefaultOIDCSessionLifetime,
		GroupRoles:      []GroupRole{},
	}
}

func (db *Database) GetOIDC(ctx context.Context) (_ OIDC, err error) {
	ctx, finish := startDBQuery(ctx, OIDCSettingsTableName, "SELECT")
	defer func() { finish(err) }()

	return db.readOIDC(ctx)
}

func (db *Database) readOIDC(ctx context.Context) (OIDC, error) {
	o := DefaultOIDC()

	var (
		scopes   string
		lifetime int64
	)

	err := db.readQuerier(ctx).QueryRowContext(ctx, getOIDCSettingsSQL).Scan(&o.Enabled, &o.ExternalURL, &o.Issuer, &o.ClientID,
		&o.ClientSecret, &o.DisplayName, &scopes, &lifetime, &o.TrustUnverifiedEmail, &o.CACertificate, &o.GroupsClaim)
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultOIDC(), nil
	}

	if err != nil {
		return OIDC{}, fmt.Errorf("query failed: %w", err)
	}

	o.Scopes = strings.Fields(scopes)
	o.SessionLifetime = time.Duration(lifetime) * time.Second

	o.GroupRoles, err = queryAll(ctx, db.readQuerier(ctx), listOIDCGroupRolesSQL, func(row scanner) (GroupRole, error) {
		var g GroupRole

		err := row.Scan(&g.Group, &g.RoleID)

		return g, err
	})
	if err != nil {
		return OIDC{}, fmt.Errorf("query failed: %w", err)
	}

	if o.GroupRoles == nil {
		o.GroupRoles = []GroupRole{}
	}

	return o, nil
}

func (db *Database) UpdateOIDC(ctx context.Context, o OIDC) (err error) {
	ctx, finish := startDBQuery(ctx, OIDCSettingsTableName, "UPSERT")
	defer func() { finish(err) }()

	_, err = opUpdateOIDC.Invoke(ctx, db, &o)

	return err
}

func (db *Database) applyUpdateOIDC(ctx context.Context, o *OIDC) (any, error) {
	current, err := db.readOIDC(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := db.runner(ctx).ExecContext(ctx, upsertOIDCSettingsSQL, o.Enabled, o.ExternalURL, o.Issuer, o.ClientID,
		o.ClientSecret, o.DisplayName, strings.Join(o.Scopes, " "), int64(o.SessionLifetime/time.Second),
		o.TrustUnverifiedEmail, o.CACertificate, o.GroupsClaim); err != nil {
		return nil, fmt.Errorf("upsert OIDC settings: %w", err)
	}

	if _, err := db.runner(ctx).ExecContext(ctx, deleteOIDCGroupRolesSQL); err != nil {
		return nil, fmt.Errorf("delete OIDC group roles: %w", err)
	}

	for i, g := range o.GroupRoles {
		if _, err := db.runner(ctx).ExecContext(ctx, insertOIDCGroupRoleSQL, i+1, g.Group, g.RoleID); err != nil {
			return nil, fmt.Errorf("insert OIDC group role: %w", err)
		}
	}

	if !o.Enabled || o.Issuer != current.Issuer {
		if _, err := db.runner(ctx).ExecContext(ctx, deleteOIDCSessionsSQL); err != nil {
			return nil, fmt.Errorf("delete OIDC sessions: %w", err)
		}
	}

	return nil, nil
}
