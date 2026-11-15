// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

const unknownExtensionID ProtocolIEID = 65000

func upTNL(host byte, teid uint32) UPTransportLayerInformation {
	return UPTransportLayerInformation{GTPTunnel: &GTPTunnel{
		TransportLayerAddress: tlaOf(10, 0, 0, host),
		GTPTEID:               GTPTEID(teid),
	}}
}

func upTNLPtr(host byte, teid uint32) *UPTransportLayerInformation {
	u := upTNL(host, teid)
	return &u
}

func qosFlowTNL(host byte, teid uint32) QosFlowPerTNLInformation {
	return QosFlowPerTNLInformation{
		UPTransportLayerInformation: upTNL(host, teid),
		AssociatedQosFlowList:       AssociatedQosFlowList{{QosFlowIdentifier: 1}},
	}
}

func qosFlowTNLList(host byte, teid uint32) QosFlowPerTNLInformationList {
	return QosFlowPerTNLInformationList{{QosFlowPerTNLInformation: qosFlowTNL(host, teid)}}
}

func pairList(ulHost byte, ulTEID uint32, dlHost byte, dlTEID uint32) UPTransportLayerInformationPairList {
	return UPTransportLayerInformationPairList{{
		ULNGUUPTNLInformation: upTNL(ulHost, ulTEID),
		DLNGUUPTNLInformation: upTNL(dlHost, dlTEID),
	}}
}

func tnlList(host byte, teid uint32) UPTransportLayerInformationList {
	return UPTransportLayerInformationList{{NGUUPTNLInformation: upTNL(host, teid)}}
}

func tnlSetupRequestTransfer() *PDUSessionResourceSetupRequestTransfer {
	return &PDUSessionResourceSetupRequestTransfer{
		ULNGUUPTNLInformation:           upTNL(1, 0x101),
		AdditionalULNGUUPTNLInformation: tnlList(1, 0x102),
		PDUSessionType:                  PDUSessionTypeIPv4,
		QosFlowSetupRequest: QosFlowSetupRequestList{{
			QosFlowIdentifier: 1,
			QosFlowLevelQosParameters: QosFlowLevelQosParameters{
				QosCharacteristics: QosCharacteristics{NonDynamic5QI: &NonDynamic5QIDescriptor{FiveQI: 9}},
				AllocationAndRetentionPriority: AllocationAndRetentionPriority{
					PriorityLevelARP:        8,
					PreemptionCapability:    PreemptionShallNotTrigger,
					PreemptionVulnerability: PreemptionNotPreemptable,
				},
			},
		}},
		RedundantULNGUUPTNLInformation:           upTNLPtr(2, 0x103),
		AdditionalRedundantULNGUUPTNLInformation: tnlList(2, 0x104),
	}
}

func tnlSetupResponseTransfer() *PDUSessionResourceSetupResponseTransfer {
	r := qosFlowTNL(4, 0x203)

	return &PDUSessionResourceSetupResponseTransfer{
		DLQosFlowPerTNLInformation:                    qosFlowTNL(3, 0x201),
		AdditionalDLQosFlowPerTNLInformation:          qosFlowTNLList(3, 0x202),
		RedundantDLQosFlowPerTNLInformation:           &r,
		AdditionalRedundantDLQosFlowPerTNLInformation: qosFlowTNLList(4, 0x204),
	}
}

func tnlModifyRequestTransfer() *PDUSessionResourceModifyRequestTransfer {
	return &PDUSessionResourceModifyRequestTransfer{
		ULNGUUPTNLModify: ULNGUUPTNLModifyList{{
			ULNGUUPTNLInformation:          upTNL(1, 0x301),
			DLNGUUPTNLInformation:          upTNL(3, 0x302),
			RedundantULNGUUPTNLInformation: upTNLPtr(2, 0x303),
			RedundantDLNGUUPTNLInformation: upTNLPtr(4, 0x304),
		}},
		QosFlowAddOrModifyRequest: QosFlowAddOrModifyRequestList{{
			QosFlowIdentifier:     1,
			ULNGUUPTNLInformation: upTNLPtr(1, 0x305),
		}},
		AdditionalULNGUUPTNLInformation:          tnlList(1, 0x306),
		AdditionalRedundantULNGUUPTNLInformation: tnlList(2, 0x307),
		RedundantULNGUUPTNLInformation:           upTNLPtr(2, 0x308),
		UserPlaneFailureIndication: &UserPlaneFailureIndication{
			UserPlaneFailureType:  UserPlaneFailureTypeUPPathFailure,
			ULNGUUPTNLInformation: upTNL(1, 0x309),
			DLNGUUPTNLInformation: upTNL(3, 0x30a),
		},
	}
}

func tnlModifyResponseTransfer() *PDUSessionResourceModifyResponseTransfer {
	return &PDUSessionResourceModifyResponseTransfer{
		DLNGUUPTNLInformation: upTNLPtr(3, 0x401),
		ULNGUUPTNLInformation: upTNLPtr(1, 0x402),
		QosFlowAddOrModifyResponse: QosFlowAddOrModifyResponseList{{
			QosFlowIdentifier:     1,
			DLNGUUPTNLInformation: upTNLPtr(3, 0x403),
		}},
		AdditionalDLQosFlowPerTNLInformation:          qosFlowTNLList(3, 0x404),
		AdditionalNGUUPTNLInformation:                 pairList(1, 0x405, 3, 0x406),
		RedundantDLNGUUPTNLInformation:                upTNLPtr(4, 0x407),
		RedundantULNGUUPTNLInformation:                upTNLPtr(2, 0x408),
		AdditionalRedundantDLQosFlowPerTNLInformation: qosFlowTNLList(4, 0x409),
		AdditionalRedundantNGUUPTNLInformation:        pairList(2, 0x40a, 4, 0x40b),
	}
}

func tnlModifyIndicationTransfer() *PDUSessionResourceModifyIndicationTransfer {
	r := qosFlowTNL(4, 0x503)

	return &PDUSessionResourceModifyIndicationTransfer{
		DLQosFlowPerTNLInformation:                    qosFlowTNL(3, 0x501),
		AdditionalDLQosFlowPerTNLInformation:          qosFlowTNLList(3, 0x502),
		RedundantDLQosFlowPerTNLInformation:           &r,
		AdditionalRedundantDLQosFlowPerTNLInformation: qosFlowTNLList(4, 0x504),
	}
}

func tnlModifyConfirmTransfer() *PDUSessionResourceModifyConfirmTransfer {
	return &PDUSessionResourceModifyConfirmTransfer{
		QosFlowModifyConfirm:                   QosFlowModifyConfirmList{{QosFlowIdentifier: 1}},
		ULNGUUPTNLInformation:                  upTNL(1, 0x601),
		AdditionalNGUUPTNLInformation:          pairList(1, 0x602, 3, 0x603),
		RedundantULNGUUPTNLInformation:         upTNLPtr(2, 0x604),
		AdditionalRedundantNGUUPTNLInformation: pairList(2, 0x605, 4, 0x606),
	}
}

func tnlPathSwitchRequestTransfer() *PathSwitchRequestTransfer {
	return &PathSwitchRequestTransfer{
		DLNGUUPTNLInformation:                         upTNL(3, 0x701),
		QosFlowAccepted:                               QosFlowAcceptedList{{QosFlowIdentifier: 1}},
		AdditionalDLQosFlowPerTNLInformation:          qosFlowTNLList(3, 0x702),
		RedundantDLNGUUPTNLInformation:                upTNLPtr(4, 0x703),
		RedundantDLNGUTNLInformationReused:            Ptr(DLNGUTNLInformationReusedTrue),
		AdditionalRedundantDLQosFlowPerTNLInformation: qosFlowTNLList(4, 0x704),
	}
}

func tnlPathSwitchRequestAcknowledgeTransfer() *PathSwitchRequestAcknowledgeTransfer {
	return &PathSwitchRequestAcknowledgeTransfer{
		ULNGUUPTNLInformation:                  upTNLPtr(1, 0x801),
		AdditionalNGUUPTNLInformation:          pairList(1, 0x802, 3, 0x803),
		RedundantULNGUUPTNLInformation:         upTNLPtr(2, 0x804),
		AdditionalRedundantNGUUPTNLInformation: pairList(2, 0x805, 4, 0x806),
	}
}

func tnlHandoverRequestAcknowledgeTransfer() *HandoverRequestAcknowledgeTransfer {
	return &HandoverRequestAcknowledgeTransfer{
		DLNGUUPTNLInformation:        upTNL(3, 0x901),
		DLForwardingUPTNLInformation: upTNLPtr(3, 0x902),
		QosFlowSetupResponse:         QosFlowListWithDataForwarding{{QosFlowIdentifier: 1}},
		DataForwardingResponseDRB: DataForwardingResponseDRBList{{
			DRBID:                        1,
			DLForwardingUPTNLInformation: upTNLPtr(3, 0x903),
			ULForwardingUPTNLInformation: upTNLPtr(3, 0x904),
		}},
		AdditionalDLUPTNLInformationForHO: AdditionalDLUPTNLInformationForHOList{{
			AdditionalDLNGUUPTNLInformation:          upTNL(3, 0x905),
			AdditionalQosFlowSetupResponse:           QosFlowListWithDataForwarding{{QosFlowIdentifier: 1}},
			AdditionalDLForwardingUPTNLInformation:   upTNLPtr(3, 0x906),
			AdditionalRedundantDLNGUUPTNLInformation: upTNLPtr(4, 0x907),
		}},
		ULForwardingUPTNLInformation:           upTNLPtr(3, 0x908),
		AdditionalULForwardingUPTNLInformation: tnlList(3, 0x909),
		DataForwardingResponseERAB: DataForwardingResponseERABList{{
			ERABID:                       5,
			DLForwardingUPTNLInformation: upTNL(3, 0x90a),
		}},
		RedundantDLNGUUPTNLInformation: upTNLPtr(4, 0x90b),
	}
}

