package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/bgp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const (
	DrainAction  = "cluster_member_drain"
	ResumeAction = "cluster_member_resume"
)

const (
	defaultDrainStepTimeout  = 5 * time.Second
	drainSessionPollInterval = 1 * time.Second
	drainMaxDeadlineSeconds  = 3600
)

type DrainRequest struct {
	DeadlineSeconds int `json:"deadlineSeconds,omitempty"`
}

type DrainResponse struct {
	Message               string `json:"message"`
	State                 string `json:"state"`
	TransferredLeadership bool   `json:"transferredLeadership"`
	RANsNotified          int    `json:"ransNotified"`
	BGPStopped            bool   `json:"bgpStopped"`
	SessionsRemaining     int    `json:"sessionsRemaining"`
}

type ResumeResponse struct {
	Message    string `json:"message"`
	State      string `json:"state"`
	BGPStarted bool   `json:"bgpStarted"`
}

// DrainClusterMember handles POST /api/v1/cluster/members/{id}/drain.
//
// The request must execute on the target node itself; LeaderProxyMiddleware
// routes per-member drain requests to the target's cluster port so this
// handler always runs on the node being drained. As defense-in-depth, it
// also verifies {id} matches the local node and refuses otherwise.
//
// Side-effects (leadership transfer, AMF Status Indication, BGP stop) run
// synchronously. The drain_state row is set to "draining" before returning;
// if deadlineSeconds > 0 a background goroutine flips it to "drained" when
// the local active-lease count reaches zero or the deadline expires. When
// deadlineSeconds is 0, the row is flipped to "drained" before returning.
func DrainClusterMember(dbInstance *db.Database, amfInstance *amf.AMF, bgpService *bgp.BGPService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nodeID, ok := parseMemberIDPath(r)
		if !ok {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid node ID", nil, logger.APILog)
			return
		}

		if nodeID != dbInstance.NodeID() {
			writeError(r.Context(), w, http.StatusBadGateway,
				"drain handler reached on a non-target node; check proxy configuration", nil, logger.APILog)

			return
		}

		member, err := dbInstance.GetClusterMember(r.Context(), nodeID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Cluster member not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to look up cluster member", err, logger.APILog)

			return
		}

		var req DrainRequest
		if r.Body != nil && r.ContentLength > 0 {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(r.Context(), w, http.StatusBadRequest, "Invalid request body", err, logger.APILog)
				return
			}
		}

		if req.DeadlineSeconds < 0 || req.DeadlineSeconds > drainMaxDeadlineSeconds {
			writeError(r.Context(), w, http.StatusBadRequest,
				fmt.Sprintf("deadlineSeconds must be between 0 and %d", drainMaxDeadlineSeconds),
				nil, logger.APILog)

			return
		}

		// Idempotent: re-draining a draining/drained node just returns the
		// current state and skips side-effects. Callers that want to retry
		// a failed drain should resume first.
		if member.DrainState == db.DrainStateDraining || member.DrainState == db.DrainStateDrained {
			remaining := countLocalActiveLeases(r.Context(), dbInstance, nodeID)

			writeResponse(r.Context(), w, DrainResponse{
				Message:           "drain already in effect",
				State:             member.DrainState,
				SessionsRemaining: remaining,
			}, http.StatusOK, logger.APILog)

			return
		}

		if err := dbInstance.SetDrainState(r.Context(), nodeID, db.DrainStateDraining); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError,
				"Failed to persist drain state", err, logger.APILog)

			return
		}

		transferred := false

		if dbInstance.IsLeader() && dbInstance.ClusterEnabled() {
			if err := dbInstance.LeadershipTransfer(); err != nil {
				// Rollback state so the operator can retry cleanly.
				if rbErr := dbInstance.SetDrainState(r.Context(), nodeID, db.DrainStateActive); rbErr != nil {
					logger.APILog.Warn("failed to roll back drain state after leadership-transfer failure",
						zap.Error(rbErr))
				}

				writeError(r.Context(), w, http.StatusInternalServerError,
					"drain aborted: leadership transfer failed", err, logger.APILog)

				return
			}

			transferred = true
		}

		ransNotified := notifyRANsUnavailable(r.Context(), amfInstance, defaultDrainStepTimeout)

		bgpStopped := false

		if bgpService != nil {
			if err := bgpService.Stop(); err != nil {
				logger.APILog.Warn("BGP stop during drain failed", zap.Error(err))
			} else {
				bgpStopped = true
			}
		}

		state := db.DrainStateDraining
		remaining := countLocalActiveLeases(r.Context(), dbInstance, nodeID)

		if req.DeadlineSeconds == 0 {
			if err := dbInstance.SetDrainState(r.Context(), nodeID, db.DrainStateDrained); err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError,
					"Failed to finalise drain state", err, logger.APILog)

				return
			}

			state = db.DrainStateDrained
		} else {
			// #nosec G118 -- the deadline watcher must outlive r.Context(),
			// which is cancelled as soon as this handler returns; the
			// goroutine polls for up to deadlineSeconds and then finalises
			// drain_state, so it deliberately uses a detached context.
			go watchDrainDeadline(context.Background(), dbInstance, nodeID, time.Duration(req.DeadlineSeconds)*time.Second)
		}

		actor := getActorFromContext(r)

		logger.LogAuditEvent(
			r.Context(),
			DrainAction,
			actor,
			getClientIP(r),
			fmt.Sprintf("Node %d drain started, state=%s, leadership_transferred=%v, rans_notified=%d, bgp_stopped=%v, deadline_s=%d",
				nodeID, state, transferred, ransNotified, bgpStopped, req.DeadlineSeconds),
		)

		writeResponse(r.Context(), w, DrainResponse{
			Message:               "draining",
			State:                 state,
			TransferredLeadership: transferred,
			RANsNotified:          ransNotified,
			BGPStopped:            bgpStopped,
			SessionsRemaining:     remaining,
		}, http.StatusOK, logger.APILog)
	})
}

