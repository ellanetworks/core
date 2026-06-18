// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

type UnsupportedIE struct {
	Status string `json:"status"`
}

func makeUnsupportedIE() *UnsupportedIE {
	return &UnsupportedIE{
		Status: "Unsupported",
	}
}