func tnlHandoverCommandTransfer() *HandoverCommandTransfer {
	return &HandoverCommandTransfer{
		DLForwardingUPTNLInformation: upTNLPtr(1, 0xa01),
		QosFlowToBeForwarded:         QosFlowToBeForwardedList{{QosFlowIdentifier: 1}},
		DataForwardingResponseDRB: DataForwardingResponseDRBList{{
			DRBID:                        1,
			DLForwardingUPTNLInformation: upTNLPtr(1, 0xa02),
			ULForwardingUPTNLInformation: upTNLPtr(1, 0xa03),
		}},
		AdditionalDLForwardingUPTNLInformation: qosFlowTNLList(1, 0xa04),
		ULForwardingUPTNLInformation:           upTNLPtr(1, 0xa05),
		AdditionalULForwardingUPTNLInformation: tnlList(1, 0xa06),
		DataForwardingResponseERAB: DataForwardingResponseERABList{{
			ERABID:                       5,
			DLForwardingUPTNLInformation: upTNL(1, 0xa07),
		}},
	}
}

type tnlTransferCase struct {
	name     string
	transfer interface {
		Marshal() (TransferContainer, error)
	}
	parse   func(TransferContainer) (any, error)
	message func(TransferContainer) interface{ Marshal() ([]byte, error) }
	teids   []uint32
	golden  string

	notTshark    []uint32
	notTsharkWhy string
}

func tnlTransferCases() []tnlTransferCase {
	return []tnlTransferCase{
		{
			name:     "PDUSessionResourceSetupRequestTransfer",
			transfer: tnlSetupRequestTransfer(),
			parse: func(b TransferContainer) (any, error) {
				return ParsePDUSessionResourceSetupRequestTransfer(b)
			},
			message: func(b TransferContainer) interface{ Marshal() ([]byte, error) } {
				return &PDUSessionResourceSetupRequest{
					AMFUENGAPID:             1,
					RANUENGAPID:             2,
					PDUSessionResourceSetup: PDUSessionResourceSetupListSUReq{{PDUSessionID: 5, SNSSAI: SNSSAI{SST: 1}, Transfer: b}},
				}
			},
			teids:  []uint32{0x101, 0x102, 0x103, 0x104},
			golden: goldenTNLSetupRequest,
		},
		{
			name:     "PDUSessionResourceSetupResponseTransfer",
			transfer: tnlSetupResponseTransfer(),
			parse: func(b TransferContainer) (any, error) {
				return ParsePDUSessionResourceSetupResponseTransfer(b)
			},
			message: func(b TransferContainer) interface{ Marshal() ([]byte, error) } {
				return &PDUSessionResourceSetupResponse{
					AMFUENGAPID:             Ptr(AMFUENGAPID(1)),
					RANUENGAPID:             Ptr(RANUENGAPID(2)),
					PDUSessionResourceSetup: PDUSessionResourceSetupListSURes{{PDUSessionID: 5, Transfer: b}},
				}
			},
			teids:  []uint32{0x201, 0x202, 0x203, 0x204},
			golden: goldenTNLSetupResponse,
		},
		{
			name:     "PDUSessionResourceModifyRequestTransfer",
			transfer: tnlModifyRequestTransfer(),
			parse: func(b TransferContainer) (any, error) {
				return ParsePDUSessionResourceModifyRequestTransfer(b)
			},
			message: func(b TransferContainer) interface{ Marshal() ([]byte, error) } {
				return &PDUSessionResourceModifyRequest{
					AMFUENGAPID:              1,
					RANUENGAPID:              2,
					PDUSessionResourceModify: PDUSessionResourceModifyListModReq{{PDUSessionID: 5, Transfer: b, SNSSAI: &SNSSAI{SST: 1, SD: &SD{0x01, 0x02, 0x03}}}},
				}
			},
			teids:        []uint32{0x301, 0x302, 0x303, 0x304, 0x305, 0x306, 0x307, 0x308, 0x309, 0x30a},
			golden:       goldenTNLModifyRequest,
			notTshark:    []uint32{0x305},
			notTsharkWhy: "Wireshark 4.6 has no QosFlowAddOrModifyRequestItem-ExtIEs entry for id-UL-NGU-UP-TNLInformation and shows its value as undissected data",
		},
		{
			name:     "PDUSessionResourceModifyResponseTransfer",
			transfer: tnlModifyResponseTransfer(),
			parse: func(b TransferContainer) (any, error) {
				return ParsePDUSessionResourceModifyResponseTransfer(b)
			},
			message: func(b TransferContainer) interface{ Marshal() ([]byte, error) } {
				return &PDUSessionResourceModifyResponse{
					AMFUENGAPID:              Ptr(AMFUENGAPID(1)),
					RANUENGAPID:              Ptr(RANUENGAPID(2)),
					PDUSessionResourceModify: PDUSessionResourceModifyListModRes{{PDUSessionID: 5, Transfer: b}},
				}
			},
			teids:        []uint32{0x401, 0x402, 0x403, 0x404, 0x405, 0x406, 0x407, 0x408, 0x409, 0x40a, 0x40b},
			golden:       goldenTNLModifyResponse,
			notTshark:    []uint32{0x403},
			notTsharkWhy: "Wireshark 4.6 has no QosFlowAddOrModifyResponseItem-ExtIEs entry for id-DL-NGU-UP-TNLInformation and shows its value as undissected data",
		},
		{
			name:     "PDUSessionResourceModifyIndicationTransfer",
			transfer: tnlModifyIndicationTransfer(),
			parse: func(b TransferContainer) (any, error) {
				return ParsePDUSessionResourceModifyIndicationTransfer(b)
			},
			message: func(b TransferContainer) interface{ Marshal() ([]byte, error) } {
				return &PDUSessionResourceModifyIndication{
					AMFUENGAPID:              1,
					RANUENGAPID:              2,
					PDUSessionResourceModify: PDUSessionResourceModifyListModInd{{PDUSessionID: 5, Transfer: b}},
				}
			},
			teids:  []uint32{0x501, 0x502, 0x503, 0x504},
			golden: goldenTNLModifyIndication,
		},
		{
			name:     "PDUSessionResourceModifyConfirmTransfer",
			transfer: tnlModifyConfirmTransfer(),
			parse: func(b TransferContainer) (any, error) {
				return ParsePDUSessionResourceModifyConfirmTransfer(b)
			},
			message: func(b TransferContainer) interface{ Marshal() ([]byte, error) } {
				return &PDUSessionResourceModifyConfirm{
					AMFUENGAPID:              Ptr(AMFUENGAPID(1)),
					RANUENGAPID:              Ptr(RANUENGAPID(2)),
					PDUSessionResourceModify: PDUSessionResourceModifyListModCfm{{PDUSessionID: 5, Transfer: b}},
				}
			},
			teids:  []uint32{0x601, 0x602, 0x603, 0x604, 0x605, 0x606},
			golden: goldenTNLModifyConfirm,
		},
		{
			name:     "PathSwitchRequestTransfer",
			transfer: tnlPathSwitchRequestTransfer(),
			parse: func(b TransferContainer) (any, error) {
				return ParsePathSwitchRequestTransfer(b)
			},
			message: func(b TransferContainer) interface{ Marshal() ([]byte, error) } {
				return &PathSwitchRequest{
					RANUENGAPID:                          2,
					SourceAMFUENGAPID:                    1,
					UserLocationInformation:              &UserLocationInformation{NR: &UserLocationInformationNR{NRCGI: NRCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, NRCellIdentity: NRCellIdentity(0)}, TAI: TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1}}},
					UESecurityCapabilities:               &UESecurityCapabilities{},
					PDUSessionResourceToBeSwitchedDLList: PDUSessionResourceToBeSwitchedDLList{{PDUSessionID: 5, Transfer: b}},
				}
			},
			teids:  []uint32{0x701, 0x702, 0x703, 0x704},
			golden: goldenTNLPathSwitchRequest,
		},
		{
			name:     "PathSwitchRequestAcknowledgeTransfer",
			transfer: tnlPathSwitchRequestAcknowledgeTransfer(),
			parse: func(b TransferContainer) (any, error) {
				return ParsePathSwitchRequestAcknowledgeTransfer(b)
			},
			message: func(b TransferContainer) interface{ Marshal() ([]byte, error) } {
				return &PathSwitchRequestAcknowledge{
					AMFUENGAPID:                    Ptr(AMFUENGAPID(1)),
					RANUENGAPID:                    Ptr(RANUENGAPID(2)),
					PDUSessionResourceSwitchedList: PDUSessionResourceSwitchedList{{PDUSessionID: 5, Transfer: b}},
					AllowedNSSAI:                   AllowedNSSAI{{SNSSAI: SNSSAI{SST: 1}}},
				}
			},
			teids:  []uint32{0x801, 0x802, 0x803, 0x804, 0x805, 0x806},
			golden: goldenTNLPathSwitchRequestAcknowledge,
		},
		{
			name:     "HandoverRequestAcknowledgeTransfer",
			transfer: tnlHandoverRequestAcknowledgeTransfer(),
			parse: func(b TransferContainer) (any, error) {
				return ParseHandoverRequestAcknowledgeTransfer(b)
			},
			message: func(b TransferContainer) interface{ Marshal() ([]byte, error) } {
				return &HandoverRequestAcknowledge{
					AMFUENGAPID:                        Ptr(AMFUENGAPID(1)),
					RANUENGAPID:                        Ptr(RANUENGAPID(2)),
					PDUSessionResourceAdmittedList:     PDUSessionResourceAdmittedList{{PDUSessionID: 5, Transfer: b}},
					TargetToSourceTransparentContainer: TargetToSourceTransparentContainer{0x00, 0x00},
				}
			},
			teids:  []uint32{0x901, 0x902, 0x903, 0x904, 0x905, 0x906, 0x907, 0x908, 0x909, 0x90a, 0x90b},
			golden: goldenTNLHandoverRequestAcknowledge,
		},
		{
			name:     "HandoverCommandTransfer",
			transfer: tnlHandoverCommandTransfer(),
			parse: func(b TransferContainer) (any, error) {
				return ParseHandoverCommandTransfer(b)
			},
			message: func(b TransferContainer) interface{ Marshal() ([]byte, error) } {
				return &HandoverCommand{
					AMFUENGAPID:                        1,
					RANUENGAPID:                        2,
					HandoverType:                       HandoverTypeIntra5GS,
					PDUSessionResourceHandoverList:     PDUSessionResourceHandoverList{{PDUSessionID: 5, Transfer: b}},
					TargetToSourceTransparentContainer: TargetToSourceTransparentContainer{0x00, 0x00},
				}
			},
			teids:  []uint32{0xa01, 0xa02, 0xa03, 0xa04, 0xa05, 0xa06, 0xa07},
			golden: goldenTNLHandoverCommand,
		},
	}
}

