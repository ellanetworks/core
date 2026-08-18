// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
)

// deregisterToEPS is the NGAP cause carried by the UE Context Release Command of a UE
// that left 5GS other than by handover: the release follows the AMF dropping the UE's
// 5GS registration (TS 38.413 §9.3.1.2, NAS cause "Deregister").
var deregisterToEPS = ngap.Cause{Group: ngap.CauseGroupNAS, Value: ngap.CauseNASDeregister}

// releaseToEPS gives a UE up to EPS. Its 5GS registration, PDU sessions and key-chain
// procedures go; its 5G-GUTI and 5G security context stay, so a return to 5GS reuses them
// instead of costing a primary authentication (TS 23.501 §5.17.2.1, TS 33.501 §8.3.2).
//
// conn is the N2 connection the move leaves behind — the handover source for a handover,
// ue.Conn() otherwise. Releasing it is what keeps the gNB from holding a UE context the
// AMF has already given up (TS 38.413 §8.3.1); nothing else reaps it, because the UE
// context deliberately outlives the connection here.
func (a *AMF) releaseToEPS(ctx context.Context, ue *UeContext, conn *UeConn, cause ngap.Cause) {
	if conn == nil {
		conn = ue.Conn()
	}

	ue.RetainForEPS(epsContextRetention)
	ue.Deregister(ctx)
	a.StartMobileReachable(ue)

	if conn == nil {
		return
	}

	conn.ReleaseAction = UeContextReleaseToEPS
	conn.SendUEContextReleaseCommand(ctx, cause)
}

// CancelRegistration drops the UE's 5GS registration because the MME has just registered
// the same subscriber in EPS. With N26 configured the network keeps only one valid MM
// state per subscriber on 3GPP access (TS 23.501 §5.17.2.2.1), and the EPS one is now the
// live half.
func (a *AMF) CancelRegistration(ctx context.Context, supi etsi.SUPI) {
	ue, ok := a.LookupUeBySupi(supi)
	if !ok || ue.State() != Registered {
		return
	}

	if a.HandoverToEPSInProgress(ue) || a.RelocationFromEPSInProgress(supi) {
		logger.From(ctx, logger.AmfLog).Debug("leaving the 5GS registration to the interworking procedure that already owns it",
			logger.SUPI(supi.String()))

		return
	}

	a.releaseToEPS(ctx, ue, nil, deregisterToEPS)

	logger.From(ctx, logger.AmfLog).Info("UE attached in EPS; dropping its 5GS registration and keeping its 5G security context for a return",
		logger.SUPI(supi.String()))
}

func (a *AMF) RelocationFromEPSInProgress(supi etsi.SUPI) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	_, busy := a.relocatingFromEPS[supi]

	return busy
}

// MarkRegistered puts the UE in 5GMM-REGISTERED and supersedes the EPS half of the
// subscriber's registration as the 5GS half takes effect. Every registration procedure
// that reaches 5GMM-REGISTERED runs through here, including the one whose REGISTRATION
// COMPLETE never arrives and is written off by T3550, so a lost acknowledgement cannot
// leave the subscriber registered on both accesses.
//
// A UE arriving by handover from EPS transitions inside the relocation instead: that
// procedure owns both halves of the registration and sends the MME its own completion.
func (a *AMF) MarkRegistered(ctx context.Context, ue *UeContext) {
	ue.TransitionTo(Registered)

	// An illegal transition drops the UE to Deregistered instead. Superseding then would
	// take the subscriber's EPS registration away in exchange for a 5GS one it never got.
	if ue.State() != Registered {
		return
	}

	a.SupersedeEPSRegistration(ctx, ue)
}

// SupersedeEPSRegistration asks the MME to drop any EPS registration the subscriber still
// holds, now that it is registered in 5GS.
//
// The UE status IE deliberately plays no part in this decision. Its EMM registration
// status bit does not mean "keep my EPS registration": TS 24.501 §5.5.1.3.2 a) requires a
// single-registration-mode UE performing an inter-system change from S1 mode to N1 mode to
// set it, and NOTE 7 names that the "moving from EPC" indication — the very UE whose EPS
// registration must go. It carries a second meaning for a dual-registration-mode UE
// (§5.5.1.2.2, NOTE 3), but dual registration does not apply here: this runs only with N26
// wired, and TS 23.501 §5.17.2.2.1 states that with N26 the UE operates in
// single-registration mode and "the network keeps only one valid MM state for the UE,
// either in the AMF or MME".
//
// The one thing that does defer is an interworking procedure already moving the UE, which
// owns both halves of the registration for its duration.
func (a *AMF) SupersedeEPSRegistration(ctx context.Context, ue *UeContext) {
	if a.EPS == nil || ue == nil {
		return
	}

	supi := ue.Supi()
	if !supi.IsValid() {
		return
	}

	if a.RelocationFromEPSInProgress(supi) {
		return
	}

	a.EPS.CancelRegistration(ctx, supi)
}
