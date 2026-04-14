// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package procedure

// Conflict matrix encoding TS 33.501 §6.9.5.1 rules.
//
// conflicts(active, incoming) returns (blocked, ruleCode).
// The matrix is asymmetric: row = active, column = incoming.
//
// Legend:
//   C1  — Rule 1: no N2-new-key while SMC ongoing
//   C2  — Rule 2: no SMC while N2-new-key ongoing
//   C4  — Rule 4: UE Context Modification with new KgNB vs. inter-AMF handover

type matrixEntry struct {
	blocked bool
	rule    string
}

// matrixKey is active×incoming.
type matrixKey struct {
	active, incoming Type
}

var matrix = map[matrixKey]matrixEntry{
	// SecurityMode (active) vs N2Handover (incoming) — Rule 1
	{SecurityMode, N2Handover}: {blocked: true, rule: "C1"},
	// N2Handover (active) vs SecurityMode (incoming) — Rule 2
	{N2Handover, SecurityMode}: {blocked: true, rule: "C2"},
	// N2Handover (active) vs UEContextMod (incoming) — Rule 4
	{N2Handover, UEContextMod}: {blocked: true, rule: "C4"},
	// UEContextMod (active) vs N2Handover (incoming) — Rule 4
	{UEContextMod, N2Handover}: {blocked: true, rule: "C4"},
}

func conflicts(active, incoming Type) (blocked bool, rule string) {
	if e, ok := matrix[matrixKey{active, incoming}]; ok {
		return e.blocked, e.rule
	}

	return false, ""
}

// isReentrant returns true for procedure types that allow multiple
// concurrent instances (e.g. Paging for different PDU sessions).
func isReentrant(t Type) bool {
	return t == Paging
}
