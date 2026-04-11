// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// decodeGlobalRANNodeID validates the CHOICE structure of a
// GlobalRANNodeID IE and the inner CHOICE of the selected variant. On
// success, the variant pointer matching the discriminator is non-nil
// and any nested *aper.BitString is non-nil, so callers (notably
// util.RanIDToModels) can deref without further checks.
func decodeGlobalRANNodeID(in *ngapType.GlobalRANNodeID) (GlobalRANNodeID, bool) {
	if in == nil {
		return GlobalRANNodeID{}, false
	}

	switch in.Present {
	case ngapType.GlobalRANNodeIDPresentGlobalGNBID:
		if in.GlobalGNBID == nil {
			return GlobalRANNodeID{}, false
		}

		if in.GlobalGNBID.GNBID.Present != ngapType.GNBIDPresentGNBID {
			return GlobalRANNodeID{}, false
		}

		if in.GlobalGNBID.GNBID.GNBID == nil {
			return GlobalRANNodeID{}, false
		}

		return GlobalRANNodeID{kind: GlobalRANNodeKindGNB, raw: in}, true

	case ngapType.GlobalRANNodeIDPresentGlobalNgENBID:
		if in.GlobalNgENBID == nil {
			return GlobalRANNodeID{}, false
		}

		switch in.GlobalNgENBID.NgENBID.Present {
		case ngapType.NgENBIDPresentMacroNgENBID:
			if in.GlobalNgENBID.NgENBID.MacroNgENBID == nil {
				return GlobalRANNodeID{}, false
			}
		case ngapType.NgENBIDPresentShortMacroNgENBID:
			if in.GlobalNgENBID.NgENBID.ShortMacroNgENBID == nil {
				return GlobalRANNodeID{}, false
			}
		case ngapType.NgENBIDPresentLongMacroNgENBID:
			if in.GlobalNgENBID.NgENBID.LongMacroNgENBID == nil {
				return GlobalRANNodeID{}, false
			}
		default:
			return GlobalRANNodeID{}, false
		}

		return GlobalRANNodeID{kind: GlobalRANNodeKindNgENB, raw: in}, true

	case ngapType.GlobalRANNodeIDPresentGlobalN3IWFID:
		if in.GlobalN3IWFID == nil {
			return GlobalRANNodeID{}, false
		}

		if in.GlobalN3IWFID.N3IWFID.Present != ngapType.N3IWFIDPresentN3IWFID {
			return GlobalRANNodeID{}, false
		}

		if in.GlobalN3IWFID.N3IWFID.N3IWFID == nil {
			return GlobalRANNodeID{}, false
		}

		return GlobalRANNodeID{kind: GlobalRANNodeKindN3IWF, raw: in}, true

	default:
		return GlobalRANNodeID{}, false
	}
}
