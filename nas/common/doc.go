// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package common implements the generation-agnostic NAS message-format
// primitives of TS 24.007 §11.2: a bounds-checked octet Reader/Writer, the
// length-prefixed value helpers (LV with a 1-octet length, LV-E with a 2-octet
// length) shared by EPS and 5GS NAS, and TBCD / PLMN identity coding.
//
// It carries no message or IE definitions — those live in the per-generation
// packages (eps, and a future 5gs). Every Reader operation is bounds-checked and
// returns an error rather than panicking on malformed input.
package common
