// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

type Tai struct {
	PlmnID *PlmnID
	Tac    string
}

func (t Tai) Equal(other Tai) bool {
	if t.PlmnID == nil || other.PlmnID == nil {
		return t.PlmnID == other.PlmnID && t.Tac == other.Tac
	}

	return t.PlmnID.Equal(*other.PlmnID) && t.Tac == other.Tac
}
