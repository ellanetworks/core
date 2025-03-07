package util

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/models"
)

// nasType: TS 24.501 9.11.3.4
func GutiToString(buf []byte) (models.Guami, string) {
	plmnID := PlmnIDToString(buf[1:4])
	amfID := hex.EncodeToString(buf[4:7])
	tmsi5G := hex.EncodeToString(buf[7:])
	var guami models.Guami
	guami.PlmnId = new(models.PlmnId)
	guami.PlmnId.Mcc = plmnID[:3]
	guami.PlmnId.Mnc = plmnID[3:]
	guami.AmfId = amfID
	guti := plmnID + amfID + tmsi5G
	return guami, guti
}
