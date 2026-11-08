// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import "regexp"

var networkInstanceNameRegex = regexp.MustCompile(`^([a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)(\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*$`)

func ValidNetworkInstanceName(name string) bool {
	return len(name) <= 62 && networkInstanceNameRegex.MatchString(name)
}
