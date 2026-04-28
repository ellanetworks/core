package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/bgp"
	"github.com/ellanetworks/core/internal/cluster/listener"
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
	DrainState string `json:"drainState"`
}

// DrainClusterMember handles POST /api/v1/cluster/members/{id}/drain.
//
// Runs on the Raft leader (follower requests arrive via the standard leader
// proxy). The leader persists drain-state transitions through Raft and
// invokes node-local side-effects (AMF Status Indication, BGP stop) either
// inline when the target is the leader itself or over the cluster mTLS port
// when the target is a different node. Leadership transfer runs only when
// the target is the leader and is the last local step, so subsequent
// state writes all land on the new leader cleanly.
//
// When deadlineSeconds is 0 (default) drain is synchronous and the response
// state is "drained". When deadlineSeconds > 0 the response returns
// immediately with state "draining"; a background watcher on the leader
// flips the state to "drained" once the target's active-lease count
// reaches zero or the deadline expires.
func DrainClusterMember(dbInstance *db.Database, amfInstance *amf.AMF, bgpService *bgp.BGPService, ln *listener.Listener) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if dbInstance.ClusterEnabled() && !dbInstance.IsLeader() {
			writeError(r.Context(), w, http.StatusMisdirectedRequest,
				"not the leader; retry against the current leader", nil, logger.APILog)

			return
		}

		nodeID, ok := parseMemberIDPath(r)
		if !ok {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid node ID", nil, logger.APILog)
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
		// current state and skips side-effects.
		if member.DrainState == db.DrainStateDraining || member.DrainState == db.DrainStateDrained {
			writeResponse(r.Context(), w, DrainResponse{
				DrainState: member.DrainState,
			}, http.StatusOK, logger.APILog)

			return
		}

		if err := dbInstance.SetDrainState(r.Context(), nodeID, db.DrainStateDraining); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError,
				"Failed to persist drain state", err, logger.APILog)

			return
		}

		// Run node-local side-effects on the target: either inline (target
		// is the leader) or over the cluster mTLS port (target is a
		// follower).
		sideEffects, sideErr := runDrainSideEffects(r.Context(), dbInstance, amfInstance, bgpService, ln, member)
		if sideErr != nil {
			if rbErr := dbInstance.SetDrainState(r.Context(), nodeID, db.DrainStateActive); rbErr != nil {
				logger.APILog.Warn("failed to roll back drain state after side-effects failure",
					zap.Error(rbErr))
			}

			writeError(r.Context(), w, http.StatusInternalServerError,
				"drain side-effects failed", sideErr, logger.APILog)

			return
		}

		state := db.DrainStateDraining

		if req.DeadlineSeconds == 0 {
			if err := dbInstance.SetDrainState(r.Context(), nodeID, db.DrainStateDrained); err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError,
					"Failed to finalise drain state", err, logger.APILog)

				return
			}

			state = db.DrainStateDrained
		} else {
			// #nosec G118 -- the deadline watcher must outlive r.Context(),
			// which is cancelled as soon as this handler returns.
			go watchDrainDeadline(context.Background(), dbInstance, nodeID, time.Duration(req.DeadlineSeconds)*time.Second)
		}

		// Leadership transfer is the last local step: transferring earlier
		// would strand subsequent replicated writes on a now-follower.
		transferred := false

		if nodeID == dbInstance.NodeID() && dbInstance.ClusterEnabled() && dbInstance.IsLeader() {
			if err := dbInstance.LeadershipTransfer(); err != nil {
				logger.APILog.Warn("leadership transfer failed during self-drain; drain state is already drained",
					zap.Error(err))
			} else {
				transferred = true
			}
		}

		actor := getActorFromContext(r)

		logger.LogAuditEvent(
			r.Context(),
			DrainAction,
			actor,
			getClientIP(r),
			fmt.Sprintf("Node %d drain, state=%s, leadership_transferred=%v, rans_notified=%d, bgp_stopped=%v, deadline_s=%d",
				nodeID, state, transferred, sideEffects.RANsNotified, sideEffects.BGPStopped, req.DeadlineSeconds),
		)

		writeResponse(r.Context(), w, DrainResponse{
			DrainState: state,
		}, http.StatusOK, logger.APILog)
	})
}

