// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"context"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
)

// PDNConnection is one PDU session described as the PDN connection it becomes in
// EPS (TS 23.502 §4.11.1.2.1 step 2).
//
// It names the session rather than restating it: the target resolves the APN's
// policy, addresses and QoS from the same subscriber data the source used, and
// takes over the anchor session by EPS bearer identity, so nothing that both
// sides can derive is carried and neither can drift from the other.
type PDNConnection struct {
	PDUSessionID      uint8
	EPSBearerIdentity uint8
	APN               string
	Snssai            *models.Snssai
}

// ENBIdentity names an eNB independently of the protocol that carried it: the
// serving PLMN, the eNB identity as a number, and how many bits of it the
// alternative it arrived in defines (18, 20, 21 or 28).
type ENBIdentity struct {
	PlmnID models.PlmnID
	ID     uint32
	Bits   uint8
	// EPSTAC is the tracking area the source RAN selected in the target system.
	EPSTAC uint16
}

// ForwardRelocationRequest is what the AMF sends the MME to prepare a handover
// to EPS (TS 23.502 §4.11.1.2.1 step 3, TS 33.501 §8.3.2 step 3).
type ForwardRelocationRequest struct {
	IMSI string

	SecurityContext EPSSecurityContext

	// PDNConnections are the sessions to transfer. A session the source could not
	// map is absent, and the source releases it separately.
	PDNConnections []PDNConnection

	// Target identifies the eNB the source RAN chose. It is stated in neutral
	// terms because the two systems spell an eNB identity differently: NGAP offers
	// no home-eNB alternative and S1AP does, so each side converts its own
	// spelling rather than reading the other's.
	Target ENBIdentity

	// SourceToTarget is the RAN container the source gNB produced in the target
	// system's format. It is relayed verbatim.
	SourceToTarget []byte

	UEAMBRUplink   models.BitRate
	UEAMBRDownlink models.BitRate
}

// ForwardRelocationResponse is the MME's answer once the target eNB has admitted
// the UE (TS 23.502 §4.11.1.2.1 step 9).
type ForwardRelocationResponse struct {
	// TargetToSource is the RAN container the target eNB produced, relayed to the
	// source gNB in the Handover Command.
	TargetToSource []byte

	// AcceptedPDUSessions are the sessions the target admitted; the source
	// releases the rest (TS 23.502 §4.11.1.2.1 step 12).
	AcceptedPDUSessions []uint8
}

// EPSTarget is the EPS half of N26, implemented by the MME. Ella runs both cores
// in one process, so the interface is the boundary rather than a protocol: it
// exists so the 5GS side names what it needs of EPS and no more.
type EPSTarget interface {
	ForwardRelocation(ctx context.Context, req ForwardRelocationRequest) (ForwardRelocationResponse, error)
	RelocationComplete(ctx context.Context, imsi string) error
	RelocationCancel(ctx context.Context, imsi string) error
}

// SUPIToIMSI renders a 5GS subscriber identity as the IMSI the EPS side keys on.
func SUPIToIMSI(supi etsi.SUPI) string { return supi.IMSI() }
