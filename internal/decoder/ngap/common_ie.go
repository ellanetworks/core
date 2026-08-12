// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/ngap"
)

// ProtocolIE-ID values the renderers cite (TS 38.413, NGAP-Constants).
const (
	idAMFName                                    ngap.ProtocolIEID = 1
	idAMFUENGAPID                                ngap.ProtocolIEID = 10
	idCause                                      ngap.ProtocolIEID = 15
	idCriticalityDiagnostics                     ngap.ProtocolIEID = 19
	idDefaultPagingDRX                           ngap.ProtocolIEID = 21
	idFiveGSTMSI                                 ngap.ProtocolIEID = 26
	idGlobalRANNodeID                            ngap.ProtocolIEID = 27
	idMobilityRestrictionList                    ngap.ProtocolIEID = 36
	idNRPPaPDU                                   ngap.ProtocolIEID = 46
	idRoutingID                                  ngap.ProtocolIEID = 89
	idUENGAPIDs                                  ngap.ProtocolIEID = 114
	idUEPagingIdentity                           ngap.ProtocolIEID = 115
	idTAIListForPaging                           ngap.ProtocolIEID = 103
	idUnavailableGUAMIList                       ngap.ProtocolIEID = 120
	idUEPresenceInAreaOfInterestList             ngap.ProtocolIEID = 116
	idLocationReportingRequestType               ngap.ProtocolIEID = 33
	idUERadioCapability                          ngap.ProtocolIEID = 117
	idNASPDU                                     ngap.ProtocolIEID = 38
	idPDUSessionResourceSetupListSURes           ngap.ProtocolIEID = 75
	idPDUSessionResourceFailedToSetupListSURes   ngap.ProtocolIEID = 58
	idPDUSessionResourceSetupListCxtRes          ngap.ProtocolIEID = 72
	idPDUSessionResourceFailedToSetupListCxtRes  ngap.ProtocolIEID = 55
	idPDUSessionResourceFailedToSetupListCxtFail ngap.ProtocolIEID = 132
	idPDUSessionResourceToReleaseListRelCmd      ngap.ProtocolIEID = 79
	idPDUSessionResourceReleasedListRelRes       ngap.ProtocolIEID = 70
	idPDUSessionResourceSetupListSUReq           ngap.ProtocolIEID = 74
	idUEAggregateMaximumBitRate                  ngap.ProtocolIEID = 110
	idRRCEstablishmentCause                      ngap.ProtocolIEID = 90
	idAMFSetID                                   ngap.ProtocolIEID = 3
	idUEContextRequest                           ngap.ProtocolIEID = 112
	idAllowedNSSAI                               ngap.ProtocolIEID = 0
	idGUAMI                                      ngap.ProtocolIEID = 28
	idPDUSessionResourceSetupListCxtReq          ngap.ProtocolIEID = 71
	idUESecurityCapabilities                     ngap.ProtocolIEID = 119
	idSecurityKey                                ngap.ProtocolIEID = 94
	idUserLocationInformation                    ngap.ProtocolIEID = 121
	idPDUSessionResourceListCxtRelCpl            ngap.ProtocolIEID = 60
	idPDUSessionResourceListCxtRelReq            ngap.ProtocolIEID = 133
	idPLMNSupportList                            ngap.ProtocolIEID = 80
	idRANNodeName                                ngap.ProtocolIEID = 82
	idRANUENGAPID                                ngap.ProtocolIEID = 85
	idRelativeAMFCapacity                        ngap.ProtocolIEID = 86
	idServedGUAMIList                            ngap.ProtocolIEID = 96
	idSupportedTAList                            ngap.ProtocolIEID = 102
	idTimeToWait                                 ngap.ProtocolIEID = 107
	idUERetentionInformation                     ngap.ProtocolIEID = 147
)

// ie renders one modeled IE.
func ie(id ngap.ProtocolIEID, crit ngap.Criticality, value any) IE {
	return IE{
		ID:          ieEnum(int64(id)),
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

// rawIEValue carries an IE the library preserved but does not model. It is a
// named struct rather than a bare hex string so inferValueType reports it as
// such — setIEValueTypes overwrites whatever ValueType a renderer sets, which
// is why the marker cannot live there. internal/decoder/s1ap does the same.
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
	p, err := nas.ParsePLMN([3]byte(id))
	if err != nil {
		return PLMNID{}
	}

	return PLMNID{Mcc: p.MCC, Mnc: p.MNC}
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
			IEID:        ieEnum(int64(item.IEID)),
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
