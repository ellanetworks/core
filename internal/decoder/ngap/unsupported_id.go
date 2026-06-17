// SPDX-FileCopyrightText: 2026-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package ngap

type UnsupportedIE struct {
	Status string `json:"status"`
}

func makeUnsupportedIE() *UnsupportedIE {
	return &UnsupportedIE{
		Status: "Unsupported",
	}
}
