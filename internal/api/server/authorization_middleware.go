package server

import (
	"errors"
	"net/http"

	"github.com/ellanetworks/core/internal/logger"
)

type RoleID int

const (
	RoleAdmin          RoleID = 1
	RoleReadOnly       RoleID = 2
	RoleNetworkManager RoleID = 3
)

var PermissionsByRole = map[RoleID][]string{
	RoleAdmin: {"*"},

	RoleReadOnly: {
		PermReadMyUser, PermUpdateMyUserPassword,
		PermReadOperator, PermGetOperatorSlice, PermGetOperatorTracking,
		PermListSubscribers, PermReadSubscriber,
		PermListDataNetworks, PermReadDataNetwork,
		PermListPolicies, PermReadPolicy,
		PermListRoutes, PermReadRoute,
		PermListRadios, PermReadRadio,
		PermGetNATInfo,
		PermGetFlowAccountingInfo,
		PermGetSubscriberUsageRetentionPolicy, PermGetSubscriberUsage,
		PermListRadioEvents, PermGetRadioEventRetentionPolicy, PermGetRadioEvent,
		PermGetFlowReportsRetentionPolicy, PermListFlowReports,
	},

	RoleNetworkManager: {
		PermReadUser, PermReadMyUser, PermUpdateMyUserPassword,
		PermReadOperator, PermUpdateOperatorSlice, PermGetOperatorSlice, PermUpdateOperatorTracking, PermGetOperatorTracking,
		PermListDataNetworks, PermCreateDataNetwork, PermUpdateDataNetwork, PermReadDataNetwork, PermDeleteDataNetwork,
		PermListSubscribers, PermCreateSubscriber, PermUpdateSubscriber, PermReadSubscriber, PermDeleteSubscriber,
		PermListPolicies, PermCreatePolicy, PermUpdatePolicy, PermReadPolicy, PermDeletePolicy,
		PermListRoutes, PermCreateRoute, PermReadRoute, PermDeleteRoute,
		PermListRadios, PermReadRadio,
		PermGetNATInfo, PermUpdateNATInfo,
		PermGetFlowAccountingInfo, PermUpdateFlowAccountingInfo,
		PermListRadioEvents, PermGetRadioEventRetentionPolicy, PermSetRadioEventRetentionPolicy, PermClearRadioEvents, PermGetRadioEvent,
		PermGetFlowReportsRetentionPolicy, PermSetFlowReportsRetentionPolicy, PermListFlowReports, PermClearFlowReports,
	},
}

const (
	// Pprof permission
	PermPprof = "pprof"

	// User permissions
	PermListUsers            = "user:list"
	PermCreateUser           = "user:create"
	PermUpdateUser           = "user:update"
	PermUpdateUserPassword   = "user:update_password"
	PermReadUser             = "user:read"
	PermDeleteUser           = "user:delete"
	PermReadMyUser           = "user:read_my_user"
	PermUpdateMyUserPassword = "user:update_my_user_password" // #nosec: G101
	PermListMyAPITokens      = "user:list_my_api_tokens"      // #nosec: G101
	PermCreateMyAPIToken     = "user:create_my_api_token"
	PermDeleteMyAPIToken     = "user:delete_my_api_token"

	// Data Network permissions
	PermListDataNetworks  = "data_network:list"
	PermCreateDataNetwork = "data_network:create"
	PermUpdateDataNetwork = "data_network:update"
	PermReadDataNetwork   = "data_network:read"
	PermDeleteDataNetwork = "data_network:delete"

	// Operator permissions
	PermReadOperator              = "operator:read"
	PermUpdateOperatorSlice       = "operator:update_slice"
	PermGetOperatorSlice          = "operator:get_slice"
	PermUpdateOperatorTracking    = "operator:update_tracking"
	PermGetOperatorTracking       = "operator:get_tracking"
	PermUpdateOperatorID          = "operator:update_id"
	PermGetOperatorID             = "operator:get_id"
	PermUpdateOperatorCode        = "operator:update_code"
	PermUpdateOperatorHomeNetwork = "operator:update_home_network"

	// Subscriber permissions
	PermListSubscribers  = "subscriber:list"
	PermCreateSubscriber = "subscriber:create"
	PermUpdateSubscriber = "subscriber:update"
	PermReadSubscriber   = "subscriber:read"
	PermDeleteSubscriber = "subscriber:delete"

	// Subscriber Usage permissions
	PermGetSubscriberUsageRetentionPolicy = "subscriber_usage:get_retention"
	PermSetSubscriberUsageRetentionPolicy = "subscriber_usage:set_retention"
	PermClearSubscriberUsage              = "subscriber_usage:clear"
	PermGetSubscriberUsage                = "subscriber_usage:get"

	// Policy permissions
	PermListPolicies = "policy:list"
	PermCreatePolicy = "policy:create"
	PermUpdatePolicy = "policy:update"
	PermReadPolicy   = "policy:read"
	PermDeletePolicy = "policy:delete"

	// Route permissions
	PermListRoutes  = "route:list"
	PermCreateRoute = "route:create"
	PermReadRoute   = "route:read"
	PermDeleteRoute = "route:delete"

	// NAT permissions
	PermGetNATInfo    = "nat:get"
	PermUpdateNATInfo = "nat:update"

	// Flow Accounting permissions
	PermGetFlowAccountingInfo    = "flow_accounting:get"
	PermUpdateFlowAccountingInfo = "flow_accounting:update"

	// Interface permissions
	PermListNetworkInterfaces = "network_interface:list"
	PermUpdateN3Interface     = "network_interface:update_n3"

	// Radio permissions
	PermListRadios = "radio:list"
	PermReadRadio  = "radio:read"

	// Radio event permissions
	PermGetRadioEventRetentionPolicy = "radio_events:get_retention"
	PermSetRadioEventRetentionPolicy = "radio_events:set_retention"
	PermListRadioEvents              = "radio_events:list"
	PermClearRadioEvents             = "radio_events:clear"
	PermGetRadioEvent                = "radio_events:get"

	// Backup and Restore permissions
	PermBackup  = "backup:create"
	PermRestore = "backup:restore"

	// Audit Log permissions
	PermGetAuditLogRetentionPolicy = "audit_logs:get_retention"
	PermSetAuditLogRetentionPolicy = "audit_logs:set_retention"
	PermListAuditLogs              = "audit_logs:list"

	// Flow Reports permissions
	PermGetFlowReportsRetentionPolicy = "flow_reports:get_retention"
	PermSetFlowReportsRetentionPolicy = "flow_reports:set_retention"
	PermListFlowReports               = "flow_reports:list"
	PermClearFlowReports              = "flow_reports:clear"
)

func Authorize(permission string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowedPerms := PermissionsByRole[r.Context().Value(contextKeyRoleID).(RoleID)]
		for _, p := range allowedPerms {
			if p == permission || p == "*" {
				next.ServeHTTP(w, r)
				return
			}
		}

		writeError(r.Context(), w, http.StatusForbidden, "Forbidden", errors.New("permission denied"), logger.APILog)
	})
}
