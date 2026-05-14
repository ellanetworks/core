package integration_test

import (
	"context"
	"os"
	"testing"
)

type apiMatrixHARunner func(ctx context.Context, t *testing.T, h *haMatrixEnv)

// apiMatrixHAResources mirrors apiMatrixResources but each runner sees
// the full 3-node cluster and is responsible for distributing writes
// across nodes plus asserting the cross-node invariant (replication for
// shared resources, locality for per-node resources).
var apiMatrixHAResources = map[string]apiMatrixHARunner{
	"profiles":                   runProfilesHAMatrix,
	"slices":                     runSlicesHAMatrix,
	"data_networks":              runDataNetworksHAMatrix,
	"policies":                   runPoliciesHAMatrix,
	"policy_rules":               runPolicyRulesHAMatrix,
	"subscribers":                runSubscribersHAMatrix,
	"users":                      runUsersHAMatrix,
	"api_tokens":                 runAPITokensHAMatrix,
	"home_network_keys":          runHomeNetworkKeysHAMatrix,
	"operator_id":                runOperatorIDHAMatrix,
	"operator_code":              runOperatorCodeHAMatrix,
	"operator_tracking":          runOperatorTrackingHAMatrix,
	"operator_nas_security":      runOperatorNASSecurityHAMatrix,
	"operator_spn":               runOperatorSPNHAMatrix,
	"subscriber_usage_retention": runSubscriberUsageRetentionHAMatrix,
	"radio_events_retention":     runRadioEventsRetentionHAMatrix,
	"flow_reports_retention":     runFlowReportsRetentionHAMatrix,
	"audit_log_retention":        runAuditLogRetentionHAMatrix,
	"routes":                     runRoutesHAMatrix,
	"bgp_peers":                  runBGPPeersHAMatrix,
	"bgp_settings":               runBGPSettingsHAMatrix,
	"n3_interface":               runN3InterfaceHAMatrix,
	"nat":                        runNATHAMatrix,
	"flow_accounting":            runFlowAccountingHAMatrix,
}

func TestAPIMatrixHA(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	ctx := context.Background()
	h := setupHAMatrixEnv(ctx, t)

	for name, run := range apiMatrixHAResources {
		name, run := name, run
		t.Run(name, func(t *testing.T) {
			run(ctx, t, h)
		})
	}
}