func tnlCaseMessage(t *testing.T, c tnlTransferCase) []byte {
	t.Helper()

	b, err := c.transfer.Marshal()
	if err != nil {
		t.Fatalf("%s: %v", c.name, err)
	}

	m, err := c.message(b).Marshal()
	if err != nil {
		t.Fatalf("%s message: %v", c.name, err)
	}

	return m
}

func TestTNLExtensionGoldenRoundTrip(t *testing.T) {
	for _, c := range tnlTransferCases() {
		t.Run(c.name, func(t *testing.T) {
			if got := hex.EncodeToString(tnlCaseMessage(t, c)); got != c.golden {
				t.Fatalf("encoded\n got %s\nwant %s", got, c.golden)
			}

			b, err := c.transfer.Marshal()
			if err != nil {
				t.Fatal(err)
			}

			out, err := c.parse(b)
			if err != nil {
				t.Fatal(err)
			}

			if !equalByMethod(out, c.transfer) {
				t.Fatalf("decoded\n got %+v\nwant %+v", out, c.transfer)
			}

			again, err := out.(interface {
				Marshal() (TransferContainer, error)
			}).Marshal()
			if err != nil || !bytes.Equal(again, b) {
				t.Fatalf("re-encoded %x (%v), want %x", again, err, b)
			}
		})
	}
}

func TestTNLExtensionGoldenTshark(t *testing.T) {
	tshark, err1 := exec.LookPath("tshark")
	text2pcap, err2 := exec.LookPath("text2pcap")

	if err1 != nil || err2 != nil {
		t.Skip("tshark/text2pcap not installed")
	}

	cases := tnlTransferCases()

	var dump strings.Builder
	for _, c := range cases {
		hexDump(&dump, mustHex(t, c.golden))
	}

	rows := tsharkFields(t, tshark, text2pcap, dump.String(), "ngap.gTP_TEID", "_ws.malformed", "_ws.expert.message")
	if len(rows) != len(cases) {
		t.Fatalf("tshark decoded %d frames, want %d", len(rows), len(cases))
	}

	for i, c := range cases {
		var want []string

		for _, teid := range c.teids {
			if !slices.Contains(c.notTshark, teid) {
				want = append(want, fmt.Sprintf("%08x", teid))
			}
		}

		got := strings.ReplaceAll(rows[i][0], ":", "")
		if got != strings.Join(want, ",") {
			t.Errorf("%s: tshark GTP-TEIDs %s, want %s", c.name, got, strings.Join(want, ","))
		}

		if rows[i][1] != "" || rows[i][2] != "" {
			t.Errorf("%s: tshark reports %q", c.name, strings.Join(rows[i][1:], "|"))
		}
	}
}

func TestTNLExtensionCriticality(t *testing.T) {
	type row struct {
		id   ProtocolIEID
		crit Criticality
	}

	critOf := func(ids []ProtocolIEID, crits []Criticality) []row {
		out := make([]row, len(ids))
		for i := range ids {
			out[i] = row{ids[i], crits[i]}
		}

		return out
	}

	crits := func(table any) []Criticality {
		v := reflect.ValueOf(table)
		out := make([]Criticality, v.Len())

		for i := range out {
			out[i] = Criticality(v.Index(i).FieldByName("Criticality").Uint())
		}

		return out
	}

	ids := func(table any) []ProtocolIEID {
		v := reflect.ValueOf(table)
		out := make([]ProtocolIEID, v.Len())

		for i := range out {
			out[i] = ProtocolIEID(v.Index(i).FieldByName("ID").Uint())
		}

		return out
	}

	ig, rj := CriticalityIgnore, CriticalityReject

	for _, tc := range []struct {
		name  string
		table any
		want  []row
	}{
		{"PDUSessionResourceSetupRequestTransferIEs", pDUSessionResourceSetupRequestTransferIEs, []row{
			{IDPDUSessionAggregateMaximumBitRate, rj},
			{IDULNGUUPTNLInformation, rj},
			{IDAdditionalULNGUUPTNLInformation, rj},
			{IDDataForwardingNotPossible, rj},
			{IDPDUSessionType, rj},
			{IDSecurityIndication, rj},
			{IDNetworkInstance, rj},
			{IDQosFlowSetupRequestList, rj},
			{IDDirectForwardingPathAvailability, ig},
			{IDRedundantULNGUUPTNLInformation, ig},
			{IDAdditionalRedundantULNGUUPTNLInformation, ig},
		}},
		{"PDUSessionResourceModifyRequestTransferIEs", pDUSessionResourceModifyRequestTransferIEs, []row{
			{IDPDUSessionAggregateMaximumBitRate, rj},
			{IDULNGUUPTNLModifyList, rj},
			{IDNetworkInstance, rj},
			{IDQosFlowAddOrModifyRequestList, rj},
			{IDQosFlowToReleaseList, rj},
			{IDAdditionalULNGUUPTNLInformation, rj},
			{IDAdditionalRedundantULNGUUPTNLInformation, ig},
			{IDRedundantULNGUUPTNLInformation, ig},
			{IDUserPlaneFailureIndication, ig},
		}},
		{"PDUSessionResourceSetupResponseTransfer-ExtIEs", extIEsPDUSessionResourceSetupResponseTransfer, []row{
			{IDRedundantDLQosFlowPerTNLInformation, ig}, {IDAdditionalRedundantDLQosFlowPerTNLInformation, ig},
		}},
		{"PDUSessionResourceModifyResponseTransfer-ExtIEs", extIEsPDUSessionResourceModifyResponseTransfer, []row{
			{IDAdditionalNGUUPTNLInformation, ig},
			{IDRedundantDLNGUUPTNLInformation, ig},
			{IDRedundantULNGUUPTNLInformation, ig},
			{IDAdditionalRedundantDLQosFlowPerTNLInformation, ig},
			{IDAdditionalRedundantNGUUPTNLInformation, ig},
		}},
		{"PDUSessionResourceModifyIndicationTransfer-ExtIEs", extIEsPDUSessionResourceModifyIndicationTransfer, []row{
			{IDRedundantDLQosFlowPerTNLInformation, ig}, {IDAdditionalRedundantDLQosFlowPerTNLInformation, ig},
		}},
		{"PDUSessionResourceModifyConfirmTransfer-ExtIEs", extIEsPDUSessionResourceModifyConfirmTransfer, []row{
			{IDRedundantULNGUUPTNLInformation, ig}, {IDAdditionalRedundantNGUUPTNLInformation, ig},
		}},
		{"PathSwitchRequestTransfer-ExtIEs", extIEsPathSwitchRequestTransfer, []row{
			{IDAdditionalDLQosFlowPerTNLInformation, ig},
			{IDRedundantDLNGUUPTNLInformation, ig},
			{IDRedundantDLNGUTNLInformationReused, ig},
			{IDAdditionalRedundantDLQosFlowPerTNLInformation, ig},
		}},
		{"PathSwitchRequestAcknowledgeTransfer-ExtIEs", extIEsPathSwitchRequestAcknowledgeTransfer, []row{
			{IDAdditionalNGUUPTNLInformation, ig}, {IDRedundantULNGUUPTNLInformation, ig}, {IDAdditionalRedundantNGUUPTNLInformation, ig},
		}},
		{"HandoverRequestAcknowledgeTransfer-ExtIEs", extIEsHandoverRequestAcknowledgeTransfer, []row{
			{IDAdditionalDLUPTNLInformationForHOList, ig},
			{IDULForwardingUPTNLInformation, rj},
			{IDAdditionalULForwardingUPTNLInformation, rj},
			{IDDataForwardingResponseERABList, ig},
			{IDRedundantDLNGUUPTNLInformation, ig},
		}},
		{"HandoverCommandTransfer-ExtIEs", extIEsHandoverCommandTransfer, []row{
			{IDAdditionalDLForwardingUPTNLInformation, ig},
			{IDULForwardingUPTNLInformation, rj},
			{IDAdditionalULForwardingUPTNLInformation, rj},
			{IDDataForwardingResponseERABList, ig},
		}},
		{"UL-NGU-UP-TNLModifyItem-ExtIEs", extIEsULNGUUPTNLModifyItem, []row{
			{IDRedundantULNGUUPTNLInformation, ig}, {IDRedundantDLNGUUPTNLInformation, ig},
		}},
		{"AdditionalDLUPTNLInformationForHOItem-ExtIEs", extIEsAdditionalDLUPTNLInformationForHOItem, []row{
			{IDAdditionalRedundantDLNGUUPTNLInformation, ig},
		}},
		{"QosFlowAddOrModifyRequestItem-ExtIEs", extIEsQosFlowAddOrModifyRequestItem, []row{{IDULNGUUPTNLInformation, ig}}},
		{"QosFlowAddOrModifyResponseItem-ExtIEs", extIEsQosFlowAddOrModifyResponseItem, []row{{IDDLNGUUPTNLInformation, ig}}},
		{"PDUSessionResourceModifyItemModReq-ExtIEs", extIEsPDUSessionResourceModifyItemModReq, []row{{IDSNSSAI, rj}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := critOf(ids(tc.table), crits(tc.table))
			next := 0

			for _, w := range tc.want {
				i := slices.Index(got[next:], w)
				if i < 0 {
					t.Fatalf("rows %v, want %v in this order", got, tc.want)
				}

				next += i + 1
			}
		})
	}
}

