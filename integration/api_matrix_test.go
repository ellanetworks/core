package integration_test

import (
	"context"
	"os"
	"testing"

	"github.com/ellanetworks/core/client"
)

type apiMatrixRunner func(ctx context.Context, t *testing.T, c *client.Client)

// apiMatrixResources is the registry of resources covered by TestAPIMatrix.
// Adding a new resource means implementing its runner in
// api_matrix_<resource>_test.go and registering it here.
var apiMatrixResources = map[string]apiMatrixRunner{
	"profiles":                   runProfilesMatrix,
	"slices":                     runSlicesMatrix,
	"data_networks":              runDataNetworksMatrix,
	"policies":                   runPoliciesMatrix,
	"subscribers":                runSubscribersMatrix,
	"routes":                     runRoutesMatrix,
	"bgp_peers":                  runBGPPeersMatrix,
	"users":                      runUsersMatrix,
	"api_tokens":                 runAPITokensMatrix,
	"home_network_keys":          runHomeNetworkKeysMatrix,
	"policy_rules":               runPolicyRulesMatrix,
	"operator_code":              runOperatorCodeMatrix,
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
