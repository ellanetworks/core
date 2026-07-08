// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package nasreply models the single outcome every inbound NAS message resolves to, shared by
// the AMF (5GMM/5GSM) and MME (EMM/ESM). It turns "reply unless the spec mandates silence"
// from a convention spread across many void return sites into one typed value a finalizer
// must consume, so an unhandled message can no longer silently vanish.
package nasreply

import "context"

// Action is what the network does with a message it has finished classifying.
type Action uint8

const (
	ActionHandled Action = iota // a downlink was already produced (an accept, or a procedure reject)
	ActionSilent                // the spec mandates silence — no reply
	ActionStatus                // reply with a mobility- or session-management STATUS
)

// Domain selects which STATUS an ActionStatus disposition sends.
type Domain uint8

const (
	DomainMM Domain = iota // 5GMM (AMF) / EMM (MME) STATUS
	DomainSM               // 5GSM (AMF) / ESM (MME) STATUS
)

// Reason records why a message was silently discarded. It never reaches the wire; it exists
// so every spec-mandated silence is explicit and auditable in one place.
type Reason uint8

const (
	ReasonUnspecified   Reason = iota
	ReasonIntegrityFail        // §4.4.4.3: forged/replayed, or plain and not on the exempt list
	ReasonTooShort             // §7.2.1: too short to carry a message type
	ReasonOutOfState           // no procedure expects this message (§7.4, network action implementation-dependent)
	ReasonNoContext            // cites a UE or session context the network does not hold
)

func (r Reason) String() string {
	switch r {
	case ReasonIntegrityFail:
		return "integrity check failed or plain non-exempt"
	case ReasonTooShort:
		return "message too short"
	case ReasonOutOfState:
		return "no procedure expects this message"
	case ReasonNoContext:
		return "no context for the cited identity"
	default:
		return "unspecified"
	}
}

// Cause values a STATUS carries. The numbers are identical in TS 24.301 and TS 24.501.
const (
	CauseInvalidMandatoryInfo      uint8 = 96
	CauseMessageTypeNotImplemented uint8 = 97
	CauseProtocolErrorUnspecified  uint8 = 111
)

// Disposition is the single outcome every inbound NAS message resolves to.
type Disposition struct {
	Action Action
	Domain Domain
	Cause  uint8
	Reason Reason
}

// Handled reports that the handler already produced its own downlink (an accept or a
// procedure reject), so the finalizer sends nothing further.
func Handled() Disposition { return Disposition{Action: ActionHandled} }

// Silent reports a spec-mandated silence, tagged with the auditable reason.
func Silent(r Reason) Disposition { return Disposition{Action: ActionSilent, Reason: r} }

// StatusMM asks the finalizer to answer with a 5GMM/EMM STATUS.
func StatusMM(cause uint8) Disposition {
	return Disposition{Action: ActionStatus, Domain: DomainMM, Cause: cause}
}

// StatusSM asks the finalizer to answer with a 5GSM/ESM STATUS.
func StatusSM(cause uint8) Disposition {
	return Disposition{Action: ActionStatus, Domain: DomainSM, Cause: cause}
}

// Egress sends a domain STATUS, or audits a discard, on the connection a NAS message arrived
// on — including a bare (context-free) connection, so an unresolvable peer still receives the
// STATUS the spec mandates. It is implemented once per RAT.
type Egress interface {
	SendMMStatus(ctx context.Context, cause uint8)
	SendSMStatus(ctx context.Context, cause uint8)
	Discard(ctx context.Context, reason Reason)
}

// Finalize turns a resolved disposition into the wire action: a STATUS, or an audited
// silence. ActionHandled sends nothing (the handler already replied).
func (d Disposition) Finalize(ctx context.Context, e Egress) {
	switch d.Action {
	case ActionStatus:
		if d.Domain == DomainSM {
			e.SendSMStatus(ctx, d.Cause)
			return
		}

		e.SendMMStatus(ctx, d.Cause)
	case ActionSilent:
		e.Discard(ctx, d.Reason)
	}
}
