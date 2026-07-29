// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

// ptr returns a pointer to v, for setting the optional fields of a message from
// a literal. The module targets Go 1.24, which has no new(expr).
func ptr[T any](v T) *T { return &v }
