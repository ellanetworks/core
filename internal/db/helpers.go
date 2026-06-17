// SPDX-FileCopyrightText: 2026-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package db

type ListArgs struct {
	Limit  int `db:"limit"`
	Offset int `db:"offset"`
}

type NumItems struct {
	Count int `db:"count"`
}

type cutoffArgs struct {
	Cutoff string `db:"cutoff"`
}

type cutoffDaysArgs struct {
	CutoffDays int64 `db:"cutoff_days"`
}

// SliceIDs is a named slice type used with sqlair's IN ($SliceIDs[:]) syntax.
type SliceIDs []string
