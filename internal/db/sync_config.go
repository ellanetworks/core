// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/fleet/client"
	"github.com/ellanetworks/core/internal/logger"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// UpdateConfig applies the fleet-provided SyncConfig to the local database.
// The entire reconciliation runs inside a single changeset capture, so the
// resulting Raft log entry is one atomic replay on every follower.
//
// Operator, settings, profiles/slices/data networks, policies, subscribers,
// home network keys, and routes are all reconciled by name (or by natural
// key for routes). Fleet-side IDs are resolved to local IDs via name at
// apply time — the IDs seen here do not need to match what the local DB
// last assigned.
func (db *Database) UpdateConfig(ctx context.Context, cfg client.SyncConfig) error {
	_, span := tracer.Start(ctx, "UpdateConfig", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	_, err := db.proposeChangeset(func(ctx context.Context) (any, error) {
		return nil, db.applySyncConfig(ctx, &cfg)
	}, "UpdateConfig")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) applySyncConfig(ctx context.Context, cfg *client.SyncConfig) error {
	if err := db.syncOperator(ctx, cfg.Operator); err != nil {
		return fmt.Errorf("sync operator: %w", err)
	}

	if err := db.syncHomeNetworkKeys(ctx, cfg.HomeNetworkKeys); err != nil {
		return fmt.Errorf("sync home network keys: %w", err)
	}

	if err := db.syncDataNetworks(ctx, cfg.Networking.DataNetworks); err != nil {
		return fmt.Errorf("sync data networks: %w", err)
	}

	if err := db.syncProfiles(ctx, cfg.Profiles); err != nil {
		return fmt.Errorf("sync profiles: %w", err)
	}

	if err := db.syncSlices(ctx, cfg.Slices); err != nil {
		return fmt.Errorf("sync slices: %w", err)
	}

	// Subscribers reference profiles; policies reference profiles, slices,
	// and data networks. Delete-then-upsert both to survive FK constraints
	// during rename-style reconciliations.
	if err := db.syncSubscribersDeletes(ctx, cfg.Subscribers); err != nil {
		return fmt.Errorf("sync subscribers (deletes): %w", err)
	}

	if err := db.syncPoliciesDeletes(ctx, cfg.Policies); err != nil {
		return fmt.Errorf("sync policies (deletes): %w", err)
	}

	if err := db.syncPoliciesUpserts(ctx, cfg.Policies); err != nil {
		return fmt.Errorf("sync policies (upserts): %w", err)
	}

	if err := db.syncSubscribersUpserts(ctx, cfg.Subscribers); err != nil {
		return fmt.Errorf("sync subscribers (upserts): %w", err)
	}

	if err := db.syncProfilesDeletes(ctx, cfg.Profiles); err != nil {
		return fmt.Errorf("sync profiles (deletes): %w", err)
	}

	if err := db.syncSlicesDeletes(ctx, cfg.Slices); err != nil {
		return fmt.Errorf("sync slices (deletes): %w", err)
	}

	if err := db.syncDataNetworksDeletes(ctx, cfg.Networking.DataNetworks); err != nil {
		return fmt.Errorf("sync data networks (deletes): %w", err)
	}

	if err := db.syncRoutes(ctx, cfg.Networking.Routes); err != nil {
		return fmt.Errorf("sync routes: %w", err)
	}

	if err := db.syncNetworkRules(ctx, cfg.NetworkRules); err != nil {
		return fmt.Errorf("sync network rules: %w", err)
	}

	if err := db.syncBGPSettings(ctx, cfg.Networking.BGP); err != nil {
		return fmt.Errorf("sync BGP settings: %w", err)
	}

	if err := db.syncBGPPeersAndPrefixes(ctx, cfg.Networking.BGPPeers, cfg.Networking.BGPImportPrefixes); err != nil {
		return fmt.Errorf("sync BGP peers: %w", err)
	}

	if err := db.syncRetentionPolicies(ctx, cfg.RetentionPolicies); err != nil {
		return fmt.Errorf("sync retention policies: %w", err)
	}

	if _, err := db.applyUpdateNATSettings(ctx, &boolPayload{Value: cfg.Networking.NAT}); err != nil {
		return fmt.Errorf("sync NAT: %w", err)
	}

	if _, err := db.applyUpdateFlowAccountingSettings(ctx, &boolPayload{Value: cfg.Networking.FlowAccounting}); err != nil {
		return fmt.Errorf("sync flow accounting: %w", err)
	}

	if _, err := db.applyUpdateN3Settings(ctx, &stringPayload{Value: cfg.Networking.NetworkInterfaces.N3ExternalAddress}); err != nil {
		return fmt.Errorf("sync N3 settings: %w", err)
	}

	return nil
}

// syncOperator updates the singleton operator row in place. Cluster ID
// and AMF identity remain local-only — Fleet does not manage them.
func (db *Database) syncOperator(ctx context.Context, desired client.Operator) error {
	if _, err := db.applyUpdateOperatorID(ctx, &Operator{Mcc: desired.ID.Mcc, Mnc: desired.ID.Mnc}); err != nil {
		return fmt.Errorf("update operator id: %w", err)
	}

	if _, err := db.applyUpdateOperatorCode(ctx, &Operator{OperatorCode: desired.OperatorCode}); err != nil {
		return fmt.Errorf("update operator code: %w", err)
	}

	tacJSON, err := json.Marshal(desired.Tracking.SupportedTacs)
	if err != nil {
		return fmt.Errorf("marshal supported TACs: %w", err)
	}

	if _, err := db.applyUpdateOperatorTracking(ctx, &Operator{SupportedTACs: string(tacJSON)}); err != nil {
		return fmt.Errorf("update operator tracking: %w", err)
	}

	ciphering, err := json.Marshal(desired.NASSecurity.Ciphering)
	if err != nil {
		return fmt.Errorf("marshal ciphering: %w", err)
	}

	integrity, err := json.Marshal(desired.NASSecurity.Integrity)
	if err != nil {
		return fmt.Errorf("marshal integrity: %w", err)
	}

	if _, err := db.applyUpdateOperatorSecurityAlgorithms(ctx, &Operator{Ciphering: string(ciphering), Integrity: string(integrity)}); err != nil {
		return fmt.Errorf("update operator NAS security: %w", err)
	}

	if _, err := db.applyUpdateOperatorSPN(ctx, &Operator{SpnFullName: desired.SPN.FullName, SpnShortName: desired.SPN.ShortName}); err != nil {
		return fmt.Errorf("update operator SPN: %w", err)
	}

	if _, err := db.applyUpdateOperatorAMFIdentity(ctx, &Operator{AmfRegionID: desired.AMF.RegionID, AmfSetID: desired.AMF.SetID}); err != nil {
		return fmt.Errorf("update operator AMF identity: %w", err)
	}

	return nil
}

// syncHomeNetworkKeys reconciles home network keys by keyIdentifier.
// Existing keys are left untouched (private keys are not rotated via
// Fleet); missing keys are created and extras are deleted.
func (db *Database) syncHomeNetworkKeys(ctx context.Context, desired []client.HomeNetworkKey) error {
	existing, err := db.listHomeNetworkKeysPinned(ctx)
	if err != nil {
		return err
	}

	have := make(map[int]bool, len(existing))
	for _, k := range existing {
		have[k.KeyIdentifier] = true
	}

	want := make(map[int]bool, len(desired))

	for _, k := range desired {
		want[k.KeyIdentifier] = true

		if have[k.KeyIdentifier] {
			continue
		}

		newKey := &HomeNetworkKey{
			KeyIdentifier: k.KeyIdentifier,
			Scheme:        k.Scheme,
			PrivateKey:    k.PrivateKey,
		}

		if _, err := db.applyCreateHomeNetworkKey(ctx, newKey); err != nil {
			return fmt.Errorf("create home network key %d: %w", k.KeyIdentifier, err)
		}

		logger.DBLog.Info("Created home network key from fleet config", zap.Int("key_identifier", k.KeyIdentifier))
	}

	for _, k := range existing {
		if want[k.KeyIdentifier] {
			continue
		}

		if _, err := db.applyDeleteHomeNetworkKey(ctx, &intPayload{Value: k.KeyIdentifier}); err != nil {
			return fmt.Errorf("delete home network key %d: %w", k.KeyIdentifier, err)
		}

		logger.DBLog.Info("Deleted home network key from fleet config", zap.Int("key_identifier", k.KeyIdentifier))
	}

	return nil
}

// syncDataNetworks upserts desired data networks. Deletions are deferred
// to syncDataNetworksDeletes so policies referencing a DN can be rewired
// or removed first.
func (db *Database) syncDataNetworks(ctx context.Context, desired []client.DataNetwork) error {
	existing, err := db.listAllDataNetworksPinned(ctx)
	if err != nil {
		return err
	}

	have := make(map[string]DataNetwork, len(existing))
	for _, dn := range existing {
		have[dn.Name] = dn
	}

	for _, d := range desired {
		want := DataNetwork{Name: d.Name, IPPool: d.IPPool, DNS: d.DNS, MTU: d.MTU}

		cur, ok := have[d.Name]
		if !ok {
			if _, err := db.applyCreateDataNetwork(ctx, &want); err != nil {
				return fmt.Errorf("create data network %q: %w", d.Name, err)
			}

			logger.DBLog.Info("Created data network from fleet config", zap.String("name", d.Name))

			continue
		}

		if cur.IPPool != d.IPPool || cur.DNS != d.DNS || cur.MTU != d.MTU {
			if _, err := db.applyUpdateDataNetwork(ctx, &want); err != nil {
				return fmt.Errorf("update data network %q: %w", d.Name, err)
			}

			logger.DBLog.Info("Updated data network from fleet config", zap.String("name", d.Name))
		}
	}

	return nil
}

func (db *Database) syncDataNetworksDeletes(ctx context.Context, desired []client.DataNetwork) error {
	existing, err := db.listAllDataNetworksPinned(ctx)
	if err != nil {
		return err
	}

	want := make(map[string]bool, len(desired))
	for _, d := range desired {
		want[d.Name] = true
	}

	for _, cur := range existing {
		if want[cur.Name] {
			continue
		}

		if _, err := db.applyDeleteDataNetwork(ctx, &stringPayload{Value: cur.Name}); err != nil {
			return fmt.Errorf("delete data network %q: %w", cur.Name, err)
		}

		logger.DBLog.Info("Deleted data network from fleet config", zap.String("name", cur.Name))
	}

	return nil
}

func (db *Database) syncProfiles(ctx context.Context, desired []client.Profile) error {
	existing, err := db.listAllProfilesPinned(ctx)
	if err != nil {
		return err
	}

	have := make(map[string]Profile, len(existing))
	for _, p := range existing {
		have[p.Name] = p
	}

	for _, d := range desired {
		want := Profile{Name: d.Name, UeAmbrUplink: d.UeAmbrUplink, UeAmbrDownlink: d.UeAmbrDownlink}

		cur, ok := have[d.Name]
		if !ok {
			if _, err := db.applyCreateProfile(ctx, &want); err != nil {
				return fmt.Errorf("create profile %q: %w", d.Name, err)
			}

			logger.DBLog.Info("Created profile from fleet config", zap.String("name", d.Name))

			continue
		}

		if cur.UeAmbrUplink != d.UeAmbrUplink || cur.UeAmbrDownlink != d.UeAmbrDownlink {
			if _, err := db.applyUpdateProfile(ctx, &want); err != nil {
				return fmt.Errorf("update profile %q: %w", d.Name, err)
			}

			logger.DBLog.Info("Updated profile from fleet config", zap.String("name", d.Name))
		}
	}

	return nil
}

func (db *Database) syncProfilesDeletes(ctx context.Context, desired []client.Profile) error {
	existing, err := db.listAllProfilesPinned(ctx)
	if err != nil {
		return err
	}

	want := make(map[string]bool, len(desired))
	for _, d := range desired {
		want[d.Name] = true
	}

	for _, cur := range existing {
		if want[cur.Name] {
			continue
		}

		if _, err := db.applyDeleteProfile(ctx, &stringPayload{Value: cur.Name}); err != nil {
			return fmt.Errorf("delete profile %q: %w", cur.Name, err)
		}

		logger.DBLog.Info("Deleted profile from fleet config", zap.String("name", cur.Name))
	}

	return nil
}

func (db *Database) syncSlices(ctx context.Context, desired []client.Slice) error {
	existing, err := db.listAllSlicesPinned(ctx)
	if err != nil {
		return err
	}

	have := make(map[string]NetworkSlice, len(existing))
	for _, s := range existing {
		have[s.Name] = s
	}

	for _, d := range desired {
		want := NetworkSlice{Name: d.Name, Sst: d.Sst, Sd: d.Sd}

		cur, ok := have[d.Name]
		if !ok {
			if _, err := db.applyCreateNetworkSlice(ctx, &want); err != nil {
				return fmt.Errorf("create slice %q: %w", d.Name, err)
			}

			logger.DBLog.Info("Created slice from fleet config", zap.String("name", d.Name))

			continue
		}

		if !slicesEqual(cur, want) {
			if _, err := db.applyUpdateNetworkSlice(ctx, &want); err != nil {
				return fmt.Errorf("update slice %q: %w", d.Name, err)
			}

			logger.DBLog.Info("Updated slice from fleet config", zap.String("name", d.Name))
		}
	}

	return nil
}

func (db *Database) syncSlicesDeletes(ctx context.Context, desired []client.Slice) error {
	existing, err := db.listAllSlicesPinned(ctx)
	if err != nil {
		return err
	}

	want := make(map[string]bool, len(desired))
	for _, d := range desired {
		want[d.Name] = true
	}

	for _, cur := range existing {
		if want[cur.Name] {
			continue
		}

		if _, err := db.applyDeleteNetworkSlice(ctx, &stringPayload{Value: cur.Name}); err != nil {
			return fmt.Errorf("delete slice %q: %w", cur.Name, err)
		}

		logger.DBLog.Info("Deleted slice from fleet config", zap.String("name", cur.Name))
	}

	return nil
}

func slicesEqual(a NetworkSlice, b NetworkSlice) bool {
	if a.Sst != b.Sst {
		return false
	}

	if (a.Sd == nil) != (b.Sd == nil) {
		return false
	}

	if a.Sd != nil && *a.Sd != *b.Sd {
		return false
	}

	return true
}

func (db *Database) syncPoliciesDeletes(ctx context.Context, desired []client.Policy) error {
	existing, err := db.listAllPoliciesPinned(ctx)
	if err != nil {
		return err
	}

	want := make(map[string]bool, len(desired))
	for _, d := range desired {
		want[d.Name] = true
	}

	for _, cur := range existing {
		if want[cur.Name] {
			continue
		}

		if _, err := db.applyDeletePolicy(ctx, &stringPayload{Value: cur.Name}); err != nil {
			return fmt.Errorf("delete policy %q: %w", cur.Name, err)
		}

		logger.DBLog.Info("Deleted policy from fleet config", zap.String("name", cur.Name))
	}

	return nil
}

// syncPoliciesUpserts resolves fleet policy references by name and
// upserts local rows. Profile/slice/data-network rows for referenced
// names must already exist locally (they are synced earlier).
func (db *Database) syncPoliciesUpserts(ctx context.Context, desired []client.Policy) error {
	profiles, err := db.listAllProfilesPinned(ctx)
	if err != nil {
		return err
	}

	profileID := make(map[string]int, len(profiles))
	for _, p := range profiles {
		profileID[p.Name] = p.ID
	}

	slices, err := db.listAllSlicesPinned(ctx)
	if err != nil {
		return err
	}

	sliceID := make(map[string]int, len(slices))
	for _, s := range slices {
		sliceID[s.Name] = s.ID
	}

	dataNetworks, err := db.listAllDataNetworksPinned(ctx)
	if err != nil {
		return err
	}

	dnID := make(map[string]int, len(dataNetworks))
	for _, dn := range dataNetworks {
		dnID[dn.Name] = dn.ID
	}

	existing, err := db.listAllPoliciesPinned(ctx)
	if err != nil {
		return err
	}

	have := make(map[string]Policy, len(existing))
	for _, p := range existing {
		have[p.Name] = p
	}

	for _, d := range desired {
		pid, ok := profileID[d.ProfileName]
		if !ok {
			return fmt.Errorf("policy %q references unknown profile %q", d.Name, d.ProfileName)
		}

		slid, ok := sliceID[d.SliceName]
		if !ok {
			return fmt.Errorf("policy %q references unknown slice %q", d.Name, d.SliceName)
		}

		did, ok := dnID[d.DataNetworkName]
		if !ok {
			return fmt.Errorf("policy %q references unknown data network %q", d.Name, d.DataNetworkName)
		}

		want := Policy{
			Name:                d.Name,
			ProfileID:           pid,
			SliceID:             slid,
			DataNetworkID:       did,
			Var5qi:              d.Var5qi,
			Arp:                 d.Arp,
			SessionAmbrUplink:   d.SessionAmbrUplink,
			SessionAmbrDownlink: d.SessionAmbrDownlink,
		}

		cur, ok := have[d.Name]
		if !ok {
			if _, err := db.applyCreatePolicy(ctx, &want); err != nil {
				return fmt.Errorf("create policy %q: %w", d.Name, err)
			}

			logger.DBLog.Info("Created policy from fleet config", zap.String("name", d.Name))

			continue
		}

		if policiesEqual(cur, want) {
			continue
		}

		if _, err := db.applyUpdatePolicy(ctx, &want); err != nil {
			return fmt.Errorf("update policy %q: %w", d.Name, err)
		}

		logger.DBLog.Info("Updated policy from fleet config", zap.String("name", d.Name))
	}

	return nil
}

func policiesEqual(a, b Policy) bool {
	return a.ProfileID == b.ProfileID &&
		a.SliceID == b.SliceID &&
		a.DataNetworkID == b.DataNetworkID &&
		a.Var5qi == b.Var5qi &&
		a.Arp == b.Arp &&
		a.SessionAmbrUplink == b.SessionAmbrUplink &&
		a.SessionAmbrDownlink == b.SessionAmbrDownlink
}

func (db *Database) syncSubscribersDeletes(ctx context.Context, desired []client.Subscriber) error {
	existing, err := db.listAllSubscribersPinned(ctx)
	if err != nil {
		return err
	}

	want := make(map[string]bool, len(desired))
	for _, d := range desired {
		want[d.Imsi] = true
	}

	for _, cur := range existing {
		if want[cur.Imsi] {
			continue
		}

		if _, err := db.applyDeleteSubscriber(ctx, &stringPayload{Value: cur.Imsi}); err != nil {
			return fmt.Errorf("delete subscriber %q: %w", cur.Imsi, err)
		}

		logger.DBLog.Info("Deleted subscriber from fleet config", zap.String("imsi", cur.Imsi))
	}

	return nil
}

// syncSubscribersUpserts creates new subscribers and re-homes existing ones
// to a different profile. Permanent key / OPC / sequence number are local
// secrets and are never overwritten once the row exists.
func (db *Database) syncSubscribersUpserts(ctx context.Context, desired []client.Subscriber) error {
	profiles, err := db.listAllProfilesPinned(ctx)
	if err != nil {
		return err
	}

	profileID := make(map[string]int, len(profiles))
	for _, p := range profiles {
		profileID[p.Name] = p.ID
	}

	existing, err := db.listAllSubscribersPinned(ctx)
	if err != nil {
		return err
	}

	have := make(map[string]Subscriber, len(existing))
	for _, s := range existing {
		have[s.Imsi] = s
	}

	for _, d := range desired {
		pid, ok := profileID[d.ProfileName]
		if !ok {
			return fmt.Errorf("subscriber %q references unknown profile %q", d.Imsi, d.ProfileName)
		}

		cur, exists := have[d.Imsi]
		if !exists {
			sub := &Subscriber{
				Imsi:           d.Imsi,
				SequenceNumber: d.SequenceNumber,
				PermanentKey:   d.PermanentKey,
				Opc:            d.Opc,
				ProfileID:      pid,
			}

			if _, err := db.applyCreateSubscriber(ctx, sub); err != nil {
				return fmt.Errorf("create subscriber %q: %w", d.Imsi, err)
			}

			logger.DBLog.Info("Created subscriber from fleet config", zap.String("imsi", d.Imsi))

			continue
		}

		if cur.ProfileID == pid {
			continue
		}

		if _, err := db.applyUpdateSubscriberProfile(ctx, &Subscriber{Imsi: d.Imsi, ProfileID: pid}); err != nil {
			return fmt.Errorf("update subscriber %q profile: %w", d.Imsi, err)
		}

		logger.DBLog.Info("Updated subscriber profile from fleet config", zap.String("imsi", d.Imsi))
	}

	return nil
}

// syncRoutes reconciles routes by their natural key (destination, gateway,
// interface, metric) since they have no stable ID from Fleet's side.
func (db *Database) syncRoutes(ctx context.Context, desired []client.Route) error {
	existing, err := db.listAllRoutesPinned(ctx)
	if err != nil {
		return err
	}

	type routeKey struct {
		Destination string
		Gateway     string
		Interface   NetworkInterface
		Metric      int
	}

	have := make(map[routeKey]int64, len(existing))
	for _, r := range existing {
		have[routeKey{r.Destination, r.Gateway, r.Interface, r.Metric}] = r.ID
	}

	want := make(map[routeKey]bool, len(desired))

	for _, d := range desired {
		iface := parseFleetNetworkInterface(d.Interface)
		key := routeKey{d.Destination, d.Gateway, iface, d.Metric}
		want[key] = true

		if _, ok := have[key]; ok {
			continue
		}

		r := &Route{
			Destination: d.Destination,
			Gateway:     d.Gateway,
			Interface:   iface,
			Metric:      d.Metric,
		}

		if _, err := db.applyCreateRoute(ctx, r); err != nil {
			return fmt.Errorf("create route %s→%s: %w", d.Destination, d.Gateway, err)
		}

		logger.DBLog.Info("Created route from fleet config",
			zap.String("destination", d.Destination),
			zap.String("gateway", d.Gateway),
		)
	}

	for key, id := range have {
		if want[key] {
			continue
		}

		if _, err := db.applyDeleteRoute(ctx, &int64Payload{Value: id}); err != nil {
			return fmt.Errorf("delete route id=%d: %w", id, err)
		}

		logger.DBLog.Info("Deleted route from fleet config", zap.Int64("id", id))
	}

	return nil
}

func parseFleetNetworkInterface(s string) NetworkInterface {
	switch s {
	case "n3":
		return N3
	case "n6":
		return N6
	default:
		return N6
	}
}

// --- pinned-runner list helpers ---
//
// These run under the pinned connection during changeset capture so the
// reads see the uncommitted writes made earlier in the same sync. Outside
// capture they fall through to the shared connection. They intentionally
// bypass tracing/metrics since the caller (applySyncConfig) already spans
// the whole operation.

func (db *Database) listAllDataNetworksPinned(ctx context.Context) ([]DataNetwork, error) {
	var rows []DataNetwork

	err := db.runner(ctx).Query(ctx, db.listAllDataNetworksStmt).GetAll(&rows)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("list data networks: %w", err)
	}

	return rows, nil
}

