package models

// SessionReconcileReason identifies why a session reconciliation was triggered.
type SessionReconcileReason string

const (
	// ReconcileSkip indicates that the reconciler could not determine the
	// correct action (e.g. policy lookup returned nil without an error).
	// The session is left untouched and the backstop timer will retry.
	ReconcileSkip SessionReconcileReason = ""
	// ReconcilePolicyChange indicates the session's policy was updated.
	ReconcilePolicyChange SessionReconcileReason = "policy_change"
	// ReconcileSliceMismatch indicates the session's network slice (SST/SD)
	// no longer matches any configured slice. The session must be released
	// with cause #39 "reactivation requested" so the UE re-establishes on
	// the correct slice.
	ReconcileSliceMismatch SessionReconcileReason = "slice_mismatch"
)

// SessionReconcileRequest carries the data needed to reconcile an active PDU
// session when subscriber-related fields (profile, policy) change in the DB.
type SessionReconcileRequest struct {
	SmContextRef string              // canonical SM context reference
	OldPolicy    *SessionPolicyDelta // previous policy values (may be nil for profile changes)
	NewPolicy    *SessionPolicyDelta // new policy values
	Reason       SessionReconcileReason
}

// SessionPolicyDelta holds the policy fields that affect an active session.
type SessionPolicyDelta struct {
	SessionAmbrUplink   string // e.g. "100 Mbps"
	SessionAmbrDownlink string // e.g. "500 Mbps"
	Var5qi              int32
	Arp                 int32
	PreemptCap          PreemptionCapability
	PreemptVuln         PreemptionVulnerability
}
