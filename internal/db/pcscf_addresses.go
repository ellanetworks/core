// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/internal/models"
)

const PCSCFAddressesTableName = "pcscf_addresses"

const pcscfAddressesSchema = 23

const MaxPCSCFAddressesPerFamily = 5

const MaxPCSCFAddresses = 2 * MaxPCSCFAddressesPerFamily

var limitedBroadcast = netip.AddrFrom4([4]byte{255, 255, 255, 255})

const (
	listPCSCFAddressesSQL   = "SELECT priority, address FROM " + PCSCFAddressesTableName + " ORDER BY priority ASC"
	insertPCSCFAddressSQL   = "INSERT INTO " + PCSCFAddressesTableName + " (priority, address) VALUES (?, ?)"
	deletePCSCFAddressesSQL = "DELETE FROM " + PCSCFAddressesTableName
)

type pcscfAddressRow struct {
	Priority int
	Address  string
}

type pcscfAddressesPayload struct {
	Addresses []string `json:"addresses"`
}

func ValidatePCSCFAddresses(addresses []string) error {
	seen := make(map[string]struct{}, len(addresses))

	var ipv4, ipv6 int

	for _, address := range addresses {
		addr, err := netip.ParseAddr(address)

		switch {
		case err == nil:
			if !IsPCSCFUnicast(addr) || addr.String() != address {
				return fmt.Errorf("P-CSCF address must be a unicast IPv4 or IPv6 address, got %q", address)
			}

			if addr.Is4() {
				ipv4++
			} else {
				ipv6++
			}
		case !diameter.IsValidHostname(address) || strings.ToLower(address) != address:
			return fmt.Errorf("P-CSCF address must be an IPv4 or IPv6 address or a lowercase host name, got %q", address)
		}

		if _, dup := seen[address]; dup {
			return fmt.Errorf("P-CSCF address %s is listed more than once", address)
		}

		seen[address] = struct{}{}
	}

	if ipv4 > MaxPCSCFAddressesPerFamily {
		return fmt.Errorf("at most %d IPv4 P-CSCF addresses are allowed, got %d", MaxPCSCFAddressesPerFamily, ipv4)
	}

	if ipv6 > MaxPCSCFAddressesPerFamily {
		return fmt.Errorf("at most %d IPv6 P-CSCF addresses are allowed, got %d", MaxPCSCFAddressesPerFamily, ipv6)
	}

	if len(addresses) > MaxPCSCFAddresses {
		return fmt.Errorf("at most %d P-CSCF addresses are allowed, got %d", MaxPCSCFAddresses, len(addresses))
	}

	return nil
}

func IsPCSCFUnicast(addr netip.Addr) bool {
	return addr.IsValid() && addr.Zone() == "" && !addr.IsUnspecified() && !addr.IsLoopback() && !addr.IsMulticast() && !addr.Is4In6() &&
		!addr.IsLinkLocalUnicast() && addr != limitedBroadcast
}

func (db *Database) ListPCSCFAddresses(ctx context.Context) (_ []string, err error) {
	ctx, finish := startDBQuery(ctx, PCSCFAddressesTableName, "SELECT")
	defer func() { finish(err) }()

	if !db.appliedSchemaAtLeast(ctx, pcscfAddressesSchema) {
		return []string{}, nil
	}

	rows, err := queryAll(ctx, db.reader(), listPCSCFAddressesSQL, scanPCSCFAddressRow)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	addresses := make([]string, 0, len(rows))
	for _, row := range rows {
		addresses = append(addresses, row.Address)
	}

	return addresses, nil
}

func (db *Database) applyReplacePCSCFAddresses(ctx context.Context, payload *pcscfAddressesPayload) (any, error) {
	if _, err := db.runner(ctx).ExecContext(ctx, deletePCSCFAddressesSQL); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	for i, addr := range payload.Addresses {
		if _, err := db.runner(ctx).ExecContext(ctx, insertPCSCFAddressSQL, i, addr); err != nil {
			return nil, fmt.Errorf("query failed: %w", err)
		}
	}

	return nil, nil
}

type IMSVoiceStatus string

const (
	IMSVoiceEnabled          IMSVoiceStatus = "enabled"
	IMSVoiceNoPCSCF          IMSVoiceStatus = "no_pcscf"
	IMSVoiceNoDataNetwork    IMSVoiceStatus = "no_ims_data_network"
	IMSVoiceIPFamilyMismatch IMSVoiceStatus = "ip_family_mismatch"
)

func (db *Database) IMSVoiceStatus(ctx context.Context) (IMSVoiceStatus, error) {
	addresses, err := db.ListPCSCFAddresses(ctx)
	if err != nil {
		return "", err
	}

	if len(addresses) == 0 {
		return IMSVoiceNoPCSCF, nil
	}

	dn, err := db.GetDataNetwork(ctx, models.IMSDataNetworkName)
	if errors.Is(err, ErrNotFound) {
		return IMSVoiceNoDataNetwork, nil
	}

	if err != nil {
		return "", err
	}

	if !dn.IMS {
		return IMSVoiceNoDataNetwork, nil
	}

	if slices.ContainsFunc(addresses, func(address string) bool {
		addr, err := netip.ParseAddr(address)

		switch {
		case err != nil:
			return dn.IPv4Pool != "" || dn.IPv6Pool != ""
		case addr.Is4():
			return dn.IPv4Pool != ""
		default:
			return dn.IPv6Pool != ""
		}
	}) {
		return IMSVoiceEnabled, nil
	}

	return IMSVoiceIPFamilyMismatch, nil
}

func scanPCSCFAddressRow(row scanner) (pcscfAddressRow, error) {
	var v pcscfAddressRow

	if err := row.Scan(&v.Priority, &v.Address); err != nil {
		return pcscfAddressRow{}, err
	}

	return v, nil
}
