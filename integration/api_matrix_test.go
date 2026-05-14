package integration_test

import (
	"context"
	"os"
	"testing"

	"github.com/ellanetworks/core/client"
)

// apiMatrixRunner runs one resource's Create→Read→Update→Read→Delete→Read(404)
// matrix against an already-bootstrapped Ella Core. Each runner owns its own
// resource lifecycle (including t.Cleanup deletes) and asserts on round-trip
// field fidelity.
type apiMatrixRunner func(ctx context.Context, t *testing.T, c *client.Client)

// apiMatrixResources is the registry of resources covered by TestAPIMatrix.
// Adding a new resource means: implement its runner in
// api_matrix_<resource>_test.go and register it here.
var apiMatrixResources = map[string]apiMatrixRunner{
	// Full CRUD resources (step 3).
	"profiles":          runProfilesMatrix,
	"slices":            runSlicesMatrix,
	"data_networks":     runDataNetworksMatrix,
	"policies":          runPoliciesMatrix,
	"subscribers":       runSubscribersMatrix,
	"routes":            runRoutesMatrix,
	"bgp_peers":         runBGPPeersMatrix,
	"users":             runUsersMatrix,
	"api_tokens":        runAPITokensMatrix,
	"home_network_keys": runHomeNetworkKeysMatrix,
	// Singletons + retention policies (step 4).
	"operator_id":                runOperatorIDMatrix,
	"operator_tracking":          runOperatorTrackingMatrix,
	"operator_nas_security":      runOperatorNASSecurityMatrix,
	"operator_spn":               runOperatorSPNMatrix,
	"nat":                        runNATMatrix,
	"bgp_settings":               runBGPSettingsMatrix,
	"flow_accounting":            runFlowAccountingMatrix,
	"n3_interface":               runN3InterfaceMatrix,
	"subscriber_usage_retention": runSubscriberUsageRetentionMatrix,
	"radio_events_retention":     runRadioEventsRetentionMatrix,
	"flow_reports_retention":     runFlowReportsRetentionMatrix,
	"audit_log_retention":        runAuditLogRetentionMatrix,
}

// TestAPIMatrix exercises Create/Read/Update/Delete (and List, and the
// missing-after-delete 404) for every CRUD-capable REST resource, using the
// Go client SDK against a live Ella Core brought up via the core-tester
// compose. The bootstrap (admin user + API token + NAT + default routes) is
// reused from setupTesterEnv; no gNB/UE traffic is involved.
//
// Each resource runs as an independent t.Run subtest so failures in one
// resource don't mask others. Subtests run sequentially against a single
// compose to keep total runtime bounded.
func TestAPIMatrix(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	ctx := context.Background()
	env := setupTesterEnv(ctx, t)

	for name, run := range apiMatrixResources {
		name, run := name, run
		t.Run(name, func(t *testing.T) {
			run(ctx, t, env.Client)
		})
	}
}
