// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/fleet/client"
	"github.com/ellanetworks/core/internal/logger"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	syncOperatorStmt        = "UPDATE %s SET mcc=$Operator.mcc, mnc=$Operator.mnc, operatorCode=$Operator.operatorCode, supportedTACs=$Operator.supportedTACs, sst=$Operator.sst, sd=$Operator.sd, homeNetworkPrivateKey=$Operator.homeNetworkPrivateKey WHERE id=1"
	listAllDataNetworksStmt = "SELECT &DataNetwork.* FROM %s"
	listAllPoliciesStmt     = "SELECT &Policy.* FROM %s"
	listAllSubscribersStmt  = "SELECT &Subscriber.* FROM %s"
	listAllRoutesStmt       = "SELECT &Route.* FROM %s"
	syncSubscriberStmt      = "UPDATE %s SET permanentKey=$Subscriber.permanentKey, opc=$Subscriber.opc, policyID=$Subscriber.policyID WHERE imsi==$Subscriber.imsi"
)

// UpdateConfig applies the fleet-provided SyncConfig to the local database.
// It runs as a single sqlair transaction: for each entity type it compares the
// incoming desired state with the current state and creates, updates, or
// deletes rows as needed.
func (db *Database) UpdateConfig(ctx context.Context, cfg client.SyncConfig) error {
	ctx, span := tracer.Start(ctx, "UpdateConfig", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	tx, err := db.conn.Begin(ctx, nil)
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

	if err = syncOperator(ctx, tx, db, cfg.Operator); err != nil {
		span.RecordError(err)
		return fmt.Errorf("sync operator: %w", err)
	}

	if err = syncNATSettings(ctx, tx, db, cfg.Networking.NAT); err != nil {
		span.RecordError(err)
		return fmt.Errorf("sync NAT settings: %w", err)
	}

	if err = syncN3Settings(ctx, tx, db, cfg.Networking.NetworkInterfaces.N3ExternalAddress); err != nil {
		span.RecordError(err)
		return fmt.Errorf("sync N3 settings: %w", err)
	}

	if err = syncDataNetworks(ctx, tx, db, cfg.Networking.DataNetworks); err != nil {
		span.RecordError(err)
		return fmt.Errorf("sync data networks: %w", err)
	}

	if err = syncPolicies(ctx, tx, db, cfg.Policies, cfg.Networking.DataNetworks); err != nil {
		span.RecordError(err)
		return fmt.Errorf("sync policies: %w", err)
	}

	if err = syncSubscribers(ctx, tx, db, cfg.Subscribers, cfg.Policies); err != nil {
		span.RecordError(err)
		return fmt.Errorf("sync subscribers: %w", err)
	}

	if err = syncRoutes(ctx, tx, db, cfg.Networking.Routes); err != nil {
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
func syncOperator(ctx context.Context, tx *sqlair.TX, db *Database, desired client.Operator) error {
	supportedTACs, err := json.Marshal(desired.Tracking.SupportedTacs)
	if err != nil {
		return fmt.Errorf("marshal supported TACs: %w", err)
	}

	op := Operator{
		Mcc:                   desired.ID.Mcc,
		Mnc:                   desired.ID.Mnc,
		OperatorCode:          desired.OperatorCode,
		SupportedTACs:         string(supportedTACs),
		Sst:                   desired.Slice.Sst,
		Sd:                    desired.Slice.Sd,
		HomeNetworkPrivateKey: desired.HomeNetwork.PrivateKey,
	}

	if err := tx.Query(ctx, db.syncOperatorStmt, op).Run(); err != nil {
		return fmt.Errorf("update operator: %w", err)
	}

	return nil
}

// syncNATSettings upserts the NAT enabled flag.
func syncNATSettings(ctx context.Context, tx *sqlair.TX, db *Database, enabled bool) error {
	arg := NATSettings{Enabled: enabled}

	if err := tx.Query(ctx, db.upsertNATSettingsStmt, arg).Run(); err != nil {
		return fmt.Errorf("upsert NAT settings: %w", err)
	}

	return nil
}

// syncN3Settings upserts the N3 external address.
func syncN3Settings(ctx context.Context, tx *sqlair.TX, db *Database, externalAddress string) error {
	arg := N3Settings{ExternalAddress: externalAddress}

	if err := tx.Query(ctx, db.updateN3SettingsStmt, arg).Run(); err != nil {
		return fmt.Errorf("upsert N3 settings: %w", err)
	}

	return nil
}

// syncDataNetworks reconciles data networks: creates new, updates changed, deletes removed.
func syncDataNetworks(ctx context.Context, tx *sqlair.TX, db *Database, desired []client.DataNetwork) error {
	var existingDNs []DataNetwork

	err := tx.Query(ctx, db.listAllDataNetworksStmt).GetAll(&existingDNs)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("list data networks: %w", err)
	}

	existing := make(map[string]DataNetwork, len(existingDNs))

	for _, dn := range existingDNs {
		existing[dn.Name] = dn
	}

	desiredNames := make(map[string]bool, len(desired))

	for _, d := range desired {
		desiredNames[d.Name] = true

		if cur, ok := existing[d.Name]; ok {
			// Update if any field differs.
			if cur.IPPool != d.IPPool || cur.DNS != d.DNS || cur.MTU != d.MTU {
				dn := DataNetwork{Name: d.Name, IPPool: d.IPPool, DNS: d.DNS, MTU: d.MTU}

				if err := tx.Query(ctx, db.editDataNetworkStmt, dn).Run(); err != nil {
					return fmt.Errorf("update data network %q: %w", d.Name, err)
				}

				logger.DBLog.Info("Updated data network from fleet config", zap.String("name", d.Name))
			}
		} else {
			dn := &DataNetwork{Name: d.Name, IPPool: d.IPPool, DNS: d.DNS, MTU: d.MTU}

			if err := tx.Query(ctx, db.createDataNetworkStmt, dn).Run(); err != nil {
				return fmt.Errorf("create data network %q: %w", d.Name, err)
			}

			logger.DBLog.Info("Created data network from fleet config", zap.String("name", d.Name))
		}
	}

	// Delete data networks not in the desired set.
	for name := range existing {
		if !desiredNames[name] {
			if err := tx.Query(ctx, db.deleteDataNetworkStmt, DataNetwork{Name: name}).Run(); err != nil {
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
func syncPolicies(ctx context.Context, tx *sqlair.TX, db *Database, desired []client.Policy, fleetDataNetworks []client.DataNetwork) error {
	// Build a map from fleet data network ID to data network name.
	fleetDNIDToName := make(map[int]string, len(fleetDataNetworks))
	for _, dn := range fleetDataNetworks {
		fleetDNIDToName[dn.ID] = dn.Name
	}

	// Build a map from data network name to local DB ID.
	var existingDNs []DataNetwork

	err := tx.Query(ctx, db.listAllDataNetworksStmt).GetAll(&existingDNs)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("list data networks for policy resolution: %w", err)
	}

	localDNNameToID := make(map[string]int, len(existingDNs))

	for _, dn := range existingDNs {
		localDNNameToID[dn.Name] = dn.ID
	}

	var existingPolicies []Policy

	err = tx.Query(ctx, db.listAllPoliciesStmt).GetAll(&existingPolicies)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("list policies: %w", err)
	}

	existing := make(map[string]Policy, len(existingPolicies))

	for _, p := range existingPolicies {
		existing[p.Name] = p
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
				p := Policy{
					Name:            d.Name,
					BitrateUplink:   d.BitrateUplink,
					BitrateDownlink: d.BitrateDownlink,
					Var5qi:          d.Var5qi,
					Arp:             d.Arp,
					DataNetworkID:   localDNID,
				}

				if err := tx.Query(ctx, db.editPolicyStmt, p).Run(); err != nil {
					return fmt.Errorf("update policy %q: %w", d.Name, err)
				}

				logger.DBLog.Info("Updated policy from fleet config", zap.String("name", d.Name))
			}
		} else {
			p := &Policy{
				Name:            d.Name,
				BitrateUplink:   d.BitrateUplink,
				BitrateDownlink: d.BitrateDownlink,
				Var5qi:          d.Var5qi,
				Arp:             d.Arp,
				DataNetworkID:   localDNID,
			}

			if err := tx.Query(ctx, db.createPolicyStmt, p).Run(); err != nil {
				return fmt.Errorf("create policy %q: %w", d.Name, err)
			}

			logger.DBLog.Info("Created policy from fleet config", zap.String("name", d.Name))
		}
	}

	for name := range existing {
		if !desiredNames[name] {
			if err := tx.Query(ctx, db.deletePolicyStmt, Policy{Name: name}).Run(); err != nil {
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
func syncSubscribers(ctx context.Context, tx *sqlair.TX, db *Database, desired []client.Subscriber, fleetPolicies []client.Policy) error {
	// Build a map from fleet policy ID to policy name.
	fleetPolicyIDToName := make(map[int]string, len(fleetPolicies))
	for _, p := range fleetPolicies {
		fleetPolicyIDToName[p.ID] = p.Name
	}

	// Build a map from policy name to local DB ID.
	var existingPolicies []Policy

	err := tx.Query(ctx, db.listAllPoliciesStmt).GetAll(&existingPolicies)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("list policies for subscriber resolution: %w", err)
	}

	localPolicyNameToID := make(map[string]int, len(existingPolicies))

	for _, p := range existingPolicies {
		localPolicyNameToID[p.Name] = p.ID
	}

	var existingSubs []Subscriber

	err = tx.Query(ctx, db.listAllSubscribersStmt).GetAll(&existingSubs)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("list subscribers: %w", err)
	}

	existing := make(map[string]Subscriber, len(existingSubs))

	for _, s := range existingSubs {
		existing[s.Imsi] = s
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
				s := Subscriber{
					Imsi:         d.Imsi,
					PermanentKey: d.PermanentKey,
					Opc:          d.Opc,
					PolicyID:     localPolicyID,
				}

				if err := tx.Query(ctx, db.syncSubscriberStmt, s).Run(); err != nil {
					return fmt.Errorf("update subscriber %q: %w", d.Imsi, err)
				}

				logger.DBLog.Info("Updated subscriber from fleet config", zap.String("imsi", d.Imsi))
			}
		} else {
			s := &Subscriber{
				Imsi:           d.Imsi,
				SequenceNumber: "000000000000",
				PermanentKey:   d.PermanentKey,
				Opc:            d.Opc,
				PolicyID:       localPolicyID,
			}

			if err := tx.Query(ctx, db.createSubscriberStmt, s).Run(); err != nil {
				return fmt.Errorf("create subscriber %q: %w", d.Imsi, err)
			}

			logger.DBLog.Info("Created subscriber from fleet config", zap.String("imsi", d.Imsi))
		}
	}

	for imsi := range existing {
		if !desiredIMSIs[imsi] {
			if err := tx.Query(ctx, db.deleteSubscriberStmt, Subscriber{Imsi: imsi}).Run(); err != nil {
				return fmt.Errorf("delete subscriber %q: %w", imsi, err)
			}

			logger.DBLog.Info("Deleted subscriber from fleet config", zap.String("imsi", imsi))
		}
	}

	return nil
}

// syncRoutes reconciles routes. Routes are keyed by (destination, gateway, interface, metric)
// since they have no natural name key.
func syncRoutes(ctx context.Context, tx *sqlair.TX, db *Database, desired []client.Route) error {
	var existingRoutes []Route

	err := tx.Query(ctx, db.listAllRoutesStmt).GetAll(&existingRoutes)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("list routes: %w", err)
	}

	type routeKey struct {
		Destination string
		Gateway     string
		Interface   NetworkInterface
		Metric      int
	}

	existing := make(map[routeKey]int64, len(existingRoutes))

	for _, r := range existingRoutes {
		existing[routeKey{r.Destination, r.Gateway, r.Interface, r.Metric}] = r.ID
	}

	desiredKeys := make(map[routeKey]bool, len(desired))

	for _, d := range desired {
		iface := parseNetworkInterface(d.Interface)
		key := routeKey{d.Destination, d.Gateway, iface, d.Metric}
		desiredKeys[key] = true

		if _, ok := existing[key]; !ok {
			route := &Route{
				Destination: d.Destination,
				Gateway:     d.Gateway,
				Interface:   iface,
				Metric:      d.Metric,
			}

			if err := tx.Query(ctx, db.createRouteStmt, route).Run(); err != nil {
				return fmt.Errorf("create route %s\u2192%s: %w", d.Destination, d.Gateway, err)
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
			if err := tx.Query(ctx, db.deleteRouteStmt, Route{ID: id}).Run(); err != nil {
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