func TestTNLExtensionWireCriticalityKept(t *testing.T) {
	in := &HandoverCommandTransfer{ULForwardingUPTNLInformation: upTNLPtr(1, 0xa05)}
	restore := swapTable(&extIEsHandoverCommandTransfer, tableRow(extIEsHandoverCommandTransfer, IDULForwardingUPTNLInformation, CriticalityIgnore))

	b, err := in.Marshal()

	restore()

	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseHandoverCommandTransfer(b)
	if err != nil {
		t.Fatal(err)
	}

	again, err := out.Marshal()
	if err != nil || !bytes.Equal(again, b) {
		t.Fatalf("re-encoded %x (%v), want %x", again, err, b)
	}
}

func tbNil(t testing.TB, err error) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}
}

func unknownRawExtensions(t testing.TB) extensionMeta {
	t.Helper()

	var e extensionMeta
	e.Append(rawIE{ID: unknownExtensionID, Criticality: CriticalityIgnore, Value: []byte{0xde, 0xad}})

	return e
}

func unknownAdditions(t testing.TB) per.ExtensionAdditions {
	t.Helper()

	w := per.NewWriter()
	tbNil(t, per.EncodeNormallySmallLength(w, per.Aligned, 1, func(int64) error {
		w.WriteBit(true)
		return nil
	}))
	w.AlignToByte()
	tbNil(t, per.EncodeOpenTypeBytes(w, per.Aligned, []byte{0xbe, 0xef}))
	w.AlignToByte()

	var e per.ExtensionAdditions
	tbNil(t, e.UnmarshalPER(per.NewReader(w.Bytes()), per.Aligned))

	return e
}

func unknownMeta(t testing.TB) extensionMeta {
	t.Helper()

	var m extensionMeta
	m.Append(rawIE{ID: unknownExtensionID, Criticality: CriticalityIgnore, Value: []byte{0xde, 0xad}})

	return m
}

type losslessCase struct {
	name   string
	value  func(t testing.TB) per.Marshaler
	decode func(b []byte) (per.Marshaler, error)
}

func decodeAs[T any, PT interface {
	*T
	per.Marshaler
	per.Unmarshaler
}](b []byte) (per.Marshaler, error) {
	v := PT(new(T))
	if err := v.UnmarshalPER(per.NewReader(b), per.Aligned); err != nil {
		return nil, err
	}

	return v, nil
}

