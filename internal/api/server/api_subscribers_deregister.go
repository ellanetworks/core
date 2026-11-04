// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

const DeregisterRoamerAction = "deregister_roamer"

var errRoamerStillRegistered = errors.New("the roamer is still registered (the node that holds it may be unreachable); retry later")

func DeregisterSubscriber(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		imsi := r.PathValue("imsi")
		if _, err := etsi.NewSUPIFromIMSI(imsi); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid IMSI format", err, logger.APILog)
			return
		}

		subscriber, err := dbInstance.GetSubscriberOrVisitor(r.Context(), imsi)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Subscriber not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve subscriber", err, logger.APILog)

			return
		}

		if subscriber.RoamingPartnerID == nil {
			writeError(r.Context(), w, http.StatusConflict, "Only roamers can be deregistered", nil, logger.APILog)
			return
		}

		err = deregisterRoamer(r.Context(), dbInstance, imsi)

		switch {
		case err == nil || errors.Is(err, db.ErrNotFound):
		case errors.Is(err, errRoamerStillRegistered):
			writeError(r.Context(), w, http.StatusConflict, "The roamer is still registered after one minute (the node that holds it may be unreachable); retry later", nil, logger.APILog)
			return
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to deregister the roamer", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Roamer deregistered successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), DeregisterRoamerAction, email, getClientIP(r), "Deregistered roamer "+imsi)
	})
}

func deregisterRoamer(ctx context.Context, dbInstance *db.Database, imsi string) error {
	timing := roamerWithdrawalTiming

	if err := dbInstance.WithdrawRoamer(ctx, imsi, timing.timeout+timing.margin); err != nil {
		return err
	}

	defer func() {
		if err := dbInstance.ClearRoamerWithdrawal(context.WithoutCancel(ctx), imsi); err != nil {
			logger.APILog.Warn("failed to clear the withdrawal of a roamer; it expires on its own", logger.SUPIFromIMSI(imsi))
		}
	}()

	deadline := time.Now().Add(timing.timeout)

	for {
		visitor, err := dbInstance.GetSubscriberOrVisitor(ctx, imsi)

		switch {
		case errors.Is(err, db.ErrNotFound) || (err == nil && visitor.RoamingPartnerID == nil):
			return nil
		case err != nil:
			return err
		}

		if time.Now().After(deadline) {
			return errRoamerStillRegistered
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(timing.poll):
		}
	}
}
