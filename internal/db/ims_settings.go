// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/internal/diameternode"
)

const IMSSettingsTableName = "ims_settings"

const imsIntegrationsSchema = 27

const threeGPPNetworkDomain = "3gppnetwork.org"

const (
	getIMSSettingsSQL    = "SELECT homeDomain, advertiseVoice, restrictPeers FROM " + IMSSettingsTableName + " WHERE singleton=TRUE"
	upsertIMSSettingsSQL = "INSERT INTO " + IMSSettingsTableName + " (singleton, homeDomain, advertiseVoice, restrictPeers) VALUES (TRUE, ?, ?, ?) ON CONFLICT(singleton) DO UPDATE SET homeDomain=excluded.homeDomain, advertiseVoice=excluded.advertiseVoice, restrictPeers=excluded.restrictPeers"
)

type IMSSettings struct {
	HomeDomain     string `json:"homeDomain"`
	AdvertiseVoice bool   `json:"advertiseVoice"`
	RestrictPeers  bool   `json:"restrictPeers"`
}

func DefaultIMSSettings() IMSSettings {
	return IMSSettings{AdvertiseVoice: true}
}

type imsSettingsPayload struct {
	Settings       IMSSettings `json:"settings"`
	PCSCFAddresses []string    `json:"pcscfAddresses"`
}

type IMSHomeDomains struct {
	Primary string
	Derived string
}

func (d IMSHomeDomains) Accepted() []string {
	if d.Primary == "" {
		return nil
	}

	if d.Derived == "" || d.Derived == d.Primary {
		return []string{d.Primary}
	}

	return []string{d.Primary, d.Derived}
}

func (d IMSHomeDomains) Accepts(domain string) bool {
	for _, accepted := range d.Accepted() {
		if strings.EqualFold(accepted, domain) {
			return true
		}
	}

	return false
}

func NormalizeIMSHomeDomain(domain, mcc, mnc string) (string, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return "", nil
	}

	if _, err := netip.ParseAddr(domain); err == nil {
		return "", fmt.Errorf("IMS home domain must be a domain name, not an IP address, got %q", domain)
	}

	if !diameter.IsValidHostname(domain) || !strings.Contains(domain, ".") {
		return "", fmt.Errorf("IMS home domain must be a domain name with at least two labels, got %q", domain)
	}

	if domain != threeGPPNetworkDomain && !strings.HasSuffix(domain, "."+threeGPPNetworkDomain) {
		return domain, nil
	}

	derived, err := diameternode.IMSRealm(mcc, mnc)
	if err != nil {
		return "", fmt.Errorf("IMS home domain %q is a 3GPP network domain, which needs the operator MCC and MNC", domain)
	}

	if domain == derived {
		return "", nil
	}

	epc, _ := diameternode.DiameterRealm(mcc, mnc)
	plmn := strings.TrimPrefix(derived, "ims.")

	if domain == epc || !strings.HasSuffix(domain, "."+plmn) {
		return "", fmt.Errorf("IMS home domain %q is reserved: use %s or a domain outside %s", domain, derived, threeGPPNetworkDomain)
	}

	return domain, nil
}

func (db *Database) GetIMSSettings(ctx context.Context) (_ *IMSSettings, err error) {
	ctx, finish := startDBQuery(ctx, IMSSettingsTableName, "SELECT")
	defer func() { finish(err) }()

	settings, err := db.imsSettings(ctx)
	if err != nil {
		return nil, err
	}

	return settings, nil
}

func (db *Database) UpdateIMSConfiguration(ctx context.Context, settings *IMSSettings, pcscfAddresses []string) (err error) {
	if err := ValidatePCSCFAddresses(pcscfAddresses); err != nil {
		return err
	}

	addresses := slices.Clone(pcscfAddresses)
	if addresses == nil {
		addresses = []string{}
	}

	op, err := db.GetOperator(ctx)
	if err != nil {
		return fmt.Errorf("get operator: %w", err)
	}

	domain, err := NormalizeIMSHomeDomain(settings.HomeDomain, op.Mcc, op.Mnc)
	if err != nil {
		return err
	}

	ctx, finish := startDBQuery(ctx, IMSSettingsTableName, "UPSERT")
	defer func() { finish(err) }()

	payload := &imsSettingsPayload{Settings: IMSSettings{HomeDomain: domain, AdvertiseVoice: settings.AdvertiseVoice, RestrictPeers: settings.RestrictPeers}, PCSCFAddresses: addresses}

	if _, err := opUpdateIMSSettings.Invoke(ctx, db, payload); err != nil {
		return err
	}

	return nil
}

func (db *Database) GetIMSHomeDomains(ctx context.Context) (IMSHomeDomains, error) {
	return db.imsHomeDomains(ctx)
}

func (db *Database) imsSettings(ctx context.Context) (*IMSSettings, error) {
	settings := DefaultIMSSettings()

	if !db.appliedSchemaAtLeast(ctx, imsIntegrationsSchema) {
		return &settings, nil
	}

	if err := db.readQuerier(ctx).QueryRowContext(ctx, getIMSSettingsSQL).Scan(&settings.HomeDomain, &settings.AdvertiseVoice, &settings.RestrictPeers); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			defaults := DefaultIMSSettings()
			return &defaults, nil
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &settings, nil
}

func (db *Database) imsHomeDomains(ctx context.Context) (IMSHomeDomains, error) {
	op, err := getOperator(ctx, db.readQuerier(ctx))
	if err != nil {
		return IMSHomeDomains{}, fmt.Errorf("get operator: %w", err)
	}

	derived, err := diameternode.IMSRealm(op.Mcc, op.Mnc)
	if err != nil {
		derived = ""
	}

	settings, err := db.imsSettings(ctx)
	if err != nil {
		return IMSHomeDomains{}, err
	}

	primary := settings.HomeDomain
	if primary == "" {
		primary = derived
	}

	return IMSHomeDomains{Primary: primary, Derived: derived}, nil
}

func (db *Database) applyUpdateIMSSettings(ctx context.Context, p *imsSettingsPayload) (any, error) {
	from, err := db.imsDomain(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := db.applyReplacePCSCFAddresses(ctx, &pcscfAddressesPayload{Addresses: p.PCSCFAddresses}); err != nil {
		return nil, err
	}

	if _, err := db.runner(ctx).ExecContext(ctx, upsertIMSSettingsSQL, p.Settings.HomeDomain, p.Settings.AdvertiseVoice, p.Settings.RestrictPeers); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	to, err := db.imsDomain(ctx)
	if err != nil {
		return nil, err
	}

	if err := db.rehomeIMSApplicationServers(ctx, from, to); err != nil {
		return nil, err
	}

	return nil, nil
}