func losslessCases() []losslessCase {
	snssai := func(t testing.TB) SNSSAI {
		return SNSSAI{SST: 1, iEExtensions: unknownRawExtensions(t), extensions: unknownAdditions(t)}
	}

	arp := func(t testing.TB) AllocationAndRetentionPriority {
		return AllocationAndRetentionPriority{
			PriorityLevelARP: 8, PreemptionCapability: PreemptionShallNotTrigger, PreemptionVulnerability: PreemptionNotPreemptable,
			iEExtensions: unknownRawExtensions(t), extensions: unknownAdditions(t),
		}
	}

	qosParams := func(t testing.TB) QosFlowLevelQosParameters {
		return QosFlowLevelQosParameters{
			QosCharacteristics: QosCharacteristics{Dynamic5QI: &Dynamic5QIDescriptor{
				PriorityLevelQos:  10,
				PacketDelayBudget: 100,
				PacketErrorRate:   PacketErrorRate{Scalar: 1, Exponent: 6, iEExtensions: unknownRawExtensions(t), extensions: unknownAdditions(t)},
				iEExtensions:      unknownRawExtensions(t),
				extensions:        unknownAdditions(t),
			}},
			AllocationAndRetentionPriority: arp(t),
			GBRQosInformation: &GBRQosInformation{
				MaximumFlowBitRateDL: 1, MaximumFlowBitRateUL: 2, GuaranteedFlowBitRateDL: 3, GuaranteedFlowBitRateUL: 4,
				iEExtensions: unknownRawExtensions(t), extensions: unknownAdditions(t),
			},
			iEExtensions: unknownRawExtensions(t),
			extensions:   unknownAdditions(t),
		}
	}

	gtp := func(t testing.TB, host byte, teid uint32) UPTransportLayerInformation {
		u := upTNL(host, teid)
		u.GTPTunnel.iEExtensions = unknownRawExtensions(t)
		u.GTPTunnel.extensions = unknownAdditions(t)

		return u
	}

	qosTNL := func(t testing.TB) QosFlowPerTNLInformation {
		return QosFlowPerTNLInformation{
			UPTransportLayerInformation: gtp(t, 3, 0x11),
			AssociatedQosFlowList:       AssociatedQosFlowList{{QosFlowIdentifier: 1}},
			iEExtensions:                unknownRawExtensions(t),
			extensions:                  unknownAdditions(t),
		}
	}

	transferItem := func(t testing.TB) (extensionMeta, per.ExtensionAdditions) {
		return unknownRawExtensions(t), unknownAdditions(t)
	}

	return []losslessCase{
		{name: "GTPTunnel", value: func(t testing.TB) per.Marshaler {
			u := gtp(t, 1, 0x10)
			return u.GTPTunnel
		}, decode: decodeAs[GTPTunnel]},
		{name: "SNSSAI", value: func(t testing.TB) per.Marshaler { s := snssai(t); return &s }, decode: decodeAs[SNSSAI]},
		{name: "UPTransportLayerInformationItem", value: func(t testing.TB) per.Marshaler {
			return &UPTransportLayerInformationItem{NGUUPTNLInformation: gtp(t, 1, 0x12), iEExtensions: unknownRawExtensions(t), extensions: unknownAdditions(t)}
		}, decode: decodeAs[UPTransportLayerInformationItem]},
		{name: "UPTransportLayerInformationPairItem", value: func(t testing.TB) per.Marshaler {
			return &UPTransportLayerInformationPairItem{
				ULNGUUPTNLInformation: gtp(t, 1, 0x13), DLNGUUPTNLInformation: gtp(t, 3, 0x14),
				iEExtensions: unknownRawExtensions(t), extensions: unknownAdditions(t),
			}
		}, decode: decodeAs[UPTransportLayerInformationPairItem]},
		{name: "UL-NGU-UP-TNLModifyItem", value: func(t testing.TB) per.Marshaler {
			return &ULNGUUPTNLModifyItem{
				ULNGUUPTNLInformation: gtp(t, 1, 0x15), DLNGUUPTNLInformation: gtp(t, 3, 0x16),
				RedundantULNGUUPTNLInformation: upTNLPtr(2, 0x17),
				iEExtensions:                   unknownMeta(t), extensions: unknownAdditions(t),
			}
		}, decode: decodeAs[ULNGUUPTNLModifyItem]},
		{name: "QosFlowPerTNLInformation", value: func(t testing.TB) per.Marshaler { q := qosTNL(t); return &q }, decode: decodeAs[QosFlowPerTNLInformation]},
		{name: "QosFlowPerTNLInformationItem", value: func(t testing.TB) per.Marshaler {
			return &QosFlowPerTNLInformationItem{QosFlowPerTNLInformation: qosTNL(t), iEExtensions: unknownRawExtensions(t), extensions: unknownAdditions(t)}
		}, decode: decodeAs[QosFlowPerTNLInformationItem]},
		{name: "DataForwardingResponseDRBItem", value: func(t testing.TB) per.Marshaler {
			ul := gtp(t, 3, 0x19)

			return &DataForwardingResponseDRBItem{
				DRBID: 1, DLForwardingUPTNLInformation: upTNLPtr(3, 0x18), ULForwardingUPTNLInformation: &ul,
				iEExtensions: unknownRawExtensions(t), extensions: unknownAdditions(t),
			}
		}, decode: decodeAs[DataForwardingResponseDRBItem]},
		{name: "DataForwardingResponseERABListItem", value: func(t testing.TB) per.Marshaler {
			return &DataForwardingResponseERABListItem{ERABID: 5, DLForwardingUPTNLInformation: gtp(t, 3, 0x1a), iEExtensions: unknownRawExtensions(t), extensions: unknownAdditions(t)}
		}, decode: decodeAs[DataForwardingResponseERABListItem]},
		{name: "AdditionalDLUPTNLInformationForHOItem", value: func(t testing.TB) per.Marshaler {
			return &AdditionalDLUPTNLInformationForHOItem{
				AdditionalDLNGUUPTNLInformation: gtp(t, 3, 0x1b),
				AdditionalQosFlowSetupResponse: QosFlowListWithDataForwarding{{
					QosFlowIdentifier: 1, iEExtensions: unknownRawExtensions(t), extensions: unknownAdditions(t),
				}},
				AdditionalRedundantDLNGUUPTNLInformation: upTNLPtr(4, 0x1c),
				iEExtensions:                             unknownMeta(t), extensions: unknownAdditions(t),
			}
		}, decode: decodeAs[AdditionalDLUPTNLInformationForHOItem]},
		{name: "UserPlaneFailureIndication", value: func(t testing.TB) per.Marshaler {
			return &UserPlaneFailureIndication{
				UserPlaneFailureType:  UserPlaneFailureTypeGTPUErrorIndicationReceived,
				ULNGUUPTNLInformation: gtp(t, 1, 0x1d), DLNGUUPTNLInformation: gtp(t, 3, 0x1e),
				iEExtensions: unknownRawExtensions(t), extensions: unknownAdditions(t),
			}
		}, decode: decodeAs[UserPlaneFailureIndication]},
		{name: "QosFlowAddOrModifyRequestItem", value: func(t testing.TB) per.Marshaler {
			q := qosParams(t)

			return &QosFlowAddOrModifyRequestItem{
				QosFlowIdentifier: 1, QosFlowLevelQosParameters: &q, ERABID: Ptr(ERABID(5)),
				ULNGUUPTNLInformation: upTNLPtr(1, 0x1f), iEExtensions: unknownMeta(t), extensions: unknownAdditions(t),
			}
		}, decode: decodeAs[QosFlowAddOrModifyRequestItem]},
		{name: "QosFlowAddOrModifyResponseItem", value: func(t testing.TB) per.Marshaler {
			return &QosFlowAddOrModifyResponseItem{QosFlowIdentifier: 1, DLNGUUPTNLInformation: upTNLPtr(3, 0x20), iEExtensions: unknownMeta(t), extensions: unknownAdditions(t)}
		}, decode: decodeAs[QosFlowAddOrModifyResponseItem]},
		{name: "QosFlowSetupRequestItem", value: func(t testing.TB) per.Marshaler {
			return &QosFlowSetupRequestItem{QosFlowIdentifier: 1, QosFlowLevelQosParameters: qosParams(t), iEExtensions: unknownRawExtensions(t), extensions: unknownAdditions(t)}
		}, decode: decodeAs[QosFlowSetupRequestItem]},
		{name: "NonDynamic5QIDescriptor", value: func(t testing.TB) per.Marshaler {
			return &NonDynamic5QIDescriptor{FiveQI: 9, iEExtensions: unknownRawExtensions(t), extensions: unknownAdditions(t)}
		}, decode: decodeAs[NonDynamic5QIDescriptor]},
		{name: "PDUSessionAggregateMaximumBitRate", value: func(t testing.TB) per.Marshaler {
			return &PDUSessionAggregateMaximumBitRate{DL: 1000, UL: 2000, iEExtensions: unknownRawExtensions(t), extensions: unknownAdditions(t)}
		}, decode: decodeAs[PDUSessionAggregateMaximumBitRate]},
		{name: "SecurityIndication", value: func(t testing.TB) per.Marshaler {
			return &SecurityIndication{iEExtensions: unknownRawExtensions(t), extensions: unknownAdditions(t)}
		}, decode: decodeAs[SecurityIndication]},
		{name: "SecurityResult", value: func(t testing.TB) per.Marshaler {
			return &SecurityResult{iEExtensions: unknownRawExtensions(t), extensions: unknownAdditions(t)}
		}, decode: decodeAs[SecurityResult]},
		{name: "UserPlaneSecurityInformation", value: func(t testing.TB) per.Marshaler {
			return &UserPlaneSecurityInformation{
				SecurityResult:     SecurityResult{iEExtensions: unknownRawExtensions(t)},
				SecurityIndication: SecurityIndication{extensions: unknownAdditions(t)},
				iEExtensions:       unknownRawExtensions(t), extensions: unknownAdditions(t),
			}
		}, decode: decodeAs[UserPlaneSecurityInformation]},
		{name: "QosFlowWithCauseItem", value: func(t testing.TB) per.Marshaler {
			return &QosFlowWithCauseItem{QosFlowIdentifier: 1, Cause: CauseMisc(0).Cause(), iEExtensions: unknownRawExtensions(t), extensions: unknownAdditions(t)}
		}, decode: decodeAs[QosFlowWithCauseItem]},
		{name: "QosFlowAcceptedItem", value: func(t testing.TB) per.Marshaler {
			return &QosFlowAcceptedItem{QosFlowIdentifier: 1, iEExtensions: unknownRawExtensions(t), extensions: unknownAdditions(t)}
		}, decode: decodeAs[QosFlowAcceptedItem]},
		{name: "QosFlowItemWithDataForwarding", value: func(t testing.TB) per.Marshaler {
			return &QosFlowItemWithDataForwarding{QosFlowIdentifier: 1, DataForwardingAccepted: Ptr(DataForwardingAcceptedTrue), iEExtensions: unknownRawExtensions(t), extensions: unknownAdditions(t)}
		}, decode: decodeAs[QosFlowItemWithDataForwarding]},
		{name: "QosFlowToBeForwardedItem", value: func(t testing.TB) per.Marshaler {
			return &QosFlowToBeForwardedItem{QosFlowIdentifier: 1, iEExtensions: unknownRawExtensions(t), extensions: unknownAdditions(t)}
		}, decode: decodeAs[QosFlowToBeForwardedItem]},
		{name: "QosFlowModifyConfirmItem", value: func(t testing.TB) per.Marshaler {
			return &QosFlowModifyConfirmItem{QosFlowIdentifier: 1, iEExtensions: unknownRawExtensions(t), extensions: unknownAdditions(t)}
		}, decode: decodeAs[QosFlowModifyConfirmItem]},
		{name: "PDUSessionResourceSetupItemSUReq", value: func(t testing.TB) per.Marshaler {
			e, a := transferItem(t)
			return &PDUSessionResourceSetupItemSUReq{PDUSessionID: 1, SNSSAI: snssai(t), Transfer: TransferContainer{0x00}, iEExtensions: e, extensions: a}
		}, decode: decodeAs[PDUSessionResourceSetupItemSUReq]},
		{name: "PDUSessionResourceSetupItemSURes", value: func(t testing.TB) per.Marshaler {
			e, a := transferItem(t)
			return &PDUSessionResourceSetupItemSURes{PDUSessionID: 1, Transfer: TransferContainer{0x00}, iEExtensions: e, extensions: a}
		}, decode: decodeAs[PDUSessionResourceSetupItemSURes]},
		{name: "PDUSessionResourceSetupItemCxtReq", value: func(t testing.TB) per.Marshaler {
			e, a := transferItem(t)
			return &PDUSessionResourceSetupItemCxtReq{PDUSessionID: 1, SNSSAI: snssai(t), Transfer: TransferContainer{0x00}, iEExtensions: e, extensions: a}
		}, decode: decodeAs[PDUSessionResourceSetupItemCxtReq]},
		{name: "PDUSessionResourceSetupItemCxtRes", value: func(t testing.TB) per.Marshaler {
			e, a := transferItem(t)
			return &PDUSessionResourceSetupItemCxtRes{PDUSessionID: 1, Transfer: TransferContainer{0x00}, iEExtensions: e, extensions: a}
		}, decode: decodeAs[PDUSessionResourceSetupItemCxtRes]},
		{name: "PDUSessionResourceSetupItemHOReq", value: func(t testing.TB) per.Marshaler {
			e, a := transferItem(t)
			return &PDUSessionResourceSetupItemHOReq{PDUSessionID: 1, SNSSAI: snssai(t), Transfer: TransferContainer{0x00}, iEExtensions: e, extensions: a}
		}, decode: decodeAs[PDUSessionResourceSetupItemHOReq]},
		{name: "PDUSessionResourceAdmittedItem", value: func(t testing.TB) per.Marshaler {
			e, a := transferItem(t)
			return &PDUSessionResourceAdmittedItem{PDUSessionID: 1, Transfer: TransferContainer{0x00}, iEExtensions: e, extensions: a}
		}, decode: decodeAs[PDUSessionResourceAdmittedItem]},
		{name: "PDUSessionResourceHandoverItem", value: func(t testing.TB) per.Marshaler {
			e, a := transferItem(t)
			return &PDUSessionResourceHandoverItem{PDUSessionID: 1, Transfer: TransferContainer{0x00}, iEExtensions: e, extensions: a}
		}, decode: decodeAs[PDUSessionResourceHandoverItem]},
		{name: "PDUSessionResourceToBeSwitchedDLItem", value: func(t testing.TB) per.Marshaler {
			e, a := transferItem(t)
			return &PDUSessionResourceToBeSwitchedDLItem{PDUSessionID: 1, Transfer: TransferContainer{0x00}, iEExtensions: e, extensions: a}
		}, decode: decodeAs[PDUSessionResourceToBeSwitchedDLItem]},
		{name: "PDUSessionResourceSwitchedItem", value: func(t testing.TB) per.Marshaler {
			e, a := transferItem(t)
			return &PDUSessionResourceSwitchedItem{PDUSessionID: 1, Transfer: TransferContainer{0x00}, iEExtensions: e, extensions: a}
		}, decode: decodeAs[PDUSessionResourceSwitchedItem]},
		{name: "PDUSessionResourceModifyItemModReq", value: func(t testing.TB) per.Marshaler {
			return &PDUSessionResourceModifyItemModReq{
				PDUSessionID: 1, Transfer: TransferContainer{0x00}, SNSSAI: &SNSSAI{SST: 1, SD: &SD{0x01, 0x02, 0x03}},
				iEExtensions: unknownMeta(t), extensions: unknownAdditions(t),
			}
		}, decode: decodeAs[PDUSessionResourceModifyItemModReq]},
		{name: "PDUSessionResourceModifyItemModRes", value: func(t testing.TB) per.Marshaler {
			e, a := transferItem(t)
			return &PDUSessionResourceModifyItemModRes{PDUSessionID: 1, Transfer: TransferContainer{0x00}, iEExtensions: e, extensions: a}
		}, decode: decodeAs[PDUSessionResourceModifyItemModRes]},
		{name: "PDUSessionResourceModifyItemModInd", value: func(t testing.TB) per.Marshaler {
			e, a := transferItem(t)
			return &PDUSessionResourceModifyItemModInd{PDUSessionID: 1, Transfer: TransferContainer{0x00}, iEExtensions: e, extensions: a}
		}, decode: decodeAs[PDUSessionResourceModifyItemModInd]},
		{name: "PDUSessionResourceModifyItemModCfm", value: func(t testing.TB) per.Marshaler {
			e, a := transferItem(t)
			return &PDUSessionResourceModifyItemModCfm{PDUSessionID: 1, Transfer: TransferContainer{0x00}, iEExtensions: e, extensions: a}
		}, decode: decodeAs[PDUSessionResourceModifyItemModCfm]},
		{name: "PDUSessionResourceSetupResponseTransfer", value: func(t testing.TB) per.Marshaler {
			v := tnlSetupResponseTransfer()
			v.DLQosFlowPerTNLInformation = qosTNL(t)
			v.SecurityResult = &SecurityResult{iEExtensions: unknownRawExtensions(t)}
			v.QosFlowFailedToSetup = QosFlowListWithCause{{QosFlowIdentifier: 2, Cause: CauseMisc(0).Cause(), extensions: unknownAdditions(t)}}
			v.iEExtensions, v.extensions = unknownMeta(t), unknownAdditions(t)

			return v
		}, decode: decodeAs[PDUSessionResourceSetupResponseTransfer]},
		{name: "PDUSessionResourceModifyResponseTransfer", value: func(t testing.TB) per.Marshaler {
			v := tnlModifyResponseTransfer()
			v.iEExtensions, v.extensions = unknownMeta(t), unknownAdditions(t)

			return v
		}, decode: decodeAs[PDUSessionResourceModifyResponseTransfer]},
		{name: "PDUSessionResourceModifyIndicationTransfer", value: func(t testing.TB) per.Marshaler {
			v := tnlModifyIndicationTransfer()
			v.iEExtensions, v.extensions = unknownMeta(t), unknownAdditions(t)

			return v
		}, decode: decodeAs[PDUSessionResourceModifyIndicationTransfer]},
		{name: "PDUSessionResourceModifyConfirmTransfer", value: func(t testing.TB) per.Marshaler {
			v := tnlModifyConfirmTransfer()
			v.iEExtensions, v.extensions = unknownMeta(t), unknownAdditions(t)

			return v
		}, decode: decodeAs[PDUSessionResourceModifyConfirmTransfer]},
		{name: "PathSwitchRequestTransfer", value: func(t testing.TB) per.Marshaler {
			v := tnlPathSwitchRequestTransfer()
			v.UserPlaneSecurityInformation = &UserPlaneSecurityInformation{iEExtensions: unknownRawExtensions(t)}
			v.iEExtensions, v.extensions = unknownMeta(t), unknownAdditions(t)

			return v
		}, decode: decodeAs[PathSwitchRequestTransfer]},
		{name: "PathSwitchRequestAcknowledgeTransfer", value: func(t testing.TB) per.Marshaler {
			v := tnlPathSwitchRequestAcknowledgeTransfer()
			v.SecurityIndication = &SecurityIndication{iEExtensions: unknownRawExtensions(t)}
			v.iEExtensions, v.extensions = unknownMeta(t), unknownAdditions(t)

			return v
		}, decode: decodeAs[PathSwitchRequestAcknowledgeTransfer]},
		{name: "HandoverRequestAcknowledgeTransfer", value: func(t testing.TB) per.Marshaler {
			v := tnlHandoverRequestAcknowledgeTransfer()
			v.iEExtensions, v.extensions = unknownMeta(t), unknownAdditions(t)

			return v
		}, decode: decodeAs[HandoverRequestAcknowledgeTransfer]},
		{name: "HandoverCommandTransfer", value: func(t testing.TB) per.Marshaler {
			v := tnlHandoverCommandTransfer()
			v.iEExtensions, v.extensions = unknownMeta(t), unknownAdditions(t)

			return v
		}, decode: decodeAs[HandoverCommandTransfer]},
	}
}