func (db *Database) listAllProfilesPinned(ctx context.Context) ([]Profile, error) {
	// No prepared listAllProfilesStmt exists yet; reuse page statement.
	var rows []Profile

	args := ListArgs{Limit: 1_000_000, Offset: 0}

	var counts []NumItems

	err := db.runner(ctx).Query(ctx, db.listProfilesStmt, args).GetAll(&rows, &counts)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("list profiles: %w", err)
	}

	return rows, nil
}

func (db *Database) listAllSlicesPinned(ctx context.Context) ([]NetworkSlice, error) {
	var rows []NetworkSlice

	err := db.runner(ctx).Query(ctx, db.listAllNetworkSlicesStmt).GetAll(&rows)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("list slices: %w", err)
	}

	return rows, nil
}

func (db *Database) listAllPoliciesPinned(ctx context.Context) ([]Policy, error) {
	var rows []Policy

	args := ListArgs{Limit: 1_000_000, Offset: 0}

	var counts []NumItems

	err := db.runner(ctx).Query(ctx, db.listPoliciesStmt, args).GetAll(&rows, &counts)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("list policies: %w", err)
	}

	return rows, nil
}

func (db *Database) listAllSubscribersPinned(ctx context.Context) ([]Subscriber, error) {
	var rows []Subscriber

	args := ListArgs{Limit: 1_000_000, Offset: 0}

	var counts []NumItems

	err := db.runner(ctx).Query(ctx, db.listSubscribersStmt, args).GetAll(&rows, &counts)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("list subscribers: %w", err)
	}

	return rows, nil
}

