package ngap

import (
	"context"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleUplinkNasTransport(ctx context.Context, amf *amfContext.AMF, ran *amfContext.Radio, msg *ngapType.UplinkNASTransport) {
	if msg == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	var (
		aMFUENGAPID             *ngapType.AMFUENGAPID
		rANUENGAPID             *ngapType.RANUENGAPID
		nASPDU                  *ngapType.NASPDU
		userLocationInformation *ngapType.UserLocationInformation
	)

	for i := 0; i < len(msg.ProtocolIEs.List); i++ {
		ie := msg.ProtocolIEs.List[i]
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			aMFUENGAPID = ie.Value.AMFUENGAPID
			if aMFUENGAPID == nil {
				ran.Log.Error("AmfUeNgapID is nil")
				return
			}
		case ngapType.ProtocolIEIDRANUENGAPID:
			rANUENGAPID = ie.Value.RANUENGAPID
			if rANUENGAPID == nil {
				ran.Log.Error("RanUeNgapID is nil")
				return
			}
		case ngapType.ProtocolIEIDNASPDU:
			nASPDU = ie.Value.NASPDU
			if nASPDU == nil {
				ran.Log.Error("nASPDU is nil")
				return
			}
		case ngapType.ProtocolIEIDUserLocationInformation:
			userLocationInformation = ie.Value.UserLocationInformation
			if userLocationInformation == nil {
				ran.Log.Error("UserLocationInformation is nil")
				return
			}
		}
	}

	if aMFUENGAPID == nil {
		ran.Log.Error("AMFUENGAPID IE (mandatory) is missing in UplinkNASTransport")
		return
	}

	if rANUENGAPID == nil {
		ran.Log.Error("RANUENGAPID IE (mandatory) is missing in UplinkNASTransport")
		return
	}

	if nASPDU == nil {
		ran.Log.Error("NASPDU IE (mandatory) is missing in UplinkNASTransport")
		return
	}

	ranAddr := ""
	if ran.Conn != nil && ran.Conn.RemoteAddr() != nil {
		ranAddr = ran.Conn.RemoteAddr().String()
	}

	ranUe := ran.FindUEByRanUeNgapID(rANUENGAPID.Value)
	if ranUe == nil {
		ran.Log.Error("ran ue is nil",
			logger.ErrorCodeField("amf_ran_ue_context_nil"),
			zap.Int64("ran_ue_ngap_id", rANUENGAPID.Value),
			zap.Int("nas_pdu_len", len(nASPDU.Value)),
		)

		return
	}

	ranUe.Radio = ran
	ranUe.TouchLastSeen()

	amfUe := ranUe.AmfUe
	if amfUe == nil {
		err := ranUe.Remove()
		if err != nil {
			ran.Log.Error("error removing ran ue context", zap.Error(err))
		}

		ran.Log.Error("No UE Context of RanUe",
			logger.ErrorCodeField("amf_ue_context_missing_for_ran_ue"),
			zap.Int64("ran_ue_ngap_id", rANUENGAPID.Value),
			zap.Int64("amf_ue_ngap_id", aMFUENGAPID.Value),
			zap.Int("nas_pdu_len", len(nASPDU.Value)),
		)

		return
	}

	if userLocationInformation != nil {
		ranUe.UpdateLocation(ctx, amf, userLocationInformation)
	}

	err := nas.HandleNAS(ctx, amf, ranUe, nASPDU.Value)
	if err != nil {
		fields := logger.UEIdentityFields(
			amfUe.Supi.String(),
			amfUe.Guti.String(),
			ranUe.AmfUeNgapID,
			ranUe.RanUeNgapID,
			ranAddr,
		)
		fields = append(fields,
			logger.ErrorCodeField("amf_nas_handling_failed"),
			zap.Int("nas_pdu_len", len(nASPDU.Value)),
			zap.Error(err),
		)
		ranUe.Log.Error("error handling NAS message", fields...)
	}
}
