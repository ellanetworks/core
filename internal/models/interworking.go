// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

// N26Supported reports whether this build runs the N26 interface between the AMF
// and the MME. It is the one switch for EPS ↔ 5GS interworking: everything the
// core tells a UE about interworking has to agree with it.
//
// TS 24.501 §5.4.2.3 defines what a UE does with the Selected EPS NAS security
// algorithms IE only for a UE "operating in single-registration mode", and
// §9.11.3.5's IWK N26 bit is what decides that mode: advertising interworking
// without N26 invites a dual-registration-capable UE into dual-registration mode,
// where the IE has no defined meaning. So the bit below and the delivery of the
// EPS NAS algorithms are two halves of one statement and are driven from here.
const N26Supported = false

// InterworkingWithoutN26 is the value of the IWK N26 bit in the 5GS and EPS
// network feature support IEs (TS 24.501 §9.11.3.5, TS 24.301 §9.9.3.12A), which
// names the inverse condition: the network offers interworking without N26
// exactly when it has no N26 interface to offer.
const InterworkingWithoutN26 = !N26Supported