func (db *Database) listAllRoutesPinned(ctx context.Context) ([]Route, error) {
	var rows []Route

	args := ListArgs{Limit: 1_000_000, Offset: 0}

	var counts []NumItems

	err := db.runner(ctx).Query(ctx, db.listRoutesStmt, args).GetAll(&rows, &counts)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("list routes: %w", err)
	}

	return rows, nil
}

func (db *Database) listHomeNetworkKeysPinned(ctx context.Context) ([]HomeNetworkKey, error) {
	var rows []HomeNetworkKey

	err := db.runner(ctx).Query(ctx, db.listHomeNetworkKeysStmt).GetAll(&rows)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("list home network keys: %w", err)
	}

	return rows, nil
}

func (db *Database) listAllBGPPeersPinned(ctx context.Context) ([]BGPPeer, error) {
	var rows []BGPPeer

	err := db.runner(ctx).Query(ctx, db.listAllBGPPeersStmt).GetAll(&rows)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("list BGP peers: %w", err)
	}

	return rows, nil
}

func (db *Database) listImportPrefixesByPeerPinned(ctx context.Context, peerID int) ([]BGPImportPrefix, error) {
	var rows []BGPImportPrefix

	err := db.runner(ctx).Query(ctx, db.listImportPrefixesByPeerStmt, BGPImportPrefix{PeerID: peerID}).GetAll(&rows)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("list import prefixes for peer %d: %w", peerID, err)
	}

	return rows, nil
}

