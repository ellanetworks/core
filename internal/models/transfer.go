// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import "errors"

// Why a move of a session between accesses was refused (TS 23.502 §4.11.2).
// They sit on the MME/SMF boundary alongside EPSBearerRequest because each
// access maps them to a NAS cause of its own, and neither NF imports the other.
var (
	// ErrSessionNotTransferable reports that no session answers the identity the
	// UE named. Each access maps it to #54, which tells the UE the network has no
	// information about the session, so it establishes a new one rather than
	// retrying (TS 24.301 §6.5.1.6 b), TS 24.501 §6.4.1.7 d).
	ErrSessionNotTransferable = errors.New("session does not exist on the other access")

	// ErrSessionOnOtherDNN reports a session that exists on a different data
	// network from the one the UE named.
	ErrSessionOnOtherDNN = errors.New("session is on another data network")

	// ErrSessionNotMovable reports a session that exists and matches, but cannot
	// move as the request describes. #54 would be untrue here — the network does
	// know the session — so this draws #26 instead, which is retryable.
	ErrSessionNotMovable = errors.New("session cannot move as described")
)
