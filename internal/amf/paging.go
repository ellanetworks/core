// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

// SendPaging pages an idle UE and arms its paging-supervision timer. The timer is
// per-UE and persistent (T3513, TS 24.501 §5.6.2): paging targets a UE with no NAS
// connection, so the timer cannot live on the connection object.
func (amf *AMF) SendPaging(ctx context.Context, ue *UeContext, ngapBuf []byte) error {
	if ue == nil {
		return fmt.Errorf("amf ue is nil")
	}

	tmsi := ue.Tmsi()
	logger.From(ctx, logger.AmfLog).Info("Paging", logger.SUPI(ue.Supi().String()), zap.Uint32("5g-tmsi", tmsi.Uint32()))

	amf.pageRadios(ctx, ue, ngapBuf)
	amf.armPaging(ue, ngapBuf)

	return nil
}

// pageRadios sends the paging PDU to every radio whose supported TAIs intersect
// the UE's registration area.
func (amf *AMF) pageRadios(ctx context.Context, ue *UeContext, ngapBuf []byte) {
	taiList := ue.RegistrationArea

	for _, ran := range amf.ConnectedRadios() {
		for _, item := range ran.SupportedTAIList() {
			if InTaiList(item.Tai, taiList) {
				if err := amf.SendToRadio(ctx, ran.Conn, send.NGAPProcedurePaging, ngapBuf); err != nil {
					// The send failure is logged at the chokepoint.
					continue
				}

				break
			}
		}
	}
}

// StopAllTimers stops every timer on every UE, so no timer-driven activity fires while
// the system is tearing down.

// armPaging starts the paging-supervision guard for a UE just paged: retransmit Paging on
// each interval up to a bound, then abandon (T3513, TS 24.501 §5.6.2). Check-and-arm under
// the UE lock so a second downlink trigger cannot reset an in-flight supervision. No-op when
// T3513 is disabled.
func (amf *AMF) armPaging(ue *UeContext, ngapBuf []byte) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.pagingTimer.Active() {
		return
	}

	ue.pagingTimer.ArmWith(amf.T3513Cfg,
		func(attempt int32) { amf.retransmitPaging(ue, ngapBuf, attempt) },
		func() { amf.abandonPaging(ue) })
}

// retransmitPaging resends the Paging each guard interval (T3513, TS 24.501 §5.6.2), or
// stops the guard once the UE has answered by re-establishing its connection.
func (amf *AMF) retransmitPaging(ue *UeContext, ngapBuf []byte, attempt int32) {
	if ue.Conn() != nil {
		ue.pagingTimer.Stop()
		return
	}

	logger.AmfLog.Info("paging unanswered, retransmitting", logger.SUPI(ue.Supi().String()), zap.Int32("attempt", attempt))
	amf.pageRadios(context.Background(), ue, ngapBuf)
}

// abandonPaging suppresses the anchor's downlink data notification so further
// downlink packets do not re-page an unreachable UE (TS 23.502 §4.2.3.3).
func (amf *AMF) abandonPaging(ue *UeContext) {
	logger.AmfLog.Info("paging unanswered, abandoning procedure", logger.SUPI(ue.Supi().String()))

	if amf.Session == nil {
		return
	}

	supi := ue.Supi()

	for id := range ue.SmContextSnapshot() {
		if err := amf.Session.HandlePagingFailure(context.Background(), supi, id); err != nil {
			logger.AmfLog.Warn("failed to suppress downlink notification after paging failure",
				logger.SUPI(supi.String()), zap.Error(err))
		}
	}
}

// pageRadios sends the paging PDU to every radio whose supported TAIs intersect
// the UE's registration area.

// buildPaging assembles the Paging message for a UE (TS 38.413 §9.2.4.1). The
// TAI list for paging is the UE's registration area. Mirrors the MME's
// buildPaging.
func (amf *AMF) buildPaging(guami *models.Guami, ue *UeContext) (*ngap.Paging, error) {
	guti, err := amf.PagingGuti(guami, ue)
	if err != nil {
		return nil, fmt.Errorf("paging: build 5G-GUTI: %w", err)
	}

	taiList, err := areaToNGAPTAIs(ue.RegistrationArea)
	if err != nil {
		return nil, fmt.Errorf("paging: %w", err)
	}

	stmsi, err := fiveGSTMSIFor(guti)
	if err != nil {
		return nil, fmt.Errorf("paging: %w", err)
	}

	return &ngap.Paging{
		FiveGSTMSI:       stmsi,
		TAIListForPaging: taiList,
		// Replay the gNB-reported paging capability so it can apply paging
		// optimisations (TS 38.413 §9.3.1.68); omitted when none was reported.
		UERadioCapabilityForPaging: util.RadioCapabilityForPagingToNGAP(ue.RadioCapabilityForPaging),
	}, nil
}

func fiveGSTMSIFor(guti etsi.GUTI5G) (*ngap.FiveGSTMSI, error) {
	_, setID, pointer, err := util.AMFIDToNGAP(guti.Amfid)
	if err != nil {
		return nil, fmt.Errorf("split AMF id: %w", err)
	}

	tmsi, err := strconv.ParseUint(guti.Tmsi.String(), 16, 32)
	if err != nil {
		return nil, fmt.Errorf("parse 5G-TMSI: %w", err)
	}

	return &ngap.FiveGSTMSI{
		AMFSetID:   setID,
		AMFPointer: pointer,
		FiveGTMSI:  ngap.FiveGTMSI(tmsi),
	}, nil
}

func areaToNGAPTAIs(area []models.Tai) ([]ngap.TAI, error) {
	if len(area) == 0 {
		return nil, fmt.Errorf("empty registration area")
	}

	out := make([]ngap.TAI, 0, len(area))

	for _, t := range area {
		if t.PlmnID == nil {
			return nil, fmt.Errorf("registration-area TAI with no PLMN")
		}

		plmn, err := util.PLMNToNGAP(*t.PlmnID)
		if err != nil {
			return nil, fmt.Errorf("encode PLMN: %w", err)
		}

		tac, err := strconv.ParseUint(t.Tac, 16, 32)
		if err != nil {
			return nil, fmt.Errorf("parse TAC %q: %w", t.Tac, err)
		}

		out = append(out, ngap.TAI{PLMNIdentity: plmn, TAC: ngap.TAC(tac)})
	}

	return out, nil
}
