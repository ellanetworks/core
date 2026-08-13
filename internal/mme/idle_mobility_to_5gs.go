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

// ServesGUTI reports whether a 4G-GUTI was assigned by this node: its serving
// PLMN and its GUMMEI (TS 23.003). A foreign GUTI would require S10, which Ella
// Core does not implement.
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

// MMContext answers the AMF's request for the UE's EPS context on an idle-mode
// inter-system change to 5GS (TS 23.502 §4.11.1.3.3 step 5a).
//
// The MME is the only node holding the EPS security context the UE protected
// its TRACKING AREA UPDATE REQUEST with, so verifying that message here is what
// authenticates the whole move: the AMF acts on the SUPI this returns
// (TS 33.501 §8.2). The count it verified at is committed before the context
// goes out, so a replayed REGISTRATION REQUEST carrying the same container
// cannot re-derive K'AMF.
func (m *MME) MMContext(ctx context.Context, req interworking.MMContextRequest) (interworking.MMContextResponse, error) {
	none := interworking.MMContextResponse{}

	if !m.ServesGUTI(ctx, req.MappedEPSGUTI) {
		return none, fmt.Errorf("%w: GUTI %s was not assigned by this MME", interworking.ErrUnknownUEContext, req.MappedEPSGUTI)
	}

	ue, ok := m.LookupUeByMTMSI(binary.BigEndian.Uint32(req.MappedEPSGUTI.TMSI[:]))
	if !ok {
		return none, fmt.Errorf("%w: no context for M-TMSI %x", interworking.ErrUnknownUEContext, req.MappedEPSGUTI.TMSI)
	}

	if ue.EMMState() != EMMRegistered || !ue.Secured() {
		return none, fmt.Errorf("%w: the context for M-TMSI %x is not a registered, secured one",
			interworking.ErrUnknownUEContext, req.MappedEPSGUTI.TMSI)
	}

	plain, count, err := ue.TryUnprotectUplink(req.EPSNAS)
	if err != nil {
		return none, fmt.Errorf("%w: %w", interworking.ErrIntegrityCheckFailed, err)
	}

	// The container of a mobility registration update carries a TRACKING AREA
	// UPDATE REQUEST (TS 24.501 §8.2.6.16); an ATTACH REQUEST belongs to the
	// initial-registration case, which this path does not serve.
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

// MMContextAck is the AMF's Context Acknowledge: the UE is served from 5GS from
// here on (TS 23.502 §4.11.1.3.3 step 8, TS 23.401 §5.3.3.1 step 7). transferred
// names the PDU sessions 5GS adopted; every PDN connection left behind is
// released, and the EMM context goes with them.
//
// Withholding the acknowledgement leaves this context untouched, so a move the
// AMF abandons costs the UE nothing on EPS.
func (m *MME) MMContextAck(ctx context.Context, supi etsi.SUPI, transferred []uint8) error {
	ue, ok := m.LookupUeBySupi(supi)
	if !ok {
		return fmt.Errorf("%w: %s", interworking.ErrUnknownUEContext, supi)
	}

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
