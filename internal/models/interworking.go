// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

// InterworkingWithoutN26 is the IWK N26 indicator both accesses advertise. The
// indication is PLMN-wide (TS 23.501 §5.17.2.3.1), so it is one value and not a
// per-subscriber or per-access decision, and each access sends it only to a UE
// that indicated support for the other (TS 24.301 §5.5.1.2.4, §5.5.3.2.4;
// TS 24.501 §5.5.1.2.4).
//
// It reads inverted from most feature bits: true means the network has no N26
// interface, so a UE moving between EPS and 5GS re-requests its session on the
// target access — with request type "handover" or "existing PDU session" — and
// the anchor keeps the session and its address. Ella Core has no N26 interface,
// so true is the only truthful value; when N26 handover ships, this becomes
// false and the UE is handed over instead.
const InterworkingWithoutN26 = true