// syncNetworkRules reconciles per-policy filter rules. For each policy in
// the local DB we delete every existing rule, then re-create the desired
// rules in precedence order — matching the API's delete-all-then-create
// semantics so there is one consistent apply path.
func (db *Database) syncNetworkRules(ctx context.Context, desired []client.NetworkRule) error {
	policies, err := db.listAllPoliciesPinned(ctx)
	if err != nil {
		return err
	}

	policyIDByName := make(map[string]int64, len(policies))
	for _, p := range policies {
		policyIDByName[p.Name] = int64(p.ID)
	}

	byPolicy := make(map[string][]client.NetworkRule, len(desired))

	for _, r := range desired {
		byPolicy[r.PolicyName] = append(byPolicy[r.PolicyName], r)
	}

	for _, p := range policies {
		if _, err := db.applyDeleteNetworkRulesByPolicy(ctx, &int64Payload{Value: int64(p.ID)}); err != nil {
			return fmt.Errorf("delete rules for policy %q: %w", p.Name, err)
		}
	}

	for policyName, rules := range byPolicy {
		policyID, ok := policyIDByName[policyName]
		if !ok {
			return fmt.Errorf("network rule references unknown policy %q", policyName)
		}

		for _, r := range rules {
			dbRule := &NetworkRule{
				PolicyID:     policyID,
				Description:  r.Description,
				Direction:    r.Direction,
				RemotePrefix: r.RemotePrefix,
				Protocol:     r.Protocol,
				PortLow:      r.PortLow,
				PortHigh:     r.PortHigh,
				Action:       r.Action,
				Precedence:   r.Precedence,
			}

			if _, err := db.applyCreateNetworkRule(ctx, dbRule); err != nil {
				return fmt.Errorf("create rule for policy %q (precedence %d): %w", policyName, r.Precedence, err)
			}
		}
	}

	return nil
}

