package server

import (
	"errors"
	"net/http"

	"github.com/ellanetworks/core/internal/db"
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
	},

	RoleNetworkManager: {
		PermReadUser, PermReadMyUser, PermUpdateMyUserPassword,
		PermReadOperator, PermUpdateOperatorSlice, PermGetOperatorSlice, PermUpdateOperatorTracking, PermGetOperatorTracking,
		PermListDataNetworks, PermCreateDataNetwork, PermUpdateDataNetwork, PermReadDataNetwork, PermDeleteDataNetwork,
		PermListSubscribers, PermCreateSubscriber, PermUpdateSubscriber, PermReadSubscriber, PermDeleteSubscriber,
		PermListPolicies, PermCreatePolicy, PermUpdatePolicy, PermReadPolicy, PermDeletePolicy,
		PermListRoutes, PermCreateRoute, PermReadRoute, PermDeleteRoute,
		PermListRadios, PermReadRadio,
	},
}

const (
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

	// Radio permissions
	PermListRadios = "radio:list"
	PermReadRadio  = "radio:read"

	// Backup and Restore permissions
	PermBackup  = "backup:create"
	PermRestore = "backup:restore"

	// Audit Log permissions
	PermGetAuditLogRetentionPolicy = "audit_logs:get_retention"
	PermSetAuditLogRetentionPolicy = "audit_logs:set_retention"
	PermListAuditLogs              = "audit_logs:list"

	// Subscriber Log permissions
	PermGetSubscriberLogRetentionPolicy = "subscriber_logs:get_retention"
	PermSetSubscriberLogRetentionPolicy = "subscriber_logs:set_retention"
	PermListSubscriberLogs              = "subscriber_logs:list"
)

func RequirePermissionOrFirstUser(permission string, database *db.Database, jwtSecret []byte, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// 1) First-user bypass only for user creation POST
		if permission == PermCreateUser && r.Method == http.MethodPost {
			n, err := database.NumUsers(ctx)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "Failed to count users", err, logger.APILog)
				return
			}
			if n == 0 {
				// No auth needed; allow bootstrap
				next.ServeHTTP(w, r)
				return
			}
		}

		// 2) Otherwise require authentication
		uid, email, role, err := authenticateRequest(r, jwtSecret, database)
		if err != nil {
			logger.LogAuditEvent(AuthenticationAction, "", getClientIP(r), "Unauthorized: "+err.Error())
			writeError(w, http.StatusUnauthorized, "Invalid token", err, logger.APILog)
			return
		}

		// 3) Put identity in context
		ctx = putIdentity(ctx, uid, email, role)
		r = r.WithContext(ctx)

		// 4) Authorization check
		if !authorize(role, permission) {
			writeError(w, http.StatusForbidden, "Forbidden", errors.New("permission denied"), logger.APILog)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func RequirePermission(permission string, jwtSecret []byte, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowedPerms := PermissionsByRole[r.Context().Value(contextKeyRoleID).(RoleID)]
		for _, p := range allowedPerms {
			if p == permission || p == "*" {
				next.ServeHTTP(w, r)
				return
			}
		}

		writeError(w, http.StatusForbidden, "Forbidden", errors.New("permission denied"), logger.APILog)
	})
}
