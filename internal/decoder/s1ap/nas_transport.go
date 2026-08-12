// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap"
)

func buildInitialUEMessage(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseInitialUEMessage(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Initial UE Message: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
		ie(s1ap.IDNASPDU, s1ap.CriticalityReject, nasPDU(m.NASPDU)),
		ie(s1ap.IDTAI, s1ap.CriticalityReject, tai(m.TAI)),
	}

	if m.EUTRANCGI != nil {
		ies = append(ies, ie(s1ap.IDEUTRANCGI, s1ap.CriticalityIgnore, eutranCGI(*m.EUTRANCGI)))
	}

	if m.RRCEstablishmentCause != nil {
		ies = append(ies, ie(s1ap.IDRRCEstablishmentCause, s1ap.CriticalityIgnore, rrcCauseToEnum(*m.RRCEstablishmentCause)))
	}

	if m.STMSI != nil {
		ies = append(ies, ie(s1ap.IDSTMSI, s1ap.CriticalityReject, stmsi(*m.STMSI)))
	}

	if m.GUMMEI != nil {
		ies = append(ies, ie(s1ap.IDGUMMEI, s1ap.CriticalityReject, gummei(*m.GUMMEI)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Initial UE Message (eNB-UE %d%s)", m.ENBUES1APID, nasSummary(m.NASPDU))
}

func buildUplinkNASTransport(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseUplinkNASTransport(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Uplink NAS Transport: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
		ie(s1ap.IDNASPDU, s1ap.CriticalityReject, nasPDU(m.NASPDU)),
	}

	if m.EUTRANCGI != nil {
		ies = append(ies, ie(s1ap.IDEUTRANCGI, s1ap.CriticalityIgnore, eutranCGI(*m.EUTRANCGI)))
	}

	if m.TAI != nil {
		ies = append(ies, ie(s1ap.IDTAI, s1ap.CriticalityIgnore, tai(*m.TAI)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Uplink NAS Transport (MME-UE %d, eNB-UE %d%s)", m.MMEUES1APID, m.ENBUES1APID, nasSummary(m.NASPDU))
}

func buildDownlinkNASTransport(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseDownlinkNASTransport(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Downlink NAS Transport: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
		ie(s1ap.IDNASPDU, s1ap.CriticalityReject, nasPDU(m.NASPDU)),
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Downlink NAS Transport (MME-UE %d, eNB-UE %d%s)", m.MMEUES1APID, m.ENBUES1APID, nasSummary(m.NASPDU))
}