func (db *Database) syncBGPSettings(ctx context.Context, desired client.BGPSettings) error {
	s := &BGPSettings{
		Enabled:       desired.Enabled,
		LocalAS:       desired.LocalAS,
		RouterID:      desired.RouterID,
		ListenAddress: desired.ListenAddress,
	}

	if _, err := db.applyUpdateBGPSettings(ctx, s); err != nil {
		return fmt.Errorf("update BGP settings: %w", err)
	}

	return nil
}

// syncBGPPeersAndPrefixes reconciles cluster-wide BGP peers (nodeID NULL)
// against the desired set. Node-local peers are left untouched — Fleet
// does not manage per-node BGP state. Import prefixes are grouped by
// peer address and replaced atomically per peer.
func (db *Database) syncBGPPeersAndPrefixes(ctx context.Context, desiredPeers []client.BGPPeer, desiredPrefixes []client.BGPImportPrefix) error {
	existing, err := db.listAllBGPPeersPinned(ctx)
	if err != nil {
		return err
	}

	have := make(map[string]BGPPeer, len(existing))
	for _, p := range existing {
		if p.NodeID != nil {
			continue
		}

		have[p.Address] = p
	}

	want := make(map[string]client.BGPPeer, len(desiredPeers))
	for _, p := range desiredPeers {
		want[p.Address] = p
	}

	for addr, cur := range have {
		if _, ok := want[addr]; ok {
			continue
		}

		if _, err := db.applyDeleteBGPPeer(ctx, &intPayload{Value: cur.ID}); err != nil {
			return fmt.Errorf("delete BGP peer %s: %w", addr, err)
		}
	}

	for _, d := range desiredPeers {
		cur, ok := have[d.Address]
		if !ok {
			newPeer := &BGPPeer{
				Address:     d.Address,
				RemoteAS:    d.RemoteAS,
				HoldTime:    d.HoldTime,
				Password:    d.Password,
				Description: d.Description,
			}

			if _, err := db.applyCreateBGPPeer(ctx, newPeer); err != nil {
				return fmt.Errorf("create BGP peer %s: %w", d.Address, err)
			}

			continue
		}

		if bgpPeerEqual(cur, d) {
			continue
		}

		upd := &BGPPeer{
			ID:          cur.ID,
			Address:     d.Address,
			RemoteAS:    d.RemoteAS,
			HoldTime:    d.HoldTime,
			Password:    d.Password,
			Description: d.Description,
		}

		if _, err := db.applyUpdateBGPPeer(ctx, upd); err != nil {
			return fmt.Errorf("update BGP peer %s: %w", d.Address, err)
		}
	}

	// Re-read peers so newly-created rows have their assigned IDs.
	refreshed, err := db.listAllBGPPeersPinned(ctx)
	if err != nil {
		return err
	}

	peerIDByAddress := make(map[string]int, len(refreshed))

	for _, p := range refreshed {
		if p.NodeID != nil {
			continue
		}

		peerIDByAddress[p.Address] = p.ID
	}

	prefixesByPeer := make(map[string][]BGPImportPrefix, len(desiredPrefixes))
	for _, pr := range desiredPrefixes {
		prefixesByPeer[pr.PeerAddress] = append(prefixesByPeer[pr.PeerAddress], BGPImportPrefix{
			Prefix:    pr.Prefix,
			MaxLength: pr.MaxLength,
		})
	}

	// Replace prefixes for every cluster-wide peer (desired ones get the
	// new list; peers with no entries get an empty list, clearing stale
	// prefixes).
	for addr, peerID := range peerIDByAddress {
		if err := db.replacePeerImportPrefixes(ctx, peerID, prefixesByPeer[addr]); err != nil {
			return fmt.Errorf("replace import prefixes for peer %s: %w", addr, err)
		}
	}

	for addr := range prefixesByPeer {
		if _, ok := peerIDByAddress[addr]; !ok {
			return fmt.Errorf("import prefix references unknown BGP peer %q", addr)
		}
	}

	return nil
}

