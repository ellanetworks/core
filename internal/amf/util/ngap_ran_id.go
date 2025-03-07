package util

import (
	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/ngap/ngapType"
)

func RanIDToModels(RanNodeID ngapType.GlobalRANNodeID) models.GlobalRanNodeID {
	var ranID models.GlobalRanNodeID
	present := RanNodeID.Present
	switch present {
	case ngapType.GlobalRANNodeIDPresentGlobalGNBID:
		ranID.GnbID = new(models.GnbID)
		gnbID := ranID.GnbID
		ngapGnbID := RanNodeID.GlobalGNBID
		plmnid := PlmnIDToModels(ngapGnbID.PLMNIdentity)
		ranID.PlmnID = &plmnid
		if ngapGnbID.GNBID.Present == ngapType.GNBIDPresentGNBID {
			choiceGnbID := ngapGnbID.GNBID.GNBID
			gnbID.BitLength = int32(choiceGnbID.BitLength)
			gnbID.GNBValue = BitStringToHex(choiceGnbID)
		}
	case ngapType.GlobalRANNodeIDPresentGlobalNgENBID:
		ngapNgENBID := RanNodeID.GlobalNgENBID
		plmnid := PlmnIDToModels(ngapNgENBID.PLMNIdentity)
		ranID.PlmnID = &plmnid
		if ngapNgENBID.NgENBID.Present == ngapType.NgENBIDPresentMacroNgENBID {
			macroNgENBID := ngapNgENBID.NgENBID.MacroNgENBID
			ranID.NgeNbID = "MacroNGeNB-" + BitStringToHex(macroNgENBID)
		} else if ngapNgENBID.NgENBID.Present == ngapType.NgENBIDPresentShortMacroNgENBID {
			shortMacroNgENBID := ngapNgENBID.NgENBID.ShortMacroNgENBID
			ranID.NgeNbID = "SMacroNGeNB-" + BitStringToHex(shortMacroNgENBID)
		} else if ngapNgENBID.NgENBID.Present == ngapType.NgENBIDPresentLongMacroNgENBID {
			longMacroNgENBID := ngapNgENBID.NgENBID.LongMacroNgENBID
			ranID.NgeNbID = "LMacroNGeNB-" + BitStringToHex(longMacroNgENBID)
		}
	case ngapType.GlobalRANNodeIDPresentGlobalN3IWFID:
		ngapN3IWFID := RanNodeID.GlobalN3IWFID
		plmnid := PlmnIDToModels(ngapN3IWFID.PLMNIdentity)
		ranID.PlmnID = &plmnid
		if ngapN3IWFID.N3IWFID.Present == ngapType.N3IWFIDPresentN3IWFID {
			choiceN3IWFID := ngapN3IWFID.N3IWFID.N3IWFID
			ranID.N3IwfID = BitStringToHex(choiceN3IWFID)
		}
	}

	return ranID
}
