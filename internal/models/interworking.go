// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

// InterworkingWithoutN26 is the value of the IWK N26 indicator both the AMF and
// the MME advertise: set, EPS↔5GS interworking is provided without an N26
// interface, so a UE moving between the systems re-requests each connection on
// the access it arrives at and keeps its address (TS 23.501 §5.17.2.3).
//
// The indication is valid for the whole PLMN and the same one goes to every UE
// it serves (§5.17.2.3.1), so both accesses read it here rather than each
// holding its own. Each advertises it only to a UE that indicated it supports
// the other access (TS 24.501 §5.5.1.2.4, TS 24.301 §5.5.1.2.4).
const InterworkingWithoutN26 = true
