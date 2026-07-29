// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// testGUTI is the Old GUTI these tests present in a TRACKING AREA UPDATE
// REQUEST; the value is immaterial to what they assert, only that one is present.
func testGUTI() eps.EPSMobileIdentity {
	return eps.GUTIIdentity(eps.GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 1, MMECode: 1, TMSI: [4]byte{0x00, 0x00, 0x00, 0x01}})
}
