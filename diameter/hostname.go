// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import "strings"

const (
	maxHostnameLength      = 253
	maxHostnameLabelLength = 63
)

func IsValidHostname(name string) bool {
	if name == "" || len(name) > maxHostnameLength {
		return false
	}

	labels := strings.Split(name, ".")

	for _, label := range labels {
		if label == "" || len(label) > maxHostnameLabelLength || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}

		for i := range len(label) {
			c := label[i]
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' {
				return false
			}
		}
	}

	return strings.Trim(labels[len(labels)-1], "0123456789") != ""
}
