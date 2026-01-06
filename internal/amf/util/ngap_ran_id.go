package util

import (
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
)

func RanIDToModels(ranNodeID ngapType.GlobalRANNodeID) models.GlobalRanNodeID {
	var ranID models.GlobalRanNodeID

	present := ranNodeID.Present
	switch present {
	case ngapType.GlobalRANNodeIDPresentGlobalGNBID:
		ranID.GNbID = new(models.GNbID)
		gnbID := ranID.GNbID
		ngapGnbID := ranNodeID.GlobalGNBID

		if ngapGnbID.GNBID.Present == ngapType.GNBIDPresentGNBID {
			choiceGnbID := ngapGnbID.GNBID.GNBID
			gnbID.BitLength = int32(choiceGnbID.BitLength)
			gnbID.GNBValue = ngapConvert.BitStringToHex(choiceGnbID)
		}
	case ngapType.GlobalRANNodeIDPresentGlobalNgENBID:
		ngapNgENBID := ranNodeID.GlobalNgENBID

		switch ngapNgENBID.NgENBID.Present {
		case ngapType.NgENBIDPresentMacroNgENBID:
			macroNgENBID := ngapNgENBID.NgENBID.MacroNgENBID
			ranID.NgeNbID = "MacroNGeNB-" + ngapConvert.BitStringToHex(macroNgENBID)
		case ngapType.NgENBIDPresentShortMacroNgENBID:
			shortMacroNgENBID := ngapNgENBID.NgENBID.ShortMacroNgENBID
			ranID.NgeNbID = "SMacroNGeNB-" + ngapConvert.BitStringToHex(shortMacroNgENBID)
		case ngapType.NgENBIDPresentLongMacroNgENBID:
			longMacroNgENBID := ngapNgENBID.NgENBID.LongMacroNgENBID
			ranID.NgeNbID = "LMacroNGeNB-" + ngapConvert.BitStringToHex(longMacroNgENBID)
		}
	case ngapType.GlobalRANNodeIDPresentGlobalN3IWFID:
		ngapN3IWFID := ranNodeID.GlobalN3IWFID

		if ngapN3IWFID.N3IWFID.Present == ngapType.N3IWFIDPresentN3IWFID {
			choiceN3IWFID := ngapN3IWFID.N3IWFID.N3IWFID
			ranID.N3IwfID = ngapConvert.BitStringToHex(choiceN3IWFID)
		}
	}

	return ranID
}
