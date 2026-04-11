package ngap

import (
	gocontext "context"
	"encoding/hex"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

func HandleUERadioCapabilityInfoIndication(ctx gocontext.Context, ran *amf.Radio, msg decode.UERadioCapabilityInfoIndication) {
	ranUe := ran.FindUEByRanUeNgapID(msg.RANUENGAPID)
	if ranUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("No UE Context", zap.Int64("RanUeNgapID", msg.RANUENGAPID))
		return
	}

	logger.WithTrace(ctx, ranUe.Log).Debug("Handle UE Radio Capability Info Indication", zap.Int64("RanUeNgapID", ranUe.RanUeNgapID), zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID))
	ranUe.TouchLastSeen()

	amfUe := ranUe.AmfUe()
	if amfUe == nil {
		logger.WithTrace(ctx, ranUe.Log).Error("amfUe is nil")
		return
	}

	if msg.UERadioCapability != nil {
		amfUe.UeRadioCapability = hex.EncodeToString(msg.UERadioCapability)
	}

	if msg.UERadioCapabilityForPaging != nil {
		amfUe.UeRadioCapabilityForPaging = &models.UERadioCapabilityForPaging{}
		if msg.UERadioCapabilityForPaging.UERadioCapabilityForPagingOfNR != nil {
			amfUe.UeRadioCapabilityForPaging.NR = hex.EncodeToString(
				msg.UERadioCapabilityForPaging.UERadioCapabilityForPagingOfNR.Value)
		}

		if msg.UERadioCapabilityForPaging.UERadioCapabilityForPagingOfEUTRA != nil {
			amfUe.UeRadioCapabilityForPaging.EUTRA = hex.EncodeToString(
				msg.UERadioCapabilityForPaging.UERadioCapabilityForPagingOfEUTRA.Value)
		}
	}

	// TS 38.413 8.14.1.2/TS 23.502 4.2.8a step5/TS 23.501, clause 5.4.4.1.
	// send its most up to date UE Radio Capability information to the RAN in the N2 REQUEST message.
}
