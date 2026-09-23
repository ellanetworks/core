// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/cluster/joinreq"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

var initMu sync.Mutex

type InitializeParams struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type InitializeResponse struct {
	Message string `json:"message"`
	Token   string `json:"token"`
}

const (
	InitializeAction = "initialize"

	initClusterPollInterval = 250 * time.Millisecond
	initClusterTimeout      = 2 * time.Minute
)

func foundClusterIfWaiting(ctx context.Context, dbInstance *db.Database) error {
	coord := loadJoinCoordinator()
	if coord == nil || coord.Status().State != joinreq.StateWaiting {
		return nil
	}

	if err := coord.Submit(joinreq.Request{Mode: joinreq.ModeBootstrap}); err != nil {
		return fmt.Errorf("found cluster: %w", err)
	}

	return waitForWritableCluster(ctx, dbInstance)
}

func waitForWritableCluster(ctx context.Context, dbInstance *db.Database) error {
	ticker := time.NewTicker(initClusterPollInterval)
	defer ticker.Stop()

	deadline := time.Now().Add(initClusterTimeout)

	for {
		if dbInstance.HasLeader() && dbInstance.IsOperatorInitialized(ctx) {
			return nil
		}

		if time.Now().After(deadline) {
			return errors.New("the cluster did not become writable in time")
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func waitForAPIUpgrade(ctx context.Context, apiUpgraded <-chan struct{}) error {
	if apiUpgraded == nil {
		return nil
	}

	timer := time.NewTimer(initClusterTimeout)
	defer timer.Stop()

	select {
	case <-apiUpgraded:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return errors.New("the full API did not become available in time")
	}
}

func resolveJWTSecret(ctx context.Context, dbInstance *db.Database, jwtSecret *JWTSecret) error {
	if len(jwtSecret.Get()) > 0 {
		return nil
	}

	secret, err := dbInstance.GetJWTSecret(ctx)
	if err != nil {
		return fmt.Errorf("load jwt secret: %w", err)
	}

	jwtSecret.Set(secret)

	return nil
}

func Initialize(dbInstance *db.Database, jwtSecret *JWTSecret, secureCookie bool, bcryptCost int, apiUpgraded <-chan struct{}) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var newUser InitializeParams

		if err := json.NewDecoder(r.Body).Decode(&newUser); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if newUser.Email == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "email is missing", errors.New("missing email"), logger.APILog)
			return
		}

		if newUser.Password == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "password is missing", errors.New("missing password"), logger.APILog)
			return
		}

		if !isValidEmail(newUser.Email) {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid email format", errors.New("bad format"), logger.APILog)
			return
		}

		initMu.Lock()
		defer initMu.Unlock()

		if err := foundClusterIfWaiting(r.Context(), dbInstance); err != nil {
			writeError(r.Context(), w, http.StatusServiceUnavailable, "Failed to form the cluster", err, logger.APILog)
			return
		}

		if err := waitForAPIUpgrade(r.Context(), apiUpgraded); err != nil {
			w.Header().Set("Retry-After", "1")
			writeError(r.Context(), w, http.StatusServiceUnavailable, "API server is not ready", err, logger.APILog)

			return
		}

		if err := resolveJWTSecret(r.Context(), dbInstance, jwtSecret); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Internal Error", err, logger.APILog)
			return
		}

		numUsers, err := dbInstance.CountUsers(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to count users", err, logger.APILog)
			return
		}

		if numUsers != 0 {
			writeError(r.Context(), w, http.StatusForbidden, "System already initialized", errors.New("users already exist"), logger.APILog)
			return
		}

		hashedPassword, err := hashPassword(newUser.Password, bcryptCost)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to hash password", err, logger.APILog)
			return
		}

		dbUser := &db.User{
			Email:          newUser.Email,
			HashedPassword: hashedPassword,
			RoleID:         db.RoleAdmin,
		}

		userID, err := dbInstance.CreateUser(r.Context(), dbUser)
		if err != nil {
			if errors.Is(err, db.ErrAlreadyExists) {
				writeError(r.Context(), w, http.StatusConflict, "User already exists", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to create user", err, logger.APILog)

			return
		}

		err = createSessionAndSetCookie(r.Context(), dbInstance, userID, secureCookie, w)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Internal Error", err, logger.APILog)
			return
		}

		token, err := generateJWT(userID, newUser.Email, RoleID(db.RoleAdmin), jwtSecret.Get())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Internal Error", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, InitializeResponse{Message: "System initialized successfully", Token: token}, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(
			r.Context(),
			InitializeAction,
			newUser.Email,
			getClientIP(r),
			fmt.Sprintf("System initialized with first user %s", newUser.Email),
		)
	})
}
