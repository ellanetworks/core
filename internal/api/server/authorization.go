package server

import (
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
)

var roleNameToID = map[string]int{
	"admin":           1,
	"readonly":        2,
	"network-manager": 3,
}

// map role ID to role name
var roleIDToName = map[int]string{
	1: "admin",
	2: "readonly",
	3: "network-manager",
}

var PermissionsByRole = map[int][]string{
	1: {"*"}, // Admin

	2: { // Read Only
		PermReadMyUser,
		PermReadOperator, PermGetOperatorSlice, PermGetOperatorTracking,
		PermListSubscribers, PermReadSubscriber,
		PermListProfiles, PermReadProfile,
		PermListRoutes, PermReadRoute,
		PermListRadios, PermReadRadio,
	},

	3: { // Network Manager
		PermUpdateUserPassword, PermReadUser, PermReadMyUser,
		PermReadOperator, PermUpdateOperatorSlice, PermGetOperatorSlice, PermUpdateOperatorTracking, PermGetOperatorTracking,
		PermListSubscribers, PermCreateSubscriber, PermUpdateSubscriber, PermReadSubscriber, PermDeleteSubscriber,
		PermListProfiles, PermCreateProfile, PermUpdateProfile, PermReadProfile, PermDeleteProfile,
		PermListRoutes, PermCreateRoute, PermReadRoute, PermDeleteRoute,
		PermListRadios, PermReadRadio,
	},
}

const (
	// User permissions
	PermListUsers          = "user:list"
	PermCreateUser         = "user:create"
	PermUpdateUser         = "user:update"
	PermUpdateUserPassword = "user:update_password"
	PermReadUser           = "user:read"
	PermDeleteUser         = "user:delete"
	PermReadMyUser         = "user:read_my_user"

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

	// Profile permissions
	PermListProfiles  = "profile:list"
	PermCreateProfile = "profile:create"
	PermUpdateProfile = "profile:update"
	PermReadProfile   = "profile:read"
	PermDeleteProfile = "profile:delete"

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
)

func RequirePermissionOrFirstUser(permission string, db *db.Database, jwtSecret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		if permission == PermCreateUser && c.Request.Method == http.MethodPost {
			userCount, err := db.NumUsers(c.Request.Context())
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to count users"})
				return
			}
			if userCount == 0 {
				c.Next()
				return
			}
		}

		// authenticate inline, not via middleware chain
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header not found"})
			return
		}

		claims, err := getClaimsFromAuthorizationHeader(authHeader, jwtSecret)
		if err != nil {
			logger.LogAuditEvent("auth_fail", "", c.ClientIP(), "unauthorized")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		c.Set("userID", claims.ID)
		c.Set("email", claims.Email)
		c.Set("role_id", claims.RoleID)

		// permission check
		allowedPerms := PermissionsByRole[claims.RoleID]
		authorized := false
		for _, p := range allowedPerms {
			if p == permission || p == "*" {
				authorized = true
				break
			}
		}
		if !authorized {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
			return
		}

		c.Next()
	}
}

func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleIDAny, exists := c.Get("role_id")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Role ID not found"})
			return
		}

		roleID, ok := roleIDAny.(int)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Invalid role ID format"})
			return
		}

		allowedPerms := PermissionsByRole[roleID]
		for _, p := range allowedPerms {
			if p == permission || p == "*" {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
	}
}