func TestTNLStructuresKeepUnknownExtensions(t *testing.T) {
	goldens := losslessGoldens()

	var regenerated []string

	for _, c := range losslessCases() {
		t.Run(c.name, func(t *testing.T) {
			in := c.value(t)

			b, err := per.Marshal(in)
			if err != nil {
				t.Fatal(err)
			}

			if os.Getenv("NGAP_TNL_GOLDEN_UPDATE") != "" {
				regenerated = append(regenerated, fmt.Sprintf("\t\t%q: %q,", c.name, hex.EncodeToString(b)))
			} else if got := hex.EncodeToString(b); got != goldens[c.name] {
				t.Fatalf("encoded\n got %s\nwant %s", got, goldens[c.name])
			}

			out, err := c.decode(b)
			if err != nil {
				t.Fatal(err)
			}

			again, err := per.Marshal(out)
			if err != nil || !bytes.Equal(again, b) {
				t.Fatalf("re-encoded %x (%v), want %x", again, err, b)
			}

			twice, err := c.decode(again)
			if err != nil {
				t.Fatal(err)
			}

			if !equalByMethod(twice, out) {
				t.Fatalf("re-decoded\n got %+v\nwant %+v", twice, out)
			}

			if !bytes.Contains(b, []byte{0xfd, 0xe8}) || !bytes.Contains(b, []byte{0xde, 0xad}) || !bytes.Contains(b, []byte{0xbe, 0xef}) {
				t.Fatalf("%x lacks the unknown extension or the extension addition", b)
			}
		})
	}

	if len(regenerated) > 0 {
		t.Log("\n" + strings.Join(regenerated, "\n"))
	}
}

func TestTNLItemKeepsUnknownExtensionCriticalityAndOrder(t *testing.T) {
	in := &HandoverCommandTransfer{ULForwardingUPTNLInformation: upTNLPtr(1, 0xa05)}
	restore := swapTable(&extIEsHandoverCommandTransfer,
		rawRow[HandoverCommandTransfer](unknownExtensionID, CriticalityNotify, []byte{0x01}),
		tableRow(extIEsHandoverCommandTransfer, IDULForwardingUPTNLInformation, CriticalityReject),
	)

	b, err := in.Marshal()

	restore()

	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseHandoverCommandTransfer(b)
	if err != nil {
		t.Fatal(err)
	}

	if got := out.iEExtensions.UnknownIEs(); len(got) != 1 || got[0].ID != unknownExtensionID || got[0].Criticality != CriticalityNotify {
		t.Fatalf("unknown extensions %+v", got)
	}

	again, err := out.Marshal()
	if err != nil || !bytes.Equal(again, b) {
		t.Fatalf("re-encoded %x (%v), want %x", again, err, b)
	}
}

