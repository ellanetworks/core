// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameternode

import (
	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/rx"
	"github.com/ellanetworks/core/diameter/s6c"
	"github.com/ellanetworks/core/diameter/sgd"
	"github.com/ellanetworks/core/diameter/sh"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const (
	PeerRoleIMS  = "ims"
	PeerRoleSMSC = "smsc"
)

const etsiVendorID uint32 = 13019

var imsApplications = []diameter.Application{
	{ID: cx.ApplicationID, VendorID: tgpp.VendorID},
	{ID: rx.ApplicationID, VendorID: tgpp.VendorID},
	{ID: sh.ApplicationID, VendorID: tgpp.VendorID},
	{ID: s6c.ApplicationID, VendorID: tgpp.VendorID},
	{ID: sgd.ApplicationID, VendorID: tgpp.VendorID},
}

var imsSupportedVendors = []uint32{tgpp.VendorID, etsiVendorID}
