// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/ellanetworks/core/fleet/client"
	"github.com/ellanetworks/core/internal/logger"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// UpdateConfig applies the fleet-provided SyncConfig to the local database.
// It runs as a single transaction: for each entity type it compares the
// incoming desired state with the current state and creates, updates, or
// deletes rows as needed.
func (db *Database) UpdateConfig(ctx context.Context, cfg client.SyncConfig) error {
	ctx, span := tracer.Start(ctx, "UpdateConfig", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	tx, err := db.conn.PlainDB().BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "begin transaction failed")

		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = syncOperator(ctx, tx, cfg.Operator); err != nil {
		span.RecordError(err)
		return fmt.Errorf("sync operator: %w", err)
	}

	if err = syncNATSettings(ctx, tx, cfg.Networking.NAT); err != nil {
		span.RecordError(err)
		return fmt.Errorf("sync NAT settings: %w", err)
	}

	if err = syncN3Settings(ctx, tx, cfg.Networking.NetworkInterfaces.N3ExternalAddress); err != nil {
		span.RecordError(err)
		return fmt.Errorf("sync N3 settings: %w", err)
	}

	if err = syncDataNetworks(ctx, tx, cfg.Networking.DataNetworks); err != nil {
		span.RecordError(err)
		return fmt.Errorf("sync data networks: %w", err)
	}

	if err = syncPolicies(ctx, tx, cfg.Policies, cfg.Networking.DataNetworks); err != nil {
		span.RecordError(err)
		return fmt.Errorf("sync policies: %w", err)
	}

	if err = syncSubscribers(ctx, tx, cfg.Subscribers, cfg.Policies); err != nil {
		span.RecordError(err)
		return fmt.Errorf("sync subscribers: %w", err)
	}

	if err = syncRoutes(ctx, tx, cfg.Networking.Routes); err != nil {
		span.RecordError(err)
		return fmt.Errorf("sync routes: %w", err)
	}

	if err = tx.Commit(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "commit failed")

		return fmt.Errorf("commit transaction: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// syncOperator updates the singleton operator row to match the desired state.
func syncOperator(ctx context.Context, tx *sql.Tx, desired client.Operator) error {
	supportedTACs, err := json.Marshal(desired.Tracking.SupportedTacs)
	if err != nil {
		return fmt.Errorf("marshal supported TACs: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		fmt.Sprintf("UPDATE %s SET mcc=?, mnc=?, operatorCode=?, supportedTACs=?, sst=?, sd=?, homeNetworkPrivateKey=? WHERE id=1", OperatorTableName),
		desired.ID.Mcc,
		desired.ID.Mnc,
		desired.OperatorCode,
		string(supportedTACs),
		desired.Slice.Sst,
		desired.Slice.Sd,
		desired.HomeNetwork.PrivateKey,
	)
	if err != nil {
		return fmt.Errorf("update operator: %w", err)
	}

	return nil
}

// syncNATSettings upserts the NAT enabled flag.
func syncNATSettings(ctx context.Context, tx *sql.Tx, enabled bool) error {
	_, err := tx.ExecContext(ctx,
		fmt.Sprintf("INSERT INTO %s (singleton, enabled) VALUES (TRUE, ?) ON CONFLICT(singleton) DO UPDATE SET enabled=?", NATSettingsTableName),
		enabled, enabled,
	)
	if err != nil {
		return fmt.Errorf("upsert NAT settings: %w", err)
	}

	return nil
}

// syncN3Settings upserts the N3 external address.
func syncN3Settings(ctx context.Context, tx *sql.Tx, externalAddress string) error {
	_, err := tx.ExecContext(ctx,
		fmt.Sprintf("INSERT INTO %s (singleton, external_address) VALUES (TRUE, ?) ON CONFLICT(singleton) DO UPDATE SET external_address=?", N3SettingsTableName),
		externalAddress, externalAddress,
	)
	if err != nil {
		return fmt.Errorf("upsert N3 settings: %w", err)
	}

	return nil
}

// syncDataNetworks reconciles data networks: creates new, updates changed, deletes removed.
func syncDataNetworks(ctx context.Context, tx *sql.Tx, desired []client.DataNetwork) error {
	rows, err := tx.QueryContext(ctx, fmt.Sprintf("SELECT id, name, ipPool, dns, mtu FROM %s", DataNetworksTableName))
	if err != nil {
		return fmt.Errorf("list data networks: %w", err)
	}

	defer func() { _ = rows.Close() }()

	existing := make(map[string]DataNetwork)

	for rows.Next() {
		var dn DataNetwork

		if err := rows.Scan(&dn.ID, &dn.Name, &dn.IPPool, &dn.DNS, &dn.MTU); err != nil {
			return fmt.Errorf("scan data network: %w", err)
		}

		existing[dn.Name] = dn
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate data networks: %w", err)
	}

	desiredNames := make(map[string]bool, len(desired))

	for _, d := range desired {
		desiredNames[d.Name] = true

		if cur, ok := existing[d.Name]; ok {
			// Update if any field differs.
			if cur.IPPool != d.IPPool || cur.DNS != d.DNS || cur.MTU != d.MTU {
				_, err := tx.ExecContext(ctx,
					fmt.Sprintf("UPDATE %s SET ipPool=?, dns=?, mtu=? WHERE name=?", DataNetworksTableName),
					d.IPPool, d.DNS, d.MTU, d.Name,
				)
				if err != nil {
					return fmt.Errorf("update data network %q: %w", d.Name, err)
				}

				logger.DBLog.Info("Updated data network from fleet config", zap.String("name", d.Name))
			}
		} else {
			// Create new.
			_, err := tx.ExecContext(ctx,
				fmt.Sprintf("INSERT INTO %s (name, ipPool, dns, mtu) VALUES (?, ?, ?, ?)", DataNetworksTableName),
				d.Name, d.IPPool, d.DNS, d.MTU,
			)
			if err != nil {
				return fmt.Errorf("create data network %q: %w", d.Name, err)
			}

			logger.DBLog.Info("Created data network from fleet config", zap.String("name", d.Name))
		}
	}

	// Delete data networks not in the desired set.
	for name := range existing {
		if !desiredNames[name] {
			_, err := tx.ExecContext(ctx,
				fmt.Sprintf("DELETE FROM %s WHERE name=?", DataNetworksTableName),
				name,
			)
			if err != nil {
				return fmt.Errorf("delete data network %q: %w", name, err)
			}

			logger.DBLog.Info("Deleted data network from fleet config", zap.String("name", name))
		}
	}

	return nil
}

// syncPolicies reconciles policies: creates new, updates changed, deletes removed.
// Policies reference data networks by ID. The desired config uses fleet-assigned
// data_network_id values which may differ from local DB IDs. We resolve them
// by mapping fleet DN ID → DN name → local DN ID.
func syncPolicies(ctx context.Context, tx *sql.Tx, desired []client.Policy, fleetDataNetworks []client.DataNetwork) error {
	// Build a map from fleet data network ID to data network name.
	fleetDNIDToName := make(map[int]string, len(fleetDataNetworks))
	for _, dn := range fleetDataNetworks {
		fleetDNIDToName[dn.ID] = dn.Name
	}

	// Build a map from data network name to local DB ID.
	dnRows, err := tx.QueryContext(ctx, fmt.Sprintf("SELECT id, name FROM %s", DataNetworksTableName))
	if err != nil {
		return fmt.Errorf("list data networks for policy resolution: %w", err)
	}

	defer func() { _ = dnRows.Close() }()

	localDNNameToID := make(map[string]int)

	for dnRows.Next() {
		var id int

		var name string

		if err := dnRows.Scan(&id, &name); err != nil {
			return fmt.Errorf("scan data network: %w", err)
		}

		localDNNameToID[name] = id
	}

	if err := dnRows.Err(); err != nil {
		return fmt.Errorf("iterate data networks: %w", err)
	}

	rows, err := tx.QueryContext(ctx,
		fmt.Sprintf("SELECT id, name, bitrateUplink, bitrateDownlink, var5qi, arp, dataNetworkID FROM %s", PoliciesTableName),
	)
	if err != nil {
		return fmt.Errorf("list policies: %w", err)
	}

	defer func() { _ = rows.Close() }()

	existing := make(map[string]Policy)

	for rows.Next() {
		var p Policy

		if err := rows.Scan(&p.ID, &p.Name, &p.BitrateUplink, &p.BitrateDownlink, &p.Var5qi, &p.Arp, &p.DataNetworkID); err != nil {
			return fmt.Errorf("scan policy: %w", err)
		}

		existing[p.Name] = p
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate policies: %w", err)
	}

	desiredNames := make(map[string]bool, len(desired))

	for _, d := range desired {
		desiredNames[d.Name] = true

		// Resolve fleet data network ID to local DB ID.
		dnName, ok := fleetDNIDToName[d.DataNetworkID]
		if !ok {
			return fmt.Errorf("policy %q references unknown fleet data network ID %d", d.Name, d.DataNetworkID)
		}

		localDNID, ok := localDNNameToID[dnName]
		if !ok {
			return fmt.Errorf("policy %q references data network %q which does not exist locally", d.Name, dnName)
		}

		if cur, ok := existing[d.Name]; ok {
			if cur.BitrateUplink != d.BitrateUplink ||
				cur.BitrateDownlink != d.BitrateDownlink ||
				cur.Var5qi != d.Var5qi ||
				cur.Arp != d.Arp ||
				cur.DataNetworkID != localDNID {
				_, err := tx.ExecContext(ctx,
					fmt.Sprintf("UPDATE %s SET bitrateUplink=?, bitrateDownlink=?, var5qi=?, arp=?, dataNetworkID=? WHERE name=?", PoliciesTableName),
					d.BitrateUplink, d.BitrateDownlink, d.Var5qi, d.Arp, localDNID, d.Name,
				)
				if err != nil {
					return fmt.Errorf("update policy %q: %w", d.Name, err)
				}

				logger.DBLog.Info("Updated policy from fleet config", zap.String("name", d.Name))
			}
		} else {
			_, err := tx.ExecContext(ctx,
				fmt.Sprintf("INSERT INTO %s (name, bitrateUplink, bitrateDownlink, var5qi, arp, dataNetworkID) VALUES (?, ?, ?, ?, ?, ?)", PoliciesTableName),
				d.Name, d.BitrateUplink, d.BitrateDownlink, d.Var5qi, d.Arp, localDNID,
			)
			if err != nil {
				return fmt.Errorf("create policy %q: %w", d.Name, err)
			}

			logger.DBLog.Info("Created policy from fleet config", zap.String("name", d.Name))
		}
	}

	for name := range existing {
		if !desiredNames[name] {
			_, err := tx.ExecContext(ctx,
				fmt.Sprintf("DELETE FROM %s WHERE name=?", PoliciesTableName),
				name,
			)
			if err != nil {
				return fmt.Errorf("delete policy %q: %w", name, err)
			}

			logger.DBLog.Info("Deleted policy from fleet config", zap.String("name", name))
		}
	}

	return nil
}

// syncSubscribers reconciles subscribers: creates new, updates changed, deletes removed.
// Subscribers are keyed by IMSI. PolicyID values from the fleet config are
// fleet-side IDs and must be resolved to local DB IDs via policy name.
func syncSubscribers(ctx context.Context, tx *sql.Tx, desired []client.Subscriber, fleetPolicies []client.Policy) error {
	// Build a map from fleet policy ID to policy name.
	fleetPolicyIDToName := make(map[int]string, len(fleetPolicies))
	for _, p := range fleetPolicies {
		fleetPolicyIDToName[p.ID] = p.Name
	}

	// Build a map from policy name to local DB ID.
	policyRows, err := tx.QueryContext(ctx, fmt.Sprintf("SELECT id, name FROM %s", PoliciesTableName))
	if err != nil {
		return fmt.Errorf("list policies for subscriber resolution: %w", err)
	}

	defer func() { _ = policyRows.Close() }()

	localPolicyNameToID := make(map[string]int)

	for policyRows.Next() {
		var id int

		var name string

		if err := policyRows.Scan(&id, &name); err != nil {
			return fmt.Errorf("scan policy: %w", err)
		}

		localPolicyNameToID[name] = id
	}

	if err := policyRows.Err(); err != nil {
		return fmt.Errorf("iterate policies: %w", err)
	}

	rows, err := tx.QueryContext(ctx,
		fmt.Sprintf("SELECT id, imsi, ipAddress, sequenceNumber, permanentKey, opc, policyID FROM %s", SubscribersTableName),
	)
	if err != nil {
		return fmt.Errorf("list subscribers: %w", err)
	}

	defer func() { _ = rows.Close() }()

	existing := make(map[string]Subscriber)

	for rows.Next() {
		var s Subscriber

		if err := rows.Scan(&s.ID, &s.Imsi, &s.IPAddress, &s.SequenceNumber, &s.PermanentKey, &s.Opc, &s.PolicyID); err != nil {
			return fmt.Errorf("scan subscriber: %w", err)
		}

		existing[s.Imsi] = s
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate subscribers: %w", err)
	}

	desiredIMSIs := make(map[string]bool, len(desired))

	for _, d := range desired {
		desiredIMSIs[d.Imsi] = true

		// Resolve fleet policy ID to local DB ID.
		policyName, ok := fleetPolicyIDToName[d.PolicyID]
		if !ok {
			return fmt.Errorf("subscriber %q references unknown fleet policy ID %d", d.Imsi, d.PolicyID)
		}

		localPolicyID, ok := localPolicyNameToID[policyName]
		if !ok {
			return fmt.Errorf("subscriber %q references policy %q which does not exist locally", d.Imsi, policyName)
		}

		if cur, ok := existing[d.Imsi]; ok {
			// Compare fields that can be updated. Sequence number and IP address
			// are local-only and should not be overwritten by fleet.
			if cur.PermanentKey != d.PermanentKey ||
				cur.Opc != d.Opc ||
				cur.PolicyID != localPolicyID {
				_, err := tx.ExecContext(ctx,
					fmt.Sprintf("UPDATE %s SET permanentKey=?, opc=?, policyID=? WHERE imsi=?", SubscribersTableName),
					d.PermanentKey, d.Opc, localPolicyID, d.Imsi,
				)
				if err != nil {
					return fmt.Errorf("update subscriber %q: %w", d.Imsi, err)
				}

				logger.DBLog.Info("Updated subscriber from fleet config", zap.String("imsi", d.Imsi))
			}
		} else {
			_, err := tx.ExecContext(ctx,
				fmt.Sprintf("INSERT INTO %s (imsi, sequenceNumber, permanentKey, opc, policyID) VALUES (?, ?, ?, ?, ?)", SubscribersTableName),
				d.Imsi, "000000000000", d.PermanentKey, d.Opc, localPolicyID,
			)
			if err != nil {
				return fmt.Errorf("create subscriber %q: %w", d.Imsi, err)
			}

			logger.DBLog.Info("Created subscriber from fleet config", zap.String("imsi", d.Imsi))
		}
	}

	for imsi := range existing {
		if !desiredIMSIs[imsi] {
			_, err := tx.ExecContext(ctx,
				fmt.Sprintf("DELETE FROM %s WHERE imsi=?", SubscribersTableName),
				imsi,
			)
			if err != nil {
				return fmt.Errorf("delete subscriber %q: %w", imsi, err)
			}

			logger.DBLog.Info("Deleted subscriber from fleet config", zap.String("imsi", imsi))
		}
	}

	return nil
}

// syncRoutes reconciles routes. Routes are keyed by (destination, gateway, interface, metric)
// since they have no natural name key.
func syncRoutes(ctx context.Context, tx *sql.Tx, desired []client.Route) error {
	rows, err := tx.QueryContext(ctx,
		fmt.Sprintf("SELECT id, destination, gateway, interface, metric FROM %s", RoutesTableName),
	)
	if err != nil {
		return fmt.Errorf("list routes: %w", err)
	}

	defer func() { _ = rows.Close() }()

	type routeKey struct {
		Destination string
		Gateway     string
		Interface   int
		Metric      int
	}

	existing := make(map[routeKey]int64) // key → id

	for rows.Next() {
		var id int64

		var dest, gw string

		var iface, metric int

		if err := rows.Scan(&id, &dest, &gw, &iface, &metric); err != nil {
			return fmt.Errorf("scan route: %w", err)
		}

		existing[routeKey{dest, gw, iface, metric}] = id
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate routes: %w", err)
	}

	desiredKeys := make(map[routeKey]bool, len(desired))

	for _, d := range desired {
		iface := parseNetworkInterface(d.Interface)
		key := routeKey{d.Destination, d.Gateway, int(iface), d.Metric}
		desiredKeys[key] = true

		if _, ok := existing[key]; !ok {
			_, err := tx.ExecContext(ctx,
				fmt.Sprintf("INSERT INTO %s (destination, gateway, interface, metric) VALUES (?, ?, ?, ?)", RoutesTableName),
				d.Destination, d.Gateway, int(iface), d.Metric,
			)
			if err != nil {
				return fmt.Errorf("create route %s→%s: %w", d.Destination, d.Gateway, err)
			}

			logger.DBLog.Info("Created route from fleet config",
				zap.String("destination", d.Destination),
				zap.String("gateway", d.Gateway),
			)
		}
		// Routes have no mutable fields beyond the key, so no update needed.
	}

	for key, id := range existing {
		if !desiredKeys[key] {
			_, err := tx.ExecContext(ctx,
				fmt.Sprintf("DELETE FROM %s WHERE id=?", RoutesTableName),
				id,
			)
			if err != nil {
				return fmt.Errorf("delete route id=%d: %w", id, err)
			}

			logger.DBLog.Info("Deleted route from fleet config", zap.Int64("id", id))
		}
	}

	return nil
}

// parseNetworkInterface converts a string interface name to a NetworkInterface value.
func parseNetworkInterface(s string) NetworkInterface {
	switch s {
	case "n3":
		return N3
	case "n6":
		return N6
	default:
		return N6
	}
}