// ResumeClusterMember handles POST /api/v1/cluster/members/{id}/resume.
//
// Restarts the local BGP speaker (if BGP is globally enabled and was stopped
// by a prior drain) and clears the drain_state back to "active". Does not
// reverse AMF Status Indication (no NGAP message does that) and does not
// reclaim Raft leadership that was transferred during drain.
func ResumeClusterMember(dbInstance *db.Database, bgpService *bgp.BGPService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nodeID, ok := parseMemberIDPath(r)
		if !ok {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid node ID", nil, logger.APILog)
			return
		}

		if nodeID != dbInstance.NodeID() {
			writeError(r.Context(), w, http.StatusBadGateway,
				"resume handler reached on a non-target node; check proxy configuration", nil, logger.APILog)

			return
		}

		member, err := dbInstance.GetClusterMember(r.Context(), nodeID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Cluster member not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to look up cluster member", err, logger.APILog)

			return
		}

		if member.DrainState == db.DrainStateActive {
			writeResponse(r.Context(), w, ResumeResponse{
				Message: "already active",
				State:   db.DrainStateActive,
			}, http.StatusOK, logger.APILog)

			return
		}

		bgpStarted := false

		if bgpService != nil && !bgpService.IsRunning() {
			bgpEnabled, err := dbInstance.IsBGPEnabled(r.Context())
			if err != nil {
				logger.APILog.Warn("resume: failed to read BGP enabled flag; skipping BGP restart",
					zap.Error(err))
			} else if bgpEnabled {
				if err := bgpService.Restart(r.Context()); err != nil {
					logger.APILog.Warn("resume: failed to restart BGP speaker", zap.Error(err))
				} else {
					bgpStarted = true
				}
			}
		}

		if err := dbInstance.SetDrainState(r.Context(), nodeID, db.DrainStateActive); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError,
				"Failed to clear drain state", err, logger.APILog)

			return
		}

		actor := getActorFromContext(r)

		logger.LogAuditEvent(
			r.Context(),
			ResumeAction,
			actor,
			getClientIP(r),
			fmt.Sprintf("Node %d resumed, bgp_started=%v", nodeID, bgpStarted),
		)

		writeResponse(r.Context(), w, ResumeResponse{
			Message:    "resumed",
			State:      db.DrainStateActive,
			BGPStarted: bgpStarted,
		}, http.StatusOK, logger.APILog)
	})
}

func parseMemberIDPath(r *http.Request) (int, bool) {
	idStr := r.PathValue("id")
	if idStr == "" {
		return 0, false
	}

	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		return 0, false
	}

	return id, true
}

func countLocalActiveLeases(ctx context.Context, dbInstance *db.Database, nodeID int) int {
	leases, err := dbInstance.ListActiveLeasesByNode(ctx, nodeID)
	if err != nil {
		logger.APILog.Warn("failed to count local active leases", zap.Error(err))
		return -1
	}

	return len(leases)
}

// watchDrainDeadline polls the local active-lease count every second and
// flips drain_state to "drained" once the count reaches zero or the deadline
// elapses. Runs in its own goroutine with a background context so it
// survives the originating HTTP request ending.
func watchDrainDeadline(ctx context.Context, dbInstance *db.Database, nodeID int, deadline time.Duration) {
	ctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	ticker := time.NewTicker(drainSessionPollInterval)
	defer ticker.Stop()

	for countLocalActiveLeases(ctx, dbInstance, nodeID) != 0 {
		select {
		case <-ctx.Done():
			logger.APILog.Info("drain deadline elapsed with active sessions still present",
				zap.Int("nodeId", nodeID))

			if err := dbInstance.SetDrainState(context.Background(), nodeID, db.DrainStateDrained); err != nil {
				logger.APILog.Warn("failed to finalise drain state after deadline",
					zap.Int("nodeId", nodeID), zap.Error(err))
			}

			return
		case <-ticker.C:
		}
	}

	if err := dbInstance.SetDrainState(context.Background(), nodeID, db.DrainStateDrained); err != nil {
		logger.APILog.Warn("failed to finalise drain state after sessions reached zero",
			zap.Int("nodeId", nodeID), zap.Error(err))
	}
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
