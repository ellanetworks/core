// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/s1ap"
)

// plmnOctets encodes an MCC/MNC pair as the 3-octet TBCD PLMN identity
// (TS 24.008 §10.5.1.3 / TS 23.003). E.g. 001/01 -> {0x00, 0xf1, 0x10}.
func plmnOctets(mcc, mnc string) (s1ap.PLMNIdentity, error) {
	octets, err := nas.PLMN{MCC: mcc, MNC: mnc}.Octets()
	if err != nil {
		return s1ap.PLMNIdentity{}, fmt.Errorf("s1enb: invalid MCC/MNC %q/%q: %w", mcc, mnc, err)
	}

	return s1ap.PLMNIdentity(octets), nil
}

func parseTAC(s string) (uint16, error) {
	if s == "" {
		return 0, fmt.Errorf("s1enb: empty TAC")
	}

	base := 10
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		s, base = s[2:], 16
	}

	v, err := strconv.ParseUint(s, base, 16)
	if err != nil {
		return 0, fmt.Errorf("s1enb: invalid TAC %q: %w", s, err)
	}

	return uint16(v), nil
}

// eutranCellID is this eNB's first cell. The library places the eNB ID and the
// cell index at the widths its kind implies, so neither is shifted here. A home
// eNB ID already fills all 28 bits of the cell identity, leaving cell 0 as its
// only cell.
func (e *ENB) eutranCellID() uint32 {
	cell := uint32(1)
	if e.enbIDKind == s1ap.ENBIDHome {
		cell = 0
	}

	id, err := e.enbNodeID().CellIdentity(cell)
	if err != nil {
		// enbID is simulator configuration, not peer input.
		panic(fmt.Sprintf("s1enb: cell identity for eNB %#x: %v", e.enbID, err))
	}

	return id
}

func (e *ENB) enbNodeID() s1ap.ENBID {
	return s1ap.ENBID{Kind: e.enbIDKind, Value: e.enbID}
}

func (e *ENB) tai() s1ap.TAI {
	return s1ap.TAI{PLMNIdentity: e.plmn, TAC: s1ap.TAC(e.tac)}
}

func (e *ENB) eutranCGI() s1ap.EUTRANCGI {
	return s1ap.EUTRANCGI{PLMNIdentity: e.plmn, CellID: e.eutranCellID()}
}

func (e *ENB) buildS1SetupRequest() ([]byte, error) {
	req := &s1ap.S1SetupRequest{
		GlobalENBID: s1ap.GlobalENBID{
			PLMNIdentity: e.plmn,
			ENBID:        e.enbNodeID(),
		},
		ENBName: new(e.name),
		SupportedTAs: s1ap.SupportedTAs{
			{TAC: s1ap.TAC(e.tac), BroadcastPLMNs: s1ap.BPLMNs{e.plmn}},
		},
		DefaultPagingDRX: s1ap.Ptr(s1ap.PagingDRXv32),
	}

	b, err := req.Marshal()
	if err != nil {
		return nil, fmt.Errorf("s1enb: build S1 Setup Request: %w", err)
	}

	return b, nil
}

// WaitForS1SetupFailure blocks until the MME answers the S1 Setup Request with
// an S1 SETUP FAILURE (TS 36.413) and returns the decoded message.
func (e *ENB) WaitForS1SetupFailure(timeout time.Duration) (*s1ap.S1SetupFailure, error) {
	frame, err := e.WaitForMessage(NoUEID, Unsuccessful, s1ap.ProcS1Setup, timeout)
	if err != nil {
		return nil, err
	}

	fail, err := s1ap.ParseS1SetupFailure(frame.Value)
	if err != nil {
		return nil, fmt.Errorf("s1enb: parse S1 Setup Failure: %w", err)
	}

	return fail, nil
}

// SendInitialUEMessage sends an INITIAL UE MESSAGE carrying the UE's first uplink
// NAS PDU (e.g. an Attach Request), as the eNB does on RRC connection setup.
func (e *ENB) SendInitialUEMessage(enbUEID int64, nas []byte) error {
	msg := &s1ap.InitialUEMessage{
		ENBUES1APID:           s1ap.ENBUES1APID(enbUEID),
		NASPDU:                s1ap.NASPDU(nas),
		TAI:                   e.tai(),
		EUTRANCGI:             s1ap.Ptr(e.eutranCGI()),
		RRCEstablishmentCause: s1ap.Ptr(s1ap.RRCCauseMOSignalling),
	}

	b, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("s1enb: build Initial UE Message: %w", err)
	}

	return e.SendMessage(b, true)
}

