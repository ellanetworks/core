package util

import (
	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/ngap/ngapType"
)

func RanIDToModels(ranNodeID ngapType.GlobalRANNodeID) (ranId models.GlobalRanNodeId) {
	present := ranNodeID.Present
	switch present {
	case ngapType.GlobalRANNodeIDPresentGlobalGNBID:
		ranId.GnbID = new(models.GnbID)
		gnbID := ranId.GnbID
		ngapGnbID := ranNodeID.GlobalGNBID
		plmnid := PlmnIDToModels(ngapGnbID.PLMNIdentity)
		ranId.PlmnID = &plmnid
		if ngapGnbID.GNBID.Present == ngapType.GNBIDPresentGNBID {
			choiceGnbID := ngapGnbID.GNBID.GNBID
			gnbID.BitLength = int32(choiceGnbID.BitLength)
			gnbID.GNBValue = BitStringToHex(choiceGnbID)
		}
	case ngapType.GlobalRANNodeIDPresentGlobalNgENBID:
		ngapNgENBID := ranNodeID.GlobalNgENBID
		plmnid := PlmnIDToModels(ngapNgENBID.PLMNIdentity)
		ranId.PlmnID = &plmnid
		if ngapNgENBID.NgENBID.Present == ngapType.NgENBIDPresentMacroNgENBID {
			macroNgENBID := ngapNgENBID.NgENBID.MacroNgENBID
			ranId.NgeNbId = "MacroNGeNB-" + BitStringToHex(macroNgENBID)
		} else if ngapNgENBID.NgENBID.Present == ngapType.NgENBIDPresentShortMacroNgENBID {
			shortMacroNgENBID := ngapNgENBID.NgENBID.ShortMacroNgENBID
			ranId.NgeNbId = "SMacroNGeNB-" + BitStringToHex(shortMacroNgENBID)
		} else if ngapNgENBID.NgENBID.Present == ngapType.NgENBIDPresentLongMacroNgENBID {
			longMacroNgENBID := ngapNgENBID.NgENBID.LongMacroNgENBID
			ranId.NgeNbId = "LMacroNGeNB-" + BitStringToHex(longMacroNgENBID)
		}
	case ngapType.GlobalRANNodeIDPresentGlobalN3IWFID:
		ngapN3IWFID := ranNodeID.GlobalN3IWFID
		plmnid := PlmnIDToModels(ngapN3IWFID.PLMNIdentity)
		ranId.PlmnID = &plmnid
		if ngapN3IWFID.N3IWFID.Present == ngapType.N3IWFIDPresentN3IWFID {
			choiceN3IWFID := ngapN3IWFID.N3IWFID.N3IWFID
			ranId.N3IwfId = BitStringToHex(choiceN3IWFID)
		}
	}

	return ranId
}
