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

	"github.com/ellanetworks/core/internal/models"
	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const PCSCFAddressesTableName = "pcscf_addresses"

const pcscfAddressesSchema = 23

const MaxPCSCFAddressesPerFamily = 3

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

func ValidatePCSCFAddresses(addresses []netip.Addr) error {
	seen := make(map[netip.Addr]struct{}, len(addresses))

	var ipv4, ipv6 int

	for _, addr := range addresses {
		if !addr.IsValid() || addr.Zone() != "" || addr.IsUnspecified() || addr.IsLoopback() || addr.IsMulticast() || addr.Is4In6() ||
			addr.IsLinkLocalUnicast() || addr == limitedBroadcast {
			return fmt.Errorf("P-CSCF address must be a unicast IPv4 or IPv6 address, got %q", addr)
		}

		if _, dup := seen[addr]; dup {
			return fmt.Errorf("P-CSCF address %s is listed more than once", addr)
		}

		seen[addr] = struct{}{}

		if addr.Is4() {
			ipv4++
		} else {
			ipv6++
		}
	}

	if ipv4 > MaxPCSCFAddressesPerFamily {
		return fmt.Errorf("at most %d IPv4 P-CSCF addresses are allowed, got %d", MaxPCSCFAddressesPerFamily, ipv4)
	}

	if ipv6 > MaxPCSCFAddressesPerFamily {
		return fmt.Errorf("at most %d IPv6 P-CSCF addresses are allowed, got %d", MaxPCSCFAddressesPerFamily, ipv6)
	}

	return nil
}

func (db *Database) ListPCSCFAddresses(ctx context.Context) ([]netip.Addr, error) {
	ctx, span := startPCSCFAddressesSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PCSCFAddressesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PCSCFAddressesTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, pcscfAddressesSchema) {
		return []netip.Addr{}, nil
	}

	var rows []pcscfAddressRow

	err := db.conn().Query(ctx, db.listPCSCFAddressesStmt).GetAll(&rows)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	addresses := make([]netip.Addr, 0, len(rows))

	for _, row := range rows {
		addr, err := netip.ParseAddr(row.Address)
		if err != nil {
			recordSpanError(span, err)

			return nil, fmt.Errorf("stored P-CSCF address %q: %w", row.Address, err)
		}

		addresses = append(addresses, addr)
	}

	return addresses, nil
}

func (db *Database) ReplacePCSCFAddresses(ctx context.Context, addresses []netip.Addr) error {
	if err := ValidatePCSCFAddresses(addresses); err != nil {
		return err
	}

	ctx, span := startPCSCFAddressesSpan(ctx, "REPLACE")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PCSCFAddressesTableName, "replace"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PCSCFAddressesTableName, "replace").Inc()

	payload := &pcscfAddressesPayload{Addresses: make([]string, 0, len(addresses))}
	for _, addr := range addresses {
		payload.Addresses = append(payload.Addresses, addr.String())
	}

	if _, err := opReplacePCSCFAddresses.Invoke(ctx, db, payload); err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
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

	if slices.ContainsFunc(addresses, func(addr netip.Addr) bool {
		if addr.Is4() {
			return dn.IPv4Pool != ""
		}

		return dn.IPv6Pool != ""
	}) {
		return IMSVoiceEnabled, nil
	}

	return IMSVoiceIPFamilyMismatch, nil
}
