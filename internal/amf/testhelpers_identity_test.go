// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// testMobileIdentity is the 5G-GUTI these tests present as the UE identity; the
// value is immaterial to what they assert, only that one is present.
func testMobileIdentity() fgs.MobileIdentity {
	return fgs.GUTIIdentity(fgs.GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, AMFRegionID: 1, AMFSetID: 1, AMFPointer: 1})
}
