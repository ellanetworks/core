// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package apimatrix

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

type Env struct {
	Client        *client.Client
	APIAddress    string
	IPv6          bool
	N6GatewayIPv4 string
	N6GatewayIPv6 string
}

type HAEnv struct {
	Clients   []*client.Client
	Leader    *client.Client
	LeaderIdx int
	IPv6      bool
}

type Runner func(ctx context.Context, t *testing.T, env *Env)

type HARunner func(ctx context.Context, t *testing.T, h *HAEnv)

var Resources = map[string]Runner{
	"profiles":          runProfilesMatrix,
	"slices":            runSlicesMatrix,
	"data_networks":     runDataNetworksMatrix,
	"static_ips":        runStaticIpsMatrix,
	"policies":          runPoliciesMatrix,
	"subscribers":       runSubscribersMatrix,
	"routes":            runRoutesMatrix,
	"bgp":               runBGPMatrix,
	"users":             runUsersMatrix,
	"api_tokens":        runAPITokensMatrix,
	"home_network_keys": runHomeNetworkKeysMatrix,
	"policy_rules":      runPolicyRulesMatrix,
	"operator":          runOperatorMatrix,
	"nat":               runNATMatrix,
	"flow_accounting":   runFlowAccountingMatrix,
	"n3_interface":      runN3InterfaceMatrix,
	"audit_logs":        runAuditLogsMatrix,
	"status":            runStatusMatrix,
	"metrics":           runMetricsMatrix,
	"radios":            runRadiosMatrix,
	"radio_events":      runRadioEventsMatrix,
	"subscriber_usage":  runSubscriberUsageMatrix,
	"flow_reports":      runFlowReportsMatrix,
	"backup_restore":    runBackupRestoreMatrix,
	"support_bundle":    runSupportBundleMatrix,
	"location":          runLocationMatrix,
	"openapi":           runOpenAPISpecMatrix,
	"cluster_health":    runClusterHealthMatrix,
	"auth":              runAuthMatrix,
}

var HAResources = map[string]HARunner{
	"profiles":                   runProfilesHAMatrix,
	"slices":                     runSlicesHAMatrix,
	"data_networks":              runDataNetworksHAMatrix,
	"static_ips":                 runStaticIpsHAMatrix,
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
	"operator_sms":               runOperatorSMSHAMatrix,
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
	"audit_logs":                 runAuditLogsHAMatrix,
	"cluster_routing":            runClusterRoutingHAMatrix,
}
