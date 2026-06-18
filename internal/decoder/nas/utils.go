// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

type UnsupportedIE struct {
	Status string `json:"status"`
}

func makeUnsupportedIE() *UnsupportedIE {
	return &UnsupportedIE{
		Status: "Unsupported",
	}
}
