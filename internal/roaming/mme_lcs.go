// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"fmt"

	"github.com/ellanetworks/core/diameter/s6a"
)

func CheckLCSPrivacy4G(sub *s6a.SubscriptionData) error {
	switch {
	case sub == nil || sub.LCSInfo == nil:
		return fmt.Errorf("%w: the home HSS sent no LCS-Info", ErrLocationPrivacy)
	case !sub.LCSInfo.AllowsPLMNClient(s6a.PLMNClientOAndMVPLMN):
		return fmt.Errorf("%w: the plmnoperator privacy exception does not list %s", ErrLocationPrivacy, s6a.PLMNClientOAndMVPLMN)
	default:
		return nil
	}
}
