// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// ICSState tracks the S1AP Initial Context Setup progress for one connection
// (TS 36.413 §8.3).
type ICSState int

const (
	// ICSNotStarted: the MME has not sent InitialContextSetupRequest yet.
	ICSNotStarted ICSState = iota
	// ICSPending: InitialContextSetupRequest sent, awaiting response.
	ICSPending
	// ICSCompleted: InitialContextSetupResponse received — radio bearers established.
	ICSCompleted
)

// UeConn is a UE's transient state for one UE-associated logical S1-connection
// (TS 36.413): the S1AP identities, the eNB association, the connection-scoped
// NAS-guard supervision, and any in-flight handover. A fresh one is bound
// on each idle→active transition; the persistent UeContext it belongs to survives
// across them. Fields are guarded by MME.mu unless noted.
type UeConn struct {
	ENBUES1APID s1ap.ENBUES1APID
	MMEUES1APID s1ap.MMEUES1APID
	conn        S1APWriter

	// Log carries the connection's MME-UE-S1AP-ID (the temporary identity, TS
	// 33.401 §7.1) so handlers correlate by it. Per-connection with an immutable id.
	Log *zap.Logger

	// ue is the persistent UE context bound to this connection, nil until a UE
	// is identified (a bare connection carries an Initial UE Message not yet
	// attached to a context). Guarded by MME.mu.
	ue *UeContext

	m *MME

	// ICS is the S1AP Initial Context Setup progress: it distinguishes a
	// fully-established connection (ICSCompleted) from one a UE is still resuming on
	// (e.g. a TAU resume that has not re-established bearers). Dispatch-confined
	// (mutated only on the eNB's S1AP goroutine), so a plain field suffices.
	ICS ICSState

	// secureExchangeEstablished records that secure NAS exchange is established on
	// this connection (a message integrity-checked, or a verified resume). Once
	// set, TS 24.301 §4.4.4.3 requires discarding any further message that is not
	// integrity protected or fails the check. Per-connection, as the spec scopes
	// it to the NAS signalling connection.
	secureExchangeEstablished bool

	// In-flight authentication working-state, scoped to this connection's attach: a
	// dropped connection re-authenticates from scratch, so it lives on the connection,
	// not the persistent context. AuthVector is the EPS challenge vector, resyncTried
	// whether SQN re-synchronisation has been attempted this exchange
	// (TS 24.301 §5.4.2.7). Dispatch-confined.
	AuthVector  *udm.EPSAV
	resyncTried bool

	// In-flight attach working-state (TS 24.301 §5.5.1.2.7 case d): AttachRequestPlain
	// is the plaintext ATTACH REQUEST that started the attach, AttachAcceptPdu the
	// protected ATTACH ACCEPT last sent — kept to resend the ACCEPT on a duplicate
	// ATTACH REQUEST with identical IEs while awaiting ATTACH COMPLETE. Connection-
	// scoped like the auth state above. Dispatch-confined.
	AttachRequestPlain []byte
	AttachAcceptPdu    []byte

	// TauReleaseOnComplete defers the S1 release of a no-active TAU until the
	// GUTI reallocation it carried is acknowledged.
	TauReleaseOnComplete bool
	// releasing gates the registry op during a UE Context Release.
	releasing bool

	// EMM common-procedure guard (TS 24.301: T3450/T3460/T3470). EMM common and
	// specific procedures are mutually exclusive, so a single guard suffices; it
	// invalidates a callback whose firing races a release or re-arm. ESM bearer
	// procedures are guarded per-bearer on PdnConnection, running on the
	// independent ESM sublayer concurrently with each other and EMM.
	nasGuard guard.Guard
	// releaseGuard supervises a sent UE Context Release Command: armed when the command
	// is sent, stopped on the Release Complete; a lost Complete fires it once and runs
	// the EMMState-keyed local cleanup so the UeConn + M-TMSI cannot leak.
	releaseGuard guard.Guard
}

// StopReleaseGuard cancels the Release-Complete supervision timer. Nil-safe.
func (c *UeConn) StopReleaseGuard() {
	if c == nil {
		return
	}

	c.releaseGuard.Stop()
}

// Conn returns the UE's current UE-associated S1-connection, or nil when the UE
// is in ECM-IDLE. The atomic load is race-safe against a concurrent connection
// swap under MME.mu.
func (ue *UeContext) Conn() *UeConn {
	if ue == nil {
		return nil
	}

	return ue.active.Load()
}

// UeContext returns the persistent UE context bound to this connection, or nil
// for a bare connection whose first NAS message has not yet warranted one. Read on
// the dispatch goroutine, where the binding set under MME.mu is stable.
func (c *UeConn) UeContext() *UeContext {
	if c == nil {
		return nil
	}

	return c.ue
}

// SecureExchangeEstablished reports whether secure exchange of NAS messages is
// established on the connection (TS 24.301 §4.4.4.3).
func (c *UeConn) SecureExchangeEstablished() bool {
	if c == nil {
		return false
	}

	return c.secureExchangeEstablished
}

// MarkSecureExchangeEstablished records that secure exchange of NAS messages is
// established on the connection (TS 24.301 §4.4.4.3).
func (c *UeConn) MarkSecureExchangeEstablished() {
	if c != nil {
		c.secureExchangeEstablished = true
	}
}
