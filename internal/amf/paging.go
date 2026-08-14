// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf/procedure"
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
				if err := amf.SendToRadio(ctx, ran.Conn, NGAPProcedurePaging, ngapBuf); err != nil {
					// The send failure is logged at the chokepoint.
					continue
				}

				break
			}
		}
	}
}

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

	// Backstop for standalone signalling whose consumer went away without cancelling. A
	// session-scoped buffer is left alone: it stays valid if the UE returns on its own.
	if req := ue.N1N2Message(); req != nil && req.Standalone() && ue.Conn() == nil {
		ue.ClearN1N2Message()
	}

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

// pageIdleUE pages an idle UE and starts paging supervision (TS 23.502 §4.2.3.3). A
// non-nil req is buffered for delivery when the UE answers. Callers should guard 
// first, via guardIdlePaging.
func (amf *AMF) pageIdleUE(ctx context.Context, ue *UeContext, req *models.N1N2MessageTransferRequest) error {
	if amf.DBInstance == nil {
		return fmt.Errorf("AMF not configured with database, cannot page")
	}

	operatorInfo, err := amf.OperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("get operator info: %w", err)
	}

	paging, err := amf.buildPaging(operatorInfo.Guami, ue)
	if err != nil {
		return fmt.Errorf("build paging: %w", err)
	}

	pkg, err := paging.Marshal()
	if err != nil {
		return fmt.Errorf("marshal paging: %w", err)
	}

	// Buffer immediately before the send: the UE may answer on another goroutine the
	// moment the Paging leaves the AMF.
	if req != nil {
		ue.SetN1N2Message(req)
	}

	if err := amf.SendPaging(ctx, ue, pkg); err != nil {
		if req != nil {
			ue.ClearN1N2Message()
		}

		return fmt.Errorf("send paging: %w", err)
	}

	return nil
}

// errPagingActive is returned when paging supervision is already in flight for the UE.
var errPagingActive = fmt.Errorf("paging already in progress")

// guardIdlePaging rejects paging a UE that is already connected, mid-registration,
// mid-handover, or not registered.
func guardIdlePaging(ue *UeContext) error {
	if ue.Conn() != nil {
		return fmt.Errorf("ue is already CM-CONNECTED")
	}

	if ue.State() == RegistrationInitiated {
		return fmt.Errorf("temporary reject: registration ongoing")
	}

	if ue.Procedures().Active(procedure.N2Handover) {
		return fmt.Errorf("temporary reject: handover ongoing")
	}

	if ue.State() != Registered {
		return fmt.Errorf("ue is not in registered state")
	}

	return nil
}

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
