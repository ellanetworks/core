// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// HandleERABSetupResponse processes the eNB's answer to an E-RAB SETUP REQUEST
// (TS 36.413 §8.2.1): it records the eNB S1-U endpoint of each established E-RAB
// on the anchor session, and releases any E-RAB the eNB failed to set up.
func HandleERABSetupResponse(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseERABSetupResponse(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode E-RAB Setup Response", zap.Error(err))
		return
	}

	ue, ok := resolveUE(m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	ue.TouchLastSeen()

	for _, erab := range msg.ERABSetup {
		p := m.LookupPDN(ue, uint8(erab.ERABID))
		if p == nil {
			ue.Conn().Log.Warn("E-RAB Setup Response for an unknown E-RAB",
				zap.Uint8("e-rab-id", uint8(erab.ERABID)))

			continue
		}

		enbAddr, ok := enbTransportAddress(erab.TransportLayerAddress)
		if !ok {
			ue.Conn().Log.Warn("E-RAB Setup Response with an invalid eNB transport address",
				zap.Uint8("e-rab-id", uint8(erab.ERABID)))

			continue
		}

		p.EnbFTEID = models.FTEID{TEID: uint32(erab.GTPTEID), Addr: enbAddr}

		if err := m.Session.ModifyEPSSession(ctx, ue.IMSI(), p.Ebi, p.EnbFTEID); err != nil {
			logger.MmeLog.Error("failed to set the eNB F-TEID on the additional EPS session",
				zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", p.Ebi), zap.Error(err))

			continue
		}

		logger.MmeLog.Info("additional PDN connection radio leg established",
			zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn), zap.Uint8("ebi", p.Ebi),
			zap.String("enb-s1u", enbAddr.String()))
	}

	for _, erab := range msg.ERABFailedToSetup {
		if p := m.LookupPDN(ue, uint8(erab.ERABID)); p != nil {
			logger.MmeLog.Warn("eNB failed to set up an additional E-RAB; releasing the PDN connection",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", uint8(erab.ERABID)))
			m.ReleasePDN(ctx, ue, p)
		}
	}
}
