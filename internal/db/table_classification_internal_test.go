// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"sort"
	"testing"
)

var (
	pinnedReplicatedTables = []string{
		"api_tokens",
		"cell_positions",
		"cluster_join_hmac",
		"cluster_join_tokens",
		"cluster_members",
		"cluster_node_certs",
		"daily_usage",
		"data_networks",
		"home_network_keys",
		"ip_leases",
		"jwt_secret",
		"network_rules",
		"network_slices",
		"operator",
		"policies",
		"profiles",
		"retention_policies",
		"schema_version",
		"sessions",
		"subscriber_framed_routes",
		"subscribers",
		"users",
	}

	pinnedLocalOnlyTables = []string{
		"audit_logs",
		"bgp_import_prefixes",
		"bgp_peers",
		"bgp_settings",
		"flow_accounting_settings",
		"flow_reports",
		"local_switch_settings",
		"n3_settings",
		"nat_settings",
		"network_logs",
		"positioning_sessions",
		"routes",
	}

	pinnedFSMInternalTables = []string{
		"fsm_state",
	}

	pinnedRetiredReplicatedTables = []string{
		"audit_logs",
		"bgp_import_prefixes",
		"bgp_peers",
		"bgp_settings",
		"cluster_issued_certs",
		"cluster_pki_intermediates",
		"cluster_pki_roots",
		"cluster_pki_state",
		"cluster_revoked_certs",
		"flow_accounting_settings",
		"n3_settings",
		"nat_settings",
		"routes",
	}
)

const tableClassificationRule = `A table was added to, or removed from, a replication class.

A Raft log entry is durable and is interpreted by whichever binary
replays it, so the class a table holds today decides what every
historical changeset is allowed to write. Demoting a table to
localOnlyTables makes restoreLocalOnlyTables carry that table's rows
across a snapshot restore, and replaying a changeset captured while it
was replicated then collides with rows the node already owns — the FSM
treats the conflict as fatal and the node crash-loops on every restart.

Removing a table from replicatedChangesetTables therefore also requires
adding it to retiredReplicatedTables, so applyChangeset keeps filtering
its rows out of entries written before the demotion, and a case in
TestApplyChangeset_SkipsRetiredReplicatedTable.

Promoting a table is safe but ordered: entries written before the
promotion never carried it.`

func assertTableSetPinned(t *testing.T, label string, pinned, actual []string) {
	t.Helper()

	pinnedSet := make(map[string]struct{}, len(pinned))
	for _, table := range pinned {
		pinnedSet[table] = struct{}{}
	}

	actualSet := make(map[string]struct{}, len(actual))
	for _, table := range actual {
		actualSet[table] = struct{}{}
	}

	for _, table := range sortedKeys(pinnedSet) {
		if _, ok := actualSet[table]; !ok {
			t.Errorf("%s: %q is pinned but absent\n\n%s", label, table, tableClassificationRule)
		}
	}

	for _, table := range sortedKeys(actualSet) {
		if _, ok := pinnedSet[table]; !ok {
			t.Errorf("%s: %q is present but not pinned\n\n%s", label, table, tableClassificationRule)
		}
	}
}

func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}

	sort.Strings(out)

	return out
}

func TestReplicatedTablesArePinned(t *testing.T) {
	assertTableSetPinned(t, "replicatedChangesetTables", pinnedReplicatedTables, replicatedChangesetTables)
}

func TestLocalOnlyTablesArePinned(t *testing.T) {
	assertTableSetPinned(t, "localOnlyTables", pinnedLocalOnlyTables, localOnlyTables)
}

func TestFSMInternalTablesArePinned(t *testing.T) {
	assertTableSetPinned(t, "fsmInternalTables", pinnedFSMInternalTables, fsmInternalTables)
}

func TestRetiredReplicatedTablesArePinned(t *testing.T) {
	assertTableSetPinned(t, "retiredReplicatedTables", pinnedRetiredReplicatedTables, retiredReplicatedTables)
}

func TestRetiredTablesAreNotReplicated(t *testing.T) {
	for _, table := range retiredReplicatedTables {
		if isReplicatedTable(table) {
			t.Errorf("%q is retired but isReplicatedTable reports it as replicated\n\n%s", table, tableClassificationRule)
		}
	}
}

func TestRetiredTablesAreLocalOnlyOrDropped(t *testing.T) {
	localOnly := make(map[string]struct{}, len(localOnlyTables))
	for _, table := range localOnlyTables {
		localOnly[table] = struct{}{}
	}

	dropped := map[string]struct{}{
		"cluster_pki_roots":         {},
		"cluster_pki_intermediates": {},
		"cluster_issued_certs":      {},
		"cluster_revoked_certs":     {},
		"cluster_pki_state":         {},
	}

	for _, table := range retiredReplicatedTables {
		_, isLocal := localOnly[table]
		_, isDropped := dropped[table]

		if !isLocal && !isDropped {
			t.Errorf("retired table %q is neither local-only nor dropped\n\n%s", table, tableClassificationRule)
		}
	}
}

func TestIsReplicatedTableMatchesCaptureSet(t *testing.T) {
	for _, table := range replicatedChangesetTables {
		if !isReplicatedTable(table) {
			t.Errorf("%q is attached at capture but isReplicatedTable rejects it\n\n%s", table, tableClassificationRule)
		}
	}

	for _, table := range localOnlyTables {
		if isReplicatedTable(table) {
			t.Errorf("local-only %q is accepted by isReplicatedTable\n\n%s", table, tableClassificationRule)
		}
	}

	for _, table := range fsmInternalTables {
		if isReplicatedTable(table) {
			t.Errorf("fsm-internal %q is accepted by isReplicatedTable\n\n%s", table, tableClassificationRule)
		}
	}
}
