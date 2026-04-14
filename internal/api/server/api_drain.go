package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/bgp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const DrainAction = "cluster_drain"

const defaultDrainStepTimeout = 5 * time.Second

type DrainRequest struct {
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`
}

type DrainResponse struct {
	Message               string `json:"message"`
	TransferredLeadership bool   `json:"transferredLeadership"`
	RANsNotified          int    `json:"ransNotified"`
	BGPStopped            bool   `json:"bgpStopped"`
}

// DrainNode gracefully prepares this node for removal: it signals RANs to
// redirect new UE registrations elsewhere via AMF Status Indication,
// withdraws BGP advertisements so upstream routers reroute user plane
// traffic, and transfers Raft leadership if this node is the leader.
// The node continues serving existing flows until it is shut down.
func DrainNode(dbInstance *db.Database, amfInstance *amf.AMF, bgpService *bgp.BGPService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req DrainRequest
		if r.Body != nil && r.ContentLength > 0 {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(r.Context(), w, http.StatusBadRequest, "Invalid request body", err, logger.APILog)
				return
			}
		}

		stepTimeout := defaultDrainStepTimeout
		if req.TimeoutSeconds > 0 {
			stepTimeout = time.Duration(req.TimeoutSeconds) * time.Second
		}

		ransNotified := notifyRANsUnavailable(r.Context(), amfInstance, stepTimeout)

		bgpStopped := false

		if bgpService != nil {
			if err := bgpService.Stop(); err != nil {
				logger.APILog.Warn("BGP stop during drain failed", zap.Error(err))
			} else {
				bgpStopped = true
			}
		}

		transferred := false

		var transferErr error

		if dbInstance.IsLeader() && dbInstance.ClusterEnabled() {
			if err := dbInstance.LeadershipTransfer(); err != nil {
				logger.APILog.Warn("Leadership transfer failed during drain", zap.Error(err))
				transferErr = err
			} else {
				transferred = true
			}
		}

		email := getEmailFromContext(r)

		logger.LogAuditEvent(
			r.Context(),
			DrainAction,
			email,
			getClientIP(r),
			fmt.Sprintf("Node %d draining, leadership_transferred=%v, rans_notified=%d, bgp_stopped=%v",
				dbInstance.NodeID(), transferred, ransNotified, bgpStopped),
		)

		status := http.StatusOK
		msg := "draining"

		if transferErr != nil {
			status = http.StatusInternalServerError
			msg = "draining but leadership transfer failed"
		}

		writeResponse(r.Context(), w, DrainResponse{
			Message:               msg,
			TransferredLeadership: transferred,
			RANsNotified:          ransNotified,
			BGPStopped:            bgpStopped,
		}, status, logger.APILog)
	})
}

// notifyRANsUnavailable signals every connected RAN that this AMF's GUAMI is
// unavailable, per TS 38.413, so the RAN redirects new UEs to sibling AMFs.
// Returns the number of RANs successfully notified.
func notifyRANsUnavailable(ctx context.Context, amfInstance *amf.AMF, timeout time.Duration) int {
	if amfInstance == nil {
		return 0
	}

	queryCtx, queryCancel := context.WithTimeout(ctx, timeout)
	operatorInfo, err := amfInstance.GetOperatorInfo(queryCtx)

	queryCancel()

	if err != nil {
		logger.APILog.Warn("Could not get operator info for drain", zap.Error(err))
		return 0
	}

	unavailableGUAMIList := send.BuildUnavailableGUAMIList(operatorInfo.Guami)

	sendCtx, sendCancel := context.WithTimeout(ctx, timeout)
	defer sendCancel()

	notified := 0

	for _, ran := range amfInstance.ListRadios() {
		if err := ran.NGAPSender.SendAMFStatusIndication(sendCtx, unavailableGUAMIList); err != nil {
			logger.APILog.Warn("failed to send AMF Status Indication to RAN during drain", zap.Error(err))
			continue
		}

		notified++
	}

	return notified
}
