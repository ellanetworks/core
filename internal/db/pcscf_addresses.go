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
	"github.com/ellanetworks/core/internal/models"
	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const PCSCFAddressesTableName = "pcscf_addresses"

const pcscfAddressesSchema = 23

const MaxPCSCFAddressesPerFamily = 5

const MaxPCSCFAddresses = 2 * MaxPCSCFAddressesPerFamily

var limitedBroadcast = netip.AddrFrom4([4]byte{255, 255, 255, 255})

const (
	listPCSCFAddressesStmt   = "SELECT &pcscfAddressRow.* FROM %s ORDER BY priority ASC"
	insertPCSCFAddressStmt   = "INSERT INTO %s (priority, address) VALUES ($pcscfAddressRow.priority, $pcscfAddressRow.address)"
	deletePCSCFAddressesStmt = "DELETE FROM %s"
)

type pcscfAddressRow struct {
	Priority int    `db:"priority"`
	Address  string `db:"address"`
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

func (db *Database) ListPCSCFAddresses(ctx context.Context) ([]string, error) {
	ctx, span := startPCSCFAddressesSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PCSCFAddressesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PCSCFAddressesTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, pcscfAddressesSchema) {
		return []string{}, nil
	}

	var rows []pcscfAddressRow

	err := db.conn().Query(ctx, db.listPCSCFAddressesStmt).GetAll(&rows)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	addresses := make([]string, 0, len(rows))
	for _, row := range rows {
		addresses = append(addresses, row.Address)
	}

	return addresses, nil
}

func (db *Database) applyReplacePCSCFAddresses(ctx context.Context, payload *pcscfAddressesPayload) (any, error) {
	if err := db.runner(ctx).Query(ctx, db.deletePCSCFAddressesStmt).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	for i, addr := range payload.Addresses {
		if err := db.runner(ctx).Query(ctx, db.insertPCSCFAddressStmt, pcscfAddressRow{Priority: i, Address: addr}).Run(); err != nil {
			return nil, fmt.Errorf("query failed: %w", err)
		}
	}

	return nil, nil
}

func startPCSCFAddressesSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	querySummary := fmt.Sprintf("%s %s", operation, PCSCFAddressesTableName)

	return tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(PCSCFAddressesTableName),
		),
	)
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
