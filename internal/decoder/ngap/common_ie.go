// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"fmt"
	"net/netip"
	"strings"

	"github.com/ellanetworks/core/ngap"
)

// ie renders one modeled IE.
func ie(id ngap.ProtocolIEID, crit ngap.Criticality, value any) IE {
	return IE{
		ID:          ieEnum(id),
		Criticality: criticalityToEnum(crit),
		Value:       value,
	}
}

// bitsHex renders the low bits of v as a left-aligned bit string in hex, the
// form a BIT STRING takes on the wire.
func bitsHex(v uint64, bits int) string {
	b := make([]byte, (bits+7)/8)

	for i := range bits {
		if v&(1<<uint(bits-1-i)) != 0 {
			b[i/8] |= 1 << uint(7-i%8)
		}
	}

	return hex.EncodeToString(b)[:(bits+3)/4]
}

// rawIEValue carries an IE the library preserved but does not model. The hex
// sits under a key so the UI can tell an unmodeled IE from a modeled one whose
// value happens to be a hex string. internal/decoder/s1ap does the same.
type rawIEValue struct {
	Hex string `json:"hex"`
}

// unmodeledIEs renders the IEs the library preserved but does not model, so a
// capture shows what a peer sent instead of dropping it. TS 38.413 §10.3.4.2
// lets a receiver carry on past these, which is why they reach here at all.
func unmodeledIEs(raw []ngap.RawIE) []IE {
	if len(raw) == 0 {
		return nil
	}

	out := make([]IE, len(raw))
	for i, r := range raw {
		out[i] = ie(r.ID, r.Criticality, rawIEValue{Hex: hex.EncodeToString(r.Value)})
	}

	return out
}

// plmnIDToDecoder splits a PLMN identity into MCC/MNC digit strings.
func plmnIDToDecoder(id ngap.PLMNIdentity) PLMNID {
	h := strings.Split(hex.EncodeToString(id[:]), "")

	out := PLMNID{Mcc: h[1] + h[0] + h[3]}

	if h[2] == "f" {
		out.Mnc = h[5] + h[4]
	} else {
		out.Mnc = h[2] + h[5] + h[4]
	}

	return out
}

func criticalityDiagnostics(cd ngap.CriticalityDiagnostics) CriticalityDiagnostics {
	out := CriticalityDiagnostics{}

	if cd.ProcedureCode != nil {
		e := procedureCodeToEnum(*cd.ProcedureCode)
		out.ProcedureCode = &e
	}

	if cd.TriggeringMessage != nil {
		e := triggeringMessageToEnum(*cd.TriggeringMessage)
		out.TriggeringMessage = &e
	}

	if cd.ProcedureCriticality != nil {
		e := criticalityToEnum(*cd.ProcedureCriticality)
		out.ProcedureCriticality = &e
	}

	for _, item := range cd.IEsCriticalityDiagnostics {
		out.IEs = append(out.IEs, CriticalityDiagnosticsIE{
			IEID:        ieEnum(item.IEID),
			Criticality: criticalityToEnum(item.IECriticality),
			TypeOfError: typeOfErrorToEnum(item.TypeOfError),
		})
	}

	return out
}

// userLocationInformation renders the CHOICE the library flattens into one
// struct (TS 38.413 §9.3.2.2). Cell identities are right-aligned in the width
// the variant implies, and rendered left-aligned in hex.
func userLocationInformation(uli ngap.UserLocationInformation) UserLocationInformation {
	switch uli.Kind {
	case ngap.UserLocationEUTRA:
		return UserLocationInformation{EUTRA: &UserLocationInformationEUTRA{
			EUTRACGI: EUTRACGI{
				PLMNID:            plmnIDToDecoder(uli.PLMNIdentity),
				EUTRACellIdentity: bitsHex(uli.CellIdentity, ngap.EUTRACellIdentityBits),
			},
			TAI:       tai(uli.TAI),
			TimeStamp: timeStamp(uli.TimeStamp),
		}}
	case ngap.UserLocationNR:
		return UserLocationInformation{NR: &UserLocationInformationNR{
			NRCGI: NRCGI{
				PLMNID:         plmnIDToDecoder(uli.PLMNIdentity),
				NRCellIdentity: bitsHex(uli.CellIdentity, ngap.NRCellIdentityBits),
			},
			TAI:       tai(uli.TAI),
			TimeStamp: timeStamp(uli.TimeStamp),
		}}
	case ngap.UserLocationN3IWF:
		return UserLocationInformation{N3IWF: &UserLocationInformationN3IWF{
			IPAddress:  ipAddressText(uli.IPAddress),
			PortNumber: int32(uli.PortNumber),
		}}
	default:
		return UserLocationInformation{Error: fmt.Sprintf("unsupported UserLocationInformation type: %d", uli.Kind)}
	}
}

func tai(t ngap.TAI) TAI {
	return TAI{
		PLMNID: plmnIDToDecoder(t.PLMNIdentity),
		TAC:    fmt.Sprintf("%06x", uint32(t.TAC)),
	}
}

// timeStamp renders the NTP-epoch seconds an NG-RAN node stamps a location
// with. A malformed value is dropped rather than reported: the timestamp is
// decoration on a location that decoded fine.
func timeStamp(ts *ngap.TimeStamp) *string {
	if ts == nil {
		return nil
	}

	s, err := timeStampToRFC3339(ts[:])
	if err != nil {
		return nil
	}

	return &s
}

// ipAddressText renders a transport layer address as an IP string; the length
// distinguishes IPv4 from IPv6 (TS 38.413 §9.3.2.4).
func ipAddressText(addr ngap.TransportLayerAddress) string {
	if ip, ok := netip.AddrFromSlice(addr); ok {
		return ip.String()
	}

	return hex.EncodeToString(addr)
}
