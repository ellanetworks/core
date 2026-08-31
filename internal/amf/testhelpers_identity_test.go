// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

func testMobileIdentity() fgs.MobileIdentity {
	return fgs.GUTIIdentity(fgs.GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, AMFRegionID: 1, AMFSetID: 1, AMFPointer: 1})
}