func TestTNLTransferRejectsUnknownRejectExtension(t *testing.T) {
	in := &HandoverCommandTransfer{}
	in.iEExtensions.Append(rawIE{ID: unknownExtensionID, Criticality: CriticalityReject, Value: []byte{0x01}})

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ParseHandoverCommandTransfer(b); err == nil {
		t.Fatal("an unknown reject-criticality extension decoded")
	}
}

func tnlFuzzParsers() []func([]byte) (per.Marshaler, error) {
	return []func([]byte) (per.Marshaler, error){
		decodeAs[PDUSessionResourceSetupResponseTransfer],
		decodeAs[PDUSessionResourceModifyResponseTransfer],
		decodeAs[PDUSessionResourceModifyIndicationTransfer],
		decodeAs[PDUSessionResourceModifyConfirmTransfer],
		decodeAs[PathSwitchRequestTransfer],
		decodeAs[PathSwitchRequestAcknowledgeTransfer],
		decodeAs[HandoverRequestAcknowledgeTransfer],
		decodeAs[HandoverCommandTransfer],
		func(b []byte) (per.Marshaler, error) {
			v, err := ParsePDUSessionResourceSetupRequestTransfer(b)
			if err != nil {
				return nil, err
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeMessageBody(w, enc, ProcPDUSessionResourceSetup, pDUSessionResourceSetupRequestTransferIEs, v)
			}), nil
		},
		func(b []byte) (per.Marshaler, error) {
			v, err := ParsePDUSessionResourceModifyRequestTransfer(b)
			if err != nil {
				return nil, err
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeMessageBody(w, enc, ProcPDUSessionResourceModify, pDUSessionResourceModifyRequestTransferIEs, v)
			}), nil
		},
	}
}

func FuzzTNLTransferRoundTrip(f *testing.F) {
	parsers := tnlFuzzParsers()

	for _, c := range tnlTransferCases() {
		b, err := c.transfer.Marshal()
		if err != nil {
			f.Fatal(err)
		}

		for i := range parsers {
			f.Add(byte(i), []byte(b))
		}
	}

	for i, c := range losslessCases() {
		b, err := per.Marshal(c.value(f))
		if err != nil {
			f.Fatal(err)
		}

		f.Add(byte(i), b)
	}

	f.Fuzz(func(t *testing.T, kind byte, data []byte) {
		parse := parsers[int(kind)%len(parsers)]

		v, err := parse(data)
		if err != nil {
			return
		}

		b, err := per.Marshal(v)
		if err != nil {
			t.Fatalf("encode decoded value: %v", err)
		}

		again, err := parse(b)
		if err != nil {
			t.Fatalf("decode re-encoded value %x: %v", b, err)
		}

		b2, err := per.Marshal(again)
		if err != nil || !bytes.Equal(b2, b) {
			t.Fatalf("re-encoded %x (%v), want %x", b2, err, b)
		}
	})
}

func losslessGoldens() map[string]string {
	return map[string]string{
		"GTPTunnel":                                  "c3e00a000001000000100000fde84002dead0102beef",
		"SNSSAI":                                     "a0200000fde84002dead0102beef",
		"UPTransportLayerInformationItem":            "d87c0a000001000000120000fde84002dead0102beef0000fde84002dead0102beef",
		"UPTransportLayerInformationPairItem":        "d87c0a000001000000130000fde84002dead0102beef61f00a000003000000140000fde84002dead0102beef0000fde84002dead0102beef",
		"UL-NGU-UP-TNLModifyItem":                    "d87c0a000001000000150000fde84002dead0102beef61f00a000003000000160000fde84002dead0102beef0001fde84002dead00c3400a01f00a000002000000170102beef",
		"QosFlowPerTNLInformation":                   "d87c0a000003000000110000fde84002dead0102beef00010000fde84002dead0102beef",
		"QosFlowPerTNLInformationItem":               "f61f0a000003000000110000fde84002dead0102beef00010000fde84002dead0102beef0000fde84002dead0102beef",
		"DataForwardingResponseDRBItem":              "f0007c0a0000030000001861f00a000003000000190000fde84002dead0102beef0000fde84002dead0102beef",
		"DataForwardingResponseERABListItem":         "cac3e00a0000030000001a0000fde84002dead0102beef0000fde84002dead0102beef",
		"AdditionalDLUPTNLInformationForHOItem":      "ac3e0a0000030000001b0000fde84002dead0102beef02810000fde84002dead0102beef0001fde84002dead00b7400a01f00a0000040000001c0102beef",
		"UserPlaneFailureIndication":                 "c61f0a0000010000001d0000fde84002dead0102beef61f00a0000030000001e0000fde84002dead0102beef0000fde84002dead0102beef",
		"QosFlowAddOrModifyRequestItem":              "f0396109000064c2600000fde84002dead0102beef0000fde84002dead0102beefdc000000fde84002dead0102beef8800010002000300040000fde84002dead0102beef0000fde84002dead0102beef280001fde84002dead008b400a01f00a0000010000001f0102beef",
		"QosFlowAddOrModifyResponseItem":             "c0800001fde84002dead0080400a01f00a000003000000200102beef",
		"QosFlowSetupRequestItem":                    "a072c2120064c2600000fde84002dead0102beef0000fde84002dead0102beefdc000000fde84002dead0102beef8800010002000300040000fde84002dead0102beef0000fde84002dead0102beef0000fde84002dead0102beef",
		"NonDynamic5QIDescriptor":                    "88090000fde84002dead0102beef",
		"PDUSessionAggregateMaximumBitRate":          "c403e81007d00000fde84002dead0102beef",
		"SecurityIndication":                         "a0000000fde84002dead0102beef",
		"SecurityResult":                             "c00000fde84002dead0102beef",
		"UserPlaneSecurityInformation":               "d00000fde84002dead80008002beef0000fde84002dead0102beef",
		"QosFlowWithCauseItem":                       "c0c00000fde84002dead0102beef",
		"QosFlowAcceptedItem":                        "c0800000fde84002dead0102beef",
		"QosFlowItemWithDataForwarding":              "e0400000fde84002dead0102beef",
		"QosFlowToBeForwardedItem":                   "c0800000fde84002dead0102beef",
		"QosFlowModifyConfirmItem":                   "c0800000fde84002dead0102beef",
		"PDUSessionResourceSetupItemSUReq":           "a001a0200000fde84002dead0102beef01000000fde84002dead0102beef",
		"PDUSessionResourceSetupItemSURes":           "c00101000000fde84002dead0102beef",
		"PDUSessionResourceSetupItemCxtReq":          "a001a0200000fde84002dead0102beef01000000fde84002dead0102beef",
		"PDUSessionResourceSetupItemCxtRes":          "c00101000000fde84002dead0102beef",
		"PDUSessionResourceSetupItemHOReq":           "c001a0200000fde84002dead0102beef01000000fde84002dead0102beef",
		"PDUSessionResourceAdmittedItem":             "c00101000000fde84002dead0102beef",
		"PDUSessionResourceHandoverItem":             "c00101000000fde84002dead0102beef",
		"PDUSessionResourceToBeSwitchedDLItem":       "c00101000000fde84002dead0102beef",
		"PDUSessionResourceSwitchedItem":             "c00101000000fde84002dead0102beef",
		"PDUSessionResourceModifyItemModReq":         "a00101000001fde84002dead0094000540200102030102beef",
		"PDUSessionResourceModifyItemModRes":         "c00101000000fde84002dead0102beef",
		"PDUSessionResourceModifyItemModInd":         "c00101000000fde84002dead0102beef",
		"PDUSessionResourceModifyItemModCfm":         "c00101000000fde84002dead0102beef",
		"PDUSessionResourceSetupResponseTransfer":    "fec3e00a000003000000110000fde84002dead0102beef00010000fde84002dead0102beef0007c00a000003000002020001400000fde84002dead0205000402beef0002fde84002dead00c1400c007c0a00000400000203000100b8400d0007c00a0000040000020400010102beef",
		"PDUSessionResourceModifyResponseTransfer":   "fa03e00a0000030000040101f00a00000100000402010200000080400a01f00a000003000004030007c00a0000030000040400010005fde84002dead009a4014001f0a0000010000040501f00a0000030000040600c0400a01f00a0000040000040700c3400a01f00a0000020000040800b8400d0007c00a00000400000409000100b94014001f0a0000020000040a01f00a0000040000040b0102beef",
		"PDUSessionResourceModifyIndicationTransfer": "e00f800a0000030000050100010007c00a0000030000050200010002fde84002dead00c1400c007c0a00000400000503000100b8400d0007c00a0000040000050400010102beef",
		"PDUSessionResourceModifyConfirmTransfer":    "d000203e0a00000100000601001f0a0000010000060201f00a000003000006030002fde84002dead00c3400a01f00a0000020000060400b94014001f0a0000020000060501f00a000004000006060102beef",
		"PathSwitchRequestTransfer":                  "b01f0a000003000007014000000000fde84002dead00020004fde84002dead009b400d0007c00a00000300000702000100c0400a01f00a0000040000070300bf40010000b8400d0007c00a0000040000070400010102beef",
		"PathSwitchRequestAcknowledgeTransfer":       "f01f0a0000010000080120000000fde84002dead0003fde84002dead009a4014001f0a0000010000080201f00a0000030000080300c3400a01f00a0000020000080400b94014001f0a0000020000080501f00a000004000008060102beef",
		"HandoverRequestAcknowledgeTransfer":         "cc07c00a0000030000090101f00a000003000009020001030003e00a0000030000090301f00a000003000009040005fde84002dead00994027180f800a00000300000905000101f00a00000300000906000000b7400a01f00a0000040000090700a4000a01f00a0000030000090800ac000a001f0a0000030000090900f9400c000a03e00a0000030000090a00c0400a01f00a0000040000090b0102beef",
		"HandoverCommandTransfer":                    "f80f800a00000100000a010002060007c00a00000100000a0201f00a00000100000a030004fde84002dead0098400d0007c00a00000100000a04000100a4000a01f00a00000100000a0500ac000a001f0a00000100000a0600f9400c000a03e00a00000100000a070102beef",
	}
}