// SendInitialUEMessageWithSTMSI sends an INITIAL UE MESSAGE identified by the
// UE's S-TMSI, as a UE returning from ECM-IDLE does for a SERVICE REQUEST or a
// TRACKING AREA UPDATE (TS 24.301). nas is the (security-protected) NAS PDU.
func (e *ENB) SendInitialUEMessageWithSTMSI(enbUEID int64, mmec uint8, mtmsi uint32, nas []byte) error {
	msg := &s1ap.InitialUEMessage{
		ENBUES1APID:           s1ap.ENBUES1APID(enbUEID),
		NASPDU:                s1ap.NASPDU(nas),
		TAI:                   e.tai(),
		EUTRANCGI:             s1ap.Ptr(e.eutranCGI()),
		RRCEstablishmentCause: s1ap.Ptr(s1ap.RRCCauseMOSignalling),
		STMSI:                 &s1ap.STMSI{MMEC: s1ap.MMECode(mmec), MTMSI: s1ap.MTMSI(mtmsi)},
	}

	b, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("s1enb: build Initial UE Message with S-TMSI: %w", err)
	}

	return e.SendMessage(b, true)
}

// SendUplinkNASTransport sends a subsequent uplink NAS PDU for an established
// S1 UE-associated connection.
func (e *ENB) SendUplinkNASTransport(mmeUEID, enbUEID int64, nas []byte) error {
	msg := &s1ap.UplinkNASTransport{
		MMEUES1APID: s1ap.MMEUES1APID(mmeUEID),
		ENBUES1APID: s1ap.ENBUES1APID(enbUEID),
		NASPDU:      s1ap.NASPDU(nas),
		EUTRANCGI:   s1ap.Ptr(e.eutranCGI()),
		TAI:         s1ap.Ptr(e.tai()),
	}

	b, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("s1enb: build Uplink NAS Transport: %w", err)
	}

	return e.SendMessage(b, true)
}

// SendPathSwitchRequest sends a PATH SWITCH REQUEST as the target eNB after an X2
// handover (TS 36.413 §8.4.4), asking the MME to switch the E-RAB downlink to this
// eNB's S1-U endpoint. It returns the eNB downlink TEID it reported, so the caller
// can build the target GTP tunnel.
func (e *ENB) SendPathSwitchRequest(enbUEID, sourceMMEUEID int64, erabID s1ap.ERABID, caps s1ap.UESecurityCapabilities) (dlTEID uint32, err error) {
	addr := e.n3Addr.To4()
	if addr == nil {
		addr = e.n3Addr.To16()
	}

	dlTEID = e.allocTEID()

	req := &s1ap.PathSwitchRequest{
		ENBUES1APID: s1ap.ENBUES1APID(enbUEID),
		ERABToBeSwitchedDL: []s1ap.ERABToBeSwitchedDLItem{{
			ERABID:                erabID,
			TransportLayerAddress: s1ap.TransportLayerAddress(addr),
			GTPTEID:               s1ap.GTPTEID(dlTEID),
		}},
		SourceMMEUES1APID:      s1ap.MMEUES1APID(sourceMMEUEID),
		EUTRANCGI:              s1ap.Ptr(e.eutranCGI()),
		TAI:                    s1ap.Ptr(e.tai()),
		UESecurityCapabilities: s1ap.Ptr(caps),
	}

	b, err := req.Marshal()
	if err != nil {
		return 0, fmt.Errorf("s1enb: build Path Switch Request: %w", err)
	}

	if err := e.SendMessage(b, true); err != nil {
		return 0, err
	}

	return dlTEID, nil
}

// CauseUserInactivity is the S1AP radioNetwork cause an eNB uses to request a UE
// context release on inactivity (TS 36.413 §9.2.1.3, value 20).
var CauseUserInactivity = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 20}

// SendUEContextReleaseRequest asks the MME to release the UE's S1 context, as an
// eNB does on inactivity or radio-link failure (TS 36.413 §8.3.2).
func (e *ENB) SendUEContextReleaseRequest(mmeUEID, enbUEID int64, cause s1ap.Cause) error {
	req := &s1ap.UEContextReleaseRequest{
		MMEUES1APID: s1ap.MMEUES1APID(mmeUEID),
		ENBUES1APID: s1ap.ENBUES1APID(enbUEID),
		Cause:       s1ap.Ptr(cause),
	}

	b, err := req.Marshal()
	if err != nil {
		return fmt.Errorf("s1enb: build UE Context Release Request: %w", err)
	}

	return e.SendMessage(b, true)
}

func (e *ENB) SendUECapabilityInfoIndication(mmeUEID, enbUEID int64, capability []byte) error {
	if len(capability) == 0 {
		return fmt.Errorf("s1enb: UE Radio Capability is required to build UE Capability Info Indication")
	}

	ind := &s1ap.UECapabilityInfoIndication{
		MMEUES1APID:       s1ap.MMEUES1APID(mmeUEID),
		ENBUES1APID:       s1ap.ENBUES1APID(enbUEID),
		UERadioCapability: s1ap.UERadioCapability(capability),
	}

	b, err := ind.Marshal()
	if err != nil {
		return fmt.Errorf("s1enb: build UE Capability Info Indication: %w", err)
	}

	return e.SendMessage(b, true)
}