// replacePeerImportPrefixes re-uses the existing delete-then-insert
// apply pattern (applySetImportPrefixesForPeer) to atomically swap a
// peer's import prefixes.
func (db *Database) replacePeerImportPrefixes(ctx context.Context, peerID int, prefixes []BGPImportPrefix) error {
	// Short-circuit when neither the DB nor the desired list has any
	// prefixes for this peer; avoids a pointless DELETE.
	current, err := db.listImportPrefixesByPeerPinned(ctx, peerID)
	if err != nil {
		return err
	}

	if len(current) == 0 && len(prefixes) == 0 {
		return nil
	}

	_, err = db.applySetImportPrefixesForPeer(ctx, &importPrefixesPayload{
		PeerID:   peerID,
		Prefixes: prefixes,
	})

	return err
}

func bgpPeerEqual(cur BGPPeer, want client.BGPPeer) bool {
	return cur.RemoteAS == want.RemoteAS &&
		cur.HoldTime == want.HoldTime &&
		cur.Password == want.Password &&
		cur.Description == want.Description
}

func (db *Database) syncRetentionPolicies(ctx context.Context, desired []client.RetentionPolicy) error {
	for _, rp := range desired {
		policy := &RetentionPolicy{
			Category: RetentionCategory(rp.Category),
			Days:     rp.Days,
		}

		if _, err := db.applySetRetentionPolicy(ctx, policy); err != nil {
			return fmt.Errorf("set retention policy %s: %w", rp.Category, err)
		}
	}

	return nil
}
