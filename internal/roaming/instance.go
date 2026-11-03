// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"strings"

	"github.com/google/uuid"
)

func NFInstanceID(fqdn string) string {
	return uuid.NewSHA1(uuid.NameSpaceDNS, []byte(strings.ToLower(fqdn))).String()
}