// ResumeClusterMember handles POST /api/v1/cluster/members/{id}/resume.
//
// Runs on the leader. Restarts the target's local BGP speaker (either inline
// when target is the leader or over the cluster mTLS port) and then clears
// the drain state. Does not reverse AMF Status Indication (no NGAP message
// does that) or reclaim Raft leadership transferred during drain.
func ResumeClusterMember(dbInstance *db.Database, bgpService *bgp.BGPService, ln *listener.Listener) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if dbInstance.ClusterEnabled() && !dbInstance.IsLeader() {
			writeError(r.Context(), w, http.StatusMisdirectedRequest,
				"not the leader; retry against the current leader", nil, logger.APILog)

			return
		}

		nodeID, ok := parseMemberIDPath(r)
		if !ok {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid node ID", nil, logger.APILog)
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
			writeResponse(r.Context(), w, SuccessResponse{Message: "Cluster member resumed"}, http.StatusOK, logger.APILog)
			return
		}

		bgpStarted, sideErr := runResumeSideEffects(r.Context(), dbInstance, bgpService, ln, member)
		if sideErr != nil {
			writeError(r.Context(), w, http.StatusInternalServerError,
				"resume side-effects failed", sideErr, logger.APILog)

			return
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

		writeResponse(r.Context(), w, SuccessResponse{Message: "Cluster member resumed"}, http.StatusOK, logger.APILog)
	})
}

// runDrainSideEffects notifies RANs and stops the BGP speaker on the target.
// When the target is the local (leader) node the steps run inline; otherwise
// they are dispatched to the target over the cluster mTLS port.
func runDrainSideEffects(ctx context.Context, dbInstance *db.Database, amfInstance *amf.AMF, bgpService *bgp.BGPService, ln *listener.Listener, member *db.ClusterMember) (DrainSideEffectsResponse, error) {
	if member.NodeID == dbInstance.NodeID() {
		ransNotified := notifyRANsUnavailable(ctx, amfInstance, defaultDrainStepTimeout)

		bgpStopped := false

		if bgpService != nil {
			if err := bgpService.Stop(); err != nil {
				logger.APILog.Warn("BGP stop during drain failed", zap.Error(err))
			} else {
				bgpStopped = true
			}
		}

		return DrainSideEffectsResponse{RANsNotified: ransNotified, BGPStopped: bgpStopped}, nil
	}

	var out DrainSideEffectsResponse

	if err := postClusterInternal(ctx, ln, member.RaftAddress, member.NodeID, "/cluster/internal/drain-side-effects", &out); err != nil {
		return DrainSideEffectsResponse{}, err
	}

	return out, nil
}

func runResumeSideEffects(ctx context.Context, dbInstance *db.Database, bgpService *bgp.BGPService, ln *listener.Listener, member *db.ClusterMember) (bool, error) {
	if member.NodeID == dbInstance.NodeID() {
		if bgpService == nil || bgpService.IsRunning() {
			return false, nil
		}

		bgpEnabled, err := dbInstance.IsBGPEnabled(ctx)
		if err != nil {
			logger.APILog.Warn("resume: failed to read BGP enabled flag; skipping BGP restart",
				zap.Error(err))

			return false, nil
		}

		if !bgpEnabled {
			return false, nil
		}

		if err := bgpService.Restart(ctx); err != nil {
			logger.APILog.Warn("resume: failed to restart BGP speaker", zap.Error(err))
			return false, nil
		}

		return true, nil
	}

	var out ResumeSideEffectsResponse

	if err := postClusterInternal(ctx, ln, member.RaftAddress, member.NodeID, "/cluster/internal/resume-side-effects", &out); err != nil {
		return false, err
	}

	return out.BGPStarted, nil
}

