package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

type InitializeParams struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

const (
	InitializeAction = "initialize"
)

func Initialize(dbInstance *db.Database, secureCookie bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var newUser InitializeParams

		if err := json.NewDecoder(r.Body).Decode(&newUser); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if newUser.Email == "" {
			writeError(w, http.StatusBadRequest, "email is missing", errors.New("missing email"), logger.APILog)
			return
		}

		if newUser.Password == "" {
			writeError(w, http.StatusBadRequest, "password is missing", errors.New("missing password"), logger.APILog)
			return
		}

		if !isValidEmail(newUser.Email) {
			writeError(w, http.StatusBadRequest, "Invalid email format", errors.New("bad format"), logger.APILog)
			return
		}

		numUsers, err := dbInstance.CountUsers(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to count users", err, logger.APILog)
			return
		}

		if numUsers != 0 {
			writeError(w, http.StatusForbidden, "System already initialized", errors.New("users already exist"), logger.APILog)
			return
		}

		hashedPassword, err := hashPassword(newUser.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to hash password", err, logger.APILog)
			return
		}

		dbUser := &db.User{
			Email:          newUser.Email,
			HashedPassword: hashedPassword,
			RoleID:         db.RoleAdmin,
		}

		userID, err := dbInstance.CreateUser(r.Context(), dbUser)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to create user", err, logger.APILog)
			return
		}

		err = createSessionAndSetCookie(r.Context(), dbInstance, userID, secureCookie, w)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Internal Error", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "System initialized successfully"}, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(
			r.Context(),
			InitializeAction,
			newUser.Email,
			getClientIP(r),
			fmt.Sprintf("System initialized with first user %s", newUser.Email),
		)
	})
}
