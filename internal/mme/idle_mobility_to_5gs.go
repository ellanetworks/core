// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"encoding/binary"
	"fmt"
	"slices"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

func (m *MME) ServesGUTI(ctx context.Context, id eps.GUTI) bool {
	operator, err := m.Operator(ctx)
	if err != nil {
		return false
	}

	plmn := operator.PLMN()
	group, code := operator.GUMMEI()

	return id.PLMN.MCC == plmn.Mcc && id.PLMN.MNC == plmn.Mnc &&
		id.MMEGroupID == group && id.MMECode == code
}

func (m *MME) MMContext(ctx context.Context, req interworking.MMContextRequest) (interworking.MMContextResponse, error) {
	none := interworking.MMContextResponse{}

	ue, ok := m.LookupUeByMTMSI(binary.BigEndian.Uint32(req.MappedEPSGUTI.TMSI[:]))
	if !ok {
		return none, fmt.Errorf("%w: no context for M-TMSI %x", interworking.ErrUnknownUEContext, req.MappedEPSGUTI.TMSI)
	}

	if !m.ServesGUTI(ctx, req.MappedEPSGUTI) {
		return none, fmt.Errorf("%w: GUTI %s was not assigned by this MME", interworking.ErrUnknownUEContext, req.MappedEPSGUTI)
	}

	if ue.EMMState() != EMMRegistered || !ue.Secured() {
		return none, fmt.Errorf("%w: the context for M-TMSI %x is not a registered, secured one",
			interworking.ErrUnknownUEContext, req.MappedEPSGUTI.TMSI)
	}

	if !ue.FiveGSInterworkingAllowed() {
		return none, fmt.Errorf("%w: 5GC is restricted for the subscriber of M-TMSI %x",
			interworking.ErrUnknownUEContext, req.MappedEPSGUTI.TMSI)
	}

	plain, count, err := ue.TryUnprotectUplink(req.EPSNAS)
	if err != nil {
		return none, fmt.Errorf("%w: %w", interworking.ErrIntegrityCheckFailed, err)
	}

	if mt, err := eps.PeekMessageType(plain); err != nil || mt != eps.MsgTrackingAreaUpdateRequest {
		return none, fmt.Errorf("%w: the container holds no TRACKING AREA UPDATE REQUEST", interworking.ErrIntegrityCheckFailed)
	}

	ue.CommitUplinkCount(count)

	security, err := ue.EPSSecurityContextForRelocation()
	if err != nil {
		return none, err
	}

	connections, _ := TransferablePDNConnections(ue)

	ambrUL, ambrDL := ue.AmbrRates()

	ue.BeginIdleMobilityTo5GS()

	logger.From(ctx, logger.MmeLog).Info("handing the UE's EPS context to 5GS for an idle-mode change",
		zap.String("imsi", ue.IMSI()), zap.Int("pdn-connections", len(connections)))

	return interworking.MMContextResponse{
		SUPI:                ue.Supi(),
		Security:            security,
		UENetworkCapability: ue.UeNetCap(),
		PDNConnections:      connections,
		AMBRUplink:          ambrUL,
		AMBRDownlink:        ambrDL,
	}, nil
}

func (m *MME) MMContextAck(ctx context.Context, supi etsi.SUPI, transferred []uint8) error {
	ue, ok := m.LookupUeBySupi(supi)
	if !ok {
		return fmt.Errorf("%w: %s", interworking.ErrUnknownUEContext, supi)
	}

	ue.EndIdleMobilityTo5GS()

	for _, p := range m.SnapshotPDNs(ue) {
		if slices.Contains(transferred, p.PDUSessionID) {
			continue
		}

		logger.From(ctx, logger.MmeLog).Info("releasing a PDN connection 5GS did not adopt",
			zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", p.Ebi), zap.String("apn", p.Apn))
		m.ReleasePDN(ctx, ue, p)
	}

	m.DeregisterEmptyUE(ctx, ue)

	return nil
}