const (
	goldenTNLSetupRequest                 = "001d0064000003000a00020001005500020002004a005100000500204b000006008b000a01f00a00000100000101007e000a001f0a0000010000010200860001000088000700010000091c0000c3400a01f00a0000020000010300ba400a001f0a00000200000104"
	goldenTNLSetupResponse                = "201d0054000003000a40020001005540020002004b40410000053d4803e00a0000030000020100010007c00a000003000002020001000100c1400c007c0a00000400000203000100b8400d0007c00a000004000002040001"
	goldenTNLModifyRequest                = "001a0080b6000003000a0002000100550002000200400080a20020058092000006008c0032101f0a0000010000030101f00a00000300000302000100c3400a01f00a0000020000030300c0400a01f00a00000400000304008700130040800000008b400a01f00a00000100000305007e000a001f0a0000010000030600ba400a001f0a0000020000030700c3400a01f00a0000020000030801b34014101f0a0000010000030901f00a0000030000030a0000009400054020010203"
	goldenTNLModifyResponse               = "201a0080ac000003000a40020001005540020002004140809800000580937a03e00a0000030000040101f00a00000100000402010200000080400a01f00a000003000004030007c00a0000030000040400010004009a4014001f0a0000010000040501f00a0000030000040600c0400a01f00a0000040000040700c3400a01f00a0000020000040800b8400d0007c00a00000400000409000100b94014001f0a0000020000040a01f00a0000040000040b"
	goldenTNLModifyIndication             = "001b0054000003000a00020001005500020002003f00410000053d600f800a0000030000050100010007c00a000003000005020001000100c1400c007c0a00000400000503000100b8400d0007c00a000004000005040001"
	goldenTNLModifyConfirm                = "201b005f000003000a40020001005540020002003e404c000005485000203e0a00000100000601001f0a0000010000060201f00a00000300000603000100c3400a01f00a0000020000060400b94014001f0a0000020000060501f00a00000400000606"
	goldenTNLPathSwitchRequest            = "0019007a0000050055000200020064000200010079400f4000f110000000000000f11000000100774009000000000000000000004c004700000543101f0a0000030000070100020003009b400d0007c00a00000300000702000100c0400a01f00a0000040000070300bf40010000b8400d0007c00a000004000007040001"
	goldenTNLPathSwitchRequestAcknowledge = "201900808c000005000a40020001005540020002005d0021000000000000000000000000000000000000000000000000000000000000000000004d404e0000054a501f0a000001000008010002009a4014001f0a0000010000080201f00a0000030000080300c3400a01f00a0000020000080400b94014001f0a0000020000080501f00a00000400000806000000020001"
	goldenTNLHandoverRequestAcknowledge   = "200d0080b4000004000a40020001005540020002003540809900000580944c07c00a0000030000090101f00a000003000009020001030003e00a0000030000090301f00a00000300000904000400994027180f800a00000300000905000101f00a00000300000906000000b7400a01f00a0000040000090700a4000a01f00a0000030000090800ac000a001f0a0000030000090900f9400c000a03e00a0000030000090a00c0400a01f00a0000040000090b006a0003020000"
	goldenTNLHandoverCommand              = "200c008085000005000a00020001005500020002001d000100003b406600000562780f800a00000100000a010002060007c00a00000100000a0201f00a00000100000a0300030098400d0007c00a00000100000a04000100a4000a01f00a00000100000a0500ac000a001f0a00000100000a0600f9400c000a03e00a00000100000a07006a0003020000"
)

func TestTNLRequestTransfersKeepUnknownIEsAndAdditions(t *testing.T) {
	setup := tnlSetupRequestTransfer()
	setup.appendUnknown(rawIE{ID: unknownExtensionID, Criticality: CriticalityIgnore, Value: []byte{0xde, 0xad}})
	setup.setExtensionAdditions(unknownAdditions(t))

	modify := tnlModifyRequestTransfer()
	modify.appendUnknown(rawIE{ID: unknownExtensionID, Criticality: CriticalityNotify, Value: []byte{0xde, 0xad}})
	modify.setExtensionAdditions(unknownAdditions(t))

	for _, c := range []struct {
		name     string
		transfer interface {
			Marshal() (TransferContainer, error)
		}
		parse func(TransferContainer) (interface {
			Marshal() (TransferContainer, error)
		}, error)
	}{
		{"PDUSessionResourceSetupRequestTransfer", setup, func(b TransferContainer) (interface {
			Marshal() (TransferContainer, error)
		}, error,
		) {
			return ParsePDUSessionResourceSetupRequestTransfer(b)
		}},
		{"PDUSessionResourceModifyRequestTransfer", modify, func(b TransferContainer) (interface {
			Marshal() (TransferContainer, error)
		}, error,
		) {
			return ParsePDUSessionResourceModifyRequestTransfer(b)
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			b, err := c.transfer.Marshal()
			if err != nil {
				t.Fatal(err)
			}

			out, err := c.parse(b)
			if err != nil {
				t.Fatal(err)
			}

			again, err := out.Marshal()
			if err != nil || !bytes.Equal(again, b) {
				t.Fatalf("re-encoded %x (%v), want %x", again, err, b)
			}

			if b[0]&0x80 == 0 || !bytes.Contains(b, []byte{0xfd, 0xe8}) || !bytes.HasSuffix(b, []byte{0x01, 0x02, 0xbe, 0xef}) {
				t.Fatalf("%x lacks the unknown IE or the extension addition", b)
			}
		})
	}
}

func TestTNLManyUnknownExtensionsKeptExactly(t *testing.T) {
	in := tnlHandoverCommandTransfer()

	fields := make([]ieField, 0, 3*paddingIEs)
	for i := range 3 * paddingIEs {
		fields = append(fields, ieField{ID: unknownExtensionID - ProtocolIEID(i), Criticality: CriticalityIgnore, Value: apimpl.RawValue([]byte{byte(i)})})
	}

	w := per.NewWriter()
	tbNil(t, encodeExtensionContainer(w, per.Aligned, fields))
	w.AlignToByte()
	tbNil(t, decodeExtensionContainer(per.NewReader(w.Bytes()), per.Aligned, extIEsHandoverCommandTransfer, in, &in.iEExtensions))

	if got := len(in.iEExtensions.UnknownIEs()); got != 3*paddingIEs {
		t.Fatalf("kept %d unknown extensions, want %d", got, 3*paddingIEs)
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseHandoverCommandTransfer(b)
	if err != nil {
		t.Fatal(err)
	}

	again, err := out.Marshal()
	if err != nil || !bytes.Equal(again, b) {
		t.Fatalf("re-encoded %x (%v), want %x", again, err, b)
	}
}

func TestTNLRequestTransferKeepsManyUnknownIEs(t *testing.T) {
	in := tnlSetupRequestTransfer()

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	r := per.NewReader(b)
	_, err = r.ReadBit()
	tbNil(t, err)

	fields, err := decodeIEContainer(r, per.Aligned)
	tbNil(t, err)

	all := make([]ieField, 0, len(fields)+3*paddingIEs)
	for _, f := range fields {
		all = append(all, f.Encodable())
	}

	for i := range 3 * paddingIEs {
		all = append(all, ieField{ID: unknownExtensionID - ProtocolIEID(i), Criticality: CriticalityIgnore, Value: apimpl.RawValue([]byte{byte(i)})})
	}

	w := per.NewWriter()
	w.WriteBit(false)
	tbNil(t, encodeIEContainer(w, per.Aligned, all))
	w.AlignToByte()

	out, err := ParsePDUSessionResourceSetupRequestTransfer(w.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	if got := len(out.UnknownIEs()); got != 3*paddingIEs {
		t.Fatalf("kept %d unknown IEs, want %d", got, 3*paddingIEs)
	}

	again, err := out.Marshal()
	if err != nil || !bytes.Equal(again, w.Bytes()) {
		t.Fatalf("re-encoded %x (%v), want %x", again, err, w.Bytes())
	}
}

func TestMessageWithManyUnknownIEsReencodesLosslessly(t *testing.T) {
	fields := ngSetupFields()
	for i := range paddingIEs + 1 {
		fields = append(fields, ieField{ID: ProtocolIEID(40000 + i), Criticality: CriticalityIgnore, Value: apimpl.RawValue([]byte{0x00})})
	}

	value := container(t, fields...)

	msg, err := ParseNGSetupRequest(value)
	if err != nil {
		t.Fatal(err)
	}

	b, err := msg.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(pdu.value(), value) {
		t.Fatalf("re-encoded %x, want %x", pdu.value(), value)
	}
}