// postClusterInternal issues a JSON-body-less POST to the target node's
// cluster mTLS port and decodes the 2xx response JSON into out. The caller
// supplies the target's Raft address, its node-id (enforced on the mTLS
// dial), and the path (including leading slash). Used for the drain/resume
// side-effect RPCs.
func postClusterInternal(ctx context.Context, ln *listener.Listener, targetRaftAddr string, targetNodeID int, path string, out any) error {
	if ln == nil {
		return fmt.Errorf("cluster listener unavailable")
	}

	if targetRaftAddr == "" {
		return fmt.Errorf("target node has no raft address")
	}

	if targetNodeID == 0 {
		return fmt.Errorf("target node has no node-id")
	}

	client := dialPeerHTTPClient(ln, targetNodeID)
	defer client.CloseIdleConnections()

	url := fmt.Sprintf("https://%s%s", targetRaftAddr, path)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(nil)) // #nosec G107,G704 -- built from Raft-replicated raft address, not user input
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req) // #nosec G704 -- URL built from Raft-replicated raft address, not user input
	if err != nil {
		return fmt.Errorf("post to %s: %w", targetRaftAddr, err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("target returned %s", resp.Status)
	}

	var envelope struct {
		Result json.RawMessage `json:"result"`
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if len(envelope.Result) == 0 {
		return nil
	}

	return json.Unmarshal(envelope.Result, out)
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

// watchDrainDeadline polls the target's active-lease count every second and
// flips drain_state to "drained" once the count reaches zero or the deadline
// elapses. Runs on the leader that handled the drain request. If leadership
// changes during the wait, the final SetDrainState proposal fails with
// ErrNotLeader and the state stays "draining"; the operator must re-drain.
// This matches the spec's non-goal of deadline persistence across failures.
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

// SignalShutdownDrain marks this node as drained in the replicated
// cluster_members table so surviving nodes see a clean "active → drained"
// transition on operator-initiated shutdown, instead of waiting for
// autopilot to remove a dead voter after LastContactThreshold. On the
// leader the write is proposed locally; on a follower it is sent to the
// current leader's cluster mTLS port. Best-effort: errors are logged and
// do not block the rest of the shutdown sequence.
//
// No-op when the node's current state is "active" — an unsolicited
// restart (systemctl restart, kernel update, ad-hoc reboot) must not
// strand the node out of service. Operator-initiated drain (state
// "draining" or "drained") still finalises to "drained" on shutdown
// so the rolling-upgrade signal is preserved.
func SignalShutdownDrain(ctx context.Context, dbInstance *db.Database, ln *listener.Listener) error {
	if dbInstance == nil || !dbInstance.ClusterEnabled() {
		return nil
	}

	nodeID := dbInstance.NodeID()

	member, err := dbInstance.GetClusterMember(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("read self cluster member: %w", err)
	}

	if member.DrainState == db.DrainStateActive {
		return nil
	}

	if dbInstance.IsLeader() {
		return dbInstance.SetDrainState(ctx, nodeID, db.DrainStateDrained)
	}

	leaderAddr, leaderID := dbInstance.LeaderAddressAndID()
	if leaderAddr == "" {
		return fmt.Errorf("no leader available")
	}

	if leaderID == 0 {
		return fmt.Errorf("leader identity not yet established")
	}

	if ln == nil {
		return fmt.Errorf("cluster listener unavailable")
	}

	client := dialPeerHTTPClient(ln, leaderID)
	defer client.CloseIdleConnections()

	url := fmt.Sprintf("https://%s/cluster/internal/drain-self", leaderAddr)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(nil)) // #nosec G107,G704 -- URL built from Raft-replicated leader address, not user input
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	resp, err := client.Do(req) // #nosec G704 -- URL built from Raft-replicated leader address, not user input
	if err != nil {
		return fmt.Errorf("post to leader: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("leader returned %s", resp.Status)
	}

	return nil
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