// WaitForUEContextReleaseCommand waits for the MME's UE CONTEXT RELEASE COMMAND
// targeting enbUEID.
func (e *ENB) WaitForUEContextReleaseCommand(enbUEID int64, timeout time.Duration) (*s1ap.UEContextReleaseCommand, error) {
	f, err := e.WaitForMessage(enbUEID, Initiating, s1ap.ProcUEContextRelease, timeout)
	if err != nil {
		return nil, err
	}

	cmd, err := s1ap.ParseUEContextReleaseCommand(f.Value)
	if err != nil {
		return nil, fmt.Errorf("s1enb: parse UE Context Release Command: %w", err)
	}

	return cmd, nil
}

// SendUEContextReleaseComplete acknowledges a UE CONTEXT RELEASE COMMAND, ending
// the S1 release procedure (TS 36.413 §8.3.3).
func (e *ENB) SendUEContextReleaseComplete(mmeUEID, enbUEID int64) error {
	resp := &s1ap.UEContextReleaseComplete{
		MMEUES1APID: s1ap.Ptr(s1ap.MMEUES1APID(mmeUEID)),
		ENBUES1APID: s1ap.Ptr(s1ap.ENBUES1APID(enbUEID)),
	}

	b, err := resp.Marshal()
	if err != nil {
		return fmt.Errorf("s1enb: build UE Context Release Complete: %w", err)
	}

	return e.SendMessage(b, true)
}

// SendReset sends an S1AP RESET (TS 36.413 §8.7.1). resetAll selects a
// whole-interface reset; otherwise items names the UE-associated logical
// S1-connections to reset. Reset is a non-UE-associated procedure, sent on
// SCTP stream 0.
func (e *ENB) SendReset(cause s1ap.Cause, resetAll bool, items []s1ap.UEAssociatedLogicalS1ConnectionItem) error {
	req := &s1ap.Reset{Cause: s1ap.Ptr(cause), ResetType: s1ap.ResetType{All: resetAll, Part: items}}

	b, err := req.Marshal()
	if err != nil {
		return fmt.Errorf("s1enb: build Reset: %w", err)
	}

	return e.SendMessage(b, false)
}

// WaitForResetAcknowledge waits for the MME's RESET ACKNOWLEDGE (TS 36.413
// §8.7.1).
func (e *ENB) WaitForResetAcknowledge(timeout time.Duration) (*s1ap.ResetAcknowledge, error) {
	f, err := e.WaitForMessage(NoUEID, Successful, s1ap.ProcReset, timeout)
	if err != nil {
		return nil, err
	}

	ack, err := s1ap.ParseResetAcknowledge(f.Value)
	if err != nil {
		return nil, fmt.Errorf("s1enb: parse Reset Acknowledge: %w", err)
	}

	return ack, nil
}

// WaitForDownlinkNAS waits for a DOWNLINK NAS TRANSPORT and returns its NAS PDU
// and the MME UE S1AP ID the MME assigned.
func (e *ENB) WaitForDownlinkNAS(enbUEID int64, timeout time.Duration) (nas []byte, mmeUEID int64, err error) {
	f, err := e.WaitForMessage(enbUEID, Initiating, s1ap.ProcDownlinkNASTransport, timeout)
	if err != nil {
		return nil, 0, err
	}

	dl, err := s1ap.ParseDownlinkNASTransport(f.Value)
	if err != nil {
		return nil, 0, fmt.Errorf("s1enb: parse Downlink NAS Transport: %w", err)
	}

	return []byte(dl.NASPDU), int64(dl.MMEUES1APID), nil
}

func messageName(cat Category, code s1ap.ProcedureCode) string {
	name := map[s1ap.ProcedureCode]string{
		s1ap.ProcS1Setup:              "S1Setup",
		s1ap.ProcInitialContextSetup:  "InitialContextSetup",
		s1ap.ProcDownlinkNASTransport: "DownlinkNASTransport",
		s1ap.ProcUEContextRelease:     "UEContextRelease",
		s1ap.ProcPathSwitchRequest:    "PathSwitchRequest",
		s1ap.ProcPaging:               "Paging",
		s1ap.ProcErrorIndication:      "ErrorIndication",
		s1ap.ProcReset:                "Reset",
		s1ap.ProcERABSetup:            "E-RABSetup",
		s1ap.ProcERABRelease:          "E-RABRelease",

		s1ap.ProcDownlinkUEAssociatedLPPaTransport: "DownlinkUEAssociatedLPPaTransport",
		s1ap.ProcUplinkUEAssociatedLPPaTransport:   "UplinkUEAssociatedLPPaTransport",
	}[code]
	if name == "" {
		name = fmt.Sprintf("procedure-%d", code)
	}

	cats := [...]string{"InitiatingMessage", "SuccessfulOutcome", "UnsuccessfulOutcome"}

	c := "Unknown"
	if int(cat) < len(cats) {
		c = cats[cat]
	}

	return c + "/" + name
}
