// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/per"
)

const idUnknownExtension ProtocolIEID = 999

func tnlExtensionContainer(fields ...ieField) []byte {
	w := per.NewWriter()

	if err := encodeExtensionContainer(w, per.Aligned, fields); err != nil {
		panic(err)
	}

	w.AlignToByte()

	return w.Bytes()
}

func tnlRawExtensions(fields ...rawIE) *rawIEExtensions {
	var ext []ieField
	for _, f := range fields {
		ext = append(ext, f.field())
	}

	var e rawIEExtensions
	if err := e.UnmarshalPER(per.NewReader(tnlExtensionContainer(ext...)), per.Aligned); err != nil {
		panic(err)
	}

	return &e
}

func tnlExtensionAdditions(contents ...[]byte) per.ExtensionAdditions {
	w := per.NewWriter()

	if err := per.EncodeNormallySmallLength(w, per.Aligned, int64(len(contents)), func(count int64) error {
		for range count {
			w.WriteBit(true)
		}

		return nil
	}); err != nil {
		panic(err)
	}

	for _, c := range contents {
		if err := per.EncodeOpenTypeBytes(w, per.Aligned, c); err != nil {
			panic(err)
		}
	}

	w.AlignToByte()

	var e per.ExtensionAdditions
	if err := e.UnmarshalPER(per.NewReader(w.Bytes()), per.Aligned); err != nil {
		panic(err)
	}

	return e
}

func tnlUnknownExtension() *rawIEExtensions {
	return tnlRawExtensions(rawIE{id: idUnknownExtension, crit: CriticalityIgnore, value: []byte{0x5a}})
}

func tnlUnknownMeta() messageMeta {
	var m messageMeta
	m.preserve(rawIE{id: idUnknownExtension, crit: CriticalityIgnore, value: []byte{0x5a}})

	return m
}

func tnlQoS() ERABLevelQoSParameters {
	return ERABLevelQoSParameters{
		QCI: 1,
		ARP: AllocationAndRetentionPriority{
			PriorityLevel: 2,
			iEExtensions:  tnlUnknownExtension(),
			extensions:    tnlExtensionAdditions([]byte{0x01}),
		},
		GBR: &GBRQosInformation{
			MaximumBitrateDL:    1000,
			MaximumBitrateUL:    1000,
			GuaranteedBitrateDL: 500,
			GuaranteedBitrateUL: 500,
			iEExtensions:        tnlUnknownMeta(),
			extensions:          tnlExtensionAdditions([]byte{0x02}),
		},
		iEExtensions: tnlUnknownMeta(),
		extensions:   tnlExtensionAdditions([]byte{0x03}),
	}
}

func ignoreCrit() containerCrit {
	c := CriticalityIgnore
	return containerCrit{crit: &c}
}

func rejectCrit() containerCrit {
	c := CriticalityReject
	return containerCrit{crit: &c}
}

type tnlValue interface {
	per.Marshaler
	per.Unmarshaler
}

func losslessRoundTrip[T any, PT interface {
	*T
	tnlValue
}](t *testing.T, in PT, want string) {
	t.Helper()

	w := per.NewWriter()
	if err := in.MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()
	b := w.Bytes()

	if want != "" && hex.EncodeToString(b) != want {
		t.Errorf("bytes = %x, want %s", b, want)
	}

	out := PT(new(T))
	if err := out.UnmarshalPER(per.NewReader(b), per.Aligned); err != nil {
		t.Fatal(err)
	}

	w = per.NewWriter()
	if err := out.MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()

	if !bytes.Equal(w.Bytes(), b) {
		t.Errorf("re-encoded %x, want %x", w.Bytes(), b)
	}

	again := PT(new(T))
	if err := again.UnmarshalPER(per.NewReader(w.Bytes()), per.Aligned); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(again, out) {
		t.Errorf("re-decoded\n got %+v\nwant %+v", again, out)
	}
}

func TestTNLItemsKnownExtensionsTyped(t *testing.T) {
	sec := &SecurityIndication{IntegrityProtectionIndication: IntegrityProtectionRequired}

	ctxt := &ERABToBeSetupItemCtxtSUReq{
		ERABID: 5, QoS: goldQoS(), TransportLayerAddress: goldTLA(), GTPTEID: 1,
		CorrelationID: &CorrelationID{1, 2, 3, 4}, SIPTOCorrelationID: &CorrelationID{5, 6, 7, 8},
		BearerType: Ptr(BearerTypeNonIP), EthernetType: Ptr(EthernetTypeTrue), SecurityIndication: sec,
	}
	ctxt.QoS.DownlinkPacketLossRate = Ptr(PacketLossRate(10))
	ctxt.QoS.UplinkPacketLossRate = Ptr(PacketLossRate(1000))
	ctxt.QoS.GBR = &GBRQosInformation{
		MaximumBitrateDL: 1, MaximumBitrateUL: 1, GuaranteedBitrateDL: 1, GuaranteedBitrateUL: 1,
		ExtendedMaximumBitrateDL:    Ptr(ExtendedBitRate(10000000001)),
		ExtendedMaximumBitrateUL:    Ptr(ExtendedBitRate(4000000000000)),
		ExtendedGuaranteedBitrateDL: Ptr(ExtendedBitRate(20000000000)),
		ExtendedGuaranteedBitrateUL: Ptr(ExtendedBitRate(30000000000)),
	}

	bearer := &ERABToBeSetupItemBearerSUReq{
		ERABID: 6, QoS: goldQoS(), TransportLayerAddress: goldTLA(), GTPTEID: 2, NASPDU: NASPDU{0x27},
		CorrelationID: &CorrelationID{1, 2, 3, 4}, BearerType: Ptr(BearerTypeNonIP), SecurityIndication: sec,
	}

	ho := &ERABToBeSetupItemHOReq{
		ERABID: 7, TransportLayerAddress: goldTLA(), GTPTEID: 3, QoS: goldQoS(),
		Extensions: &ERABToBeSetupItemHOReqExtIEs{
			DataForwardingNotPossible: Ptr(DataForwardingNotPossibleTrue),
			BearerType:                Ptr(BearerTypeNonIP),
			EthernetType:              Ptr(EthernetTypeTrue),
			SecurityIndication:        sec,
		},
	}

	switched := &ERABToBeSwitchedDLItem{ERABID: 8, TransportLayerAddress: goldTLA(), GTPTEID: 4, SecurityIndication: sec}

	for _, c := range []struct {
		name string
		in   tnlValue
		out  tnlValue
	}{
		{"E-RABToBeSetupItemCtxtSUReq", ctxt, new(ERABToBeSetupItemCtxtSUReq)},
		{"E-RABToBeSetupItemBearerSUReq", bearer, new(ERABToBeSetupItemBearerSUReq)},
		{"E-RABToBeSetupItemHOReq", ho, new(ERABToBeSetupItemHOReq)},
		{"E-RABToBeSwitchedDLItem", switched, new(ERABToBeSwitchedDLItem)},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := per.NewWriter()
			if err := c.in.MarshalPER(w, per.Aligned); err != nil {
				t.Fatal(err)
			}

			w.AlignToByte()

			if err := c.out.UnmarshalPER(per.NewReader(w.Bytes()), per.Aligned); err != nil {
				t.Fatalf("a known extension was not comprehended: %v", err)
			}

			if !reflect.DeepEqual(c.out, c.in) {
				t.Fatalf("decoded\n got %+v\nwant %+v", c.out, c.in)
			}
		})
	}
}

func TestTNLItemsLossless(t *testing.T) {
	ext := func() *rawIEExtensions { return tnlUnknownExtension() }
	add := func() per.ExtensionAdditions { return tnlExtensionAdditions([]byte{0xaa}, []byte{0xbb, 0xcc}) }

	t.Run("E-RABToBeSetupItemCtxtSUReq", func(t *testing.T) {
		losslessRoundTrip(t, &ERABToBeSetupItemCtxtSUReq{ERABID: 5, QoS: tnlQoS(), TransportLayerAddress: goldTLA(), GTPTEID: 1, NASPDU: NASPDU{0x27}, CorrelationID: &CorrelationID{1, 2, 3, 4}, BearerType: Ptr(BearerTypeNonIP), SecurityIndication: &SecurityIndication{IntegrityProtectionIndication: IntegrityProtectionRequired}, iEExtensions: tnlUnknownMeta(), extensions: add()}, "")
	})
	t.Run("E-RABSetupItemCtxtSURes", func(t *testing.T) {
		losslessRoundTrip(t, &ERABSetupItemCtxtSURes{ERABID: 5, TransportLayerAddress: goldTLA(), GTPTEID: 2, iEExtensions: ext(), extensions: add()}, "")
	})
	t.Run("E-RABToBeSetupItemBearerSUReq", func(t *testing.T) {
		losslessRoundTrip(t, &ERABToBeSetupItemBearerSUReq{ERABID: 6, QoS: tnlQoS(), TransportLayerAddress: goldTLA(), GTPTEID: 3, NASPDU: NASPDU{0x27}, SIPTOCorrelationID: &CorrelationID{5, 6, 7, 8}, EthernetType: Ptr(EthernetTypeTrue), SecurityIndication: &SecurityIndication{IntegrityProtectionIndication: IntegrityProtectionPreferred}, iEExtensions: tnlUnknownMeta(), extensions: add()}, "")
	})
	t.Run("E-RABToBeModifiedItemBearerModReq", func(t *testing.T) {
		in := &ERABToBeModifiedItemBearerModReq{ERABID: 7, QoS: tnlQoS(), NASPDU: NASPDU{0x27}, extensions: add()}

		container := tnlExtensionContainer(
			ieField{id: IDTransportInformation, crit: CriticalityReject, val: &TransportInformation{TransportLayerAddress: goldTLA(), ULGTPTEID: 4, extensions: add()}},
			ieField{id: idUnknownExtension, crit: CriticalityIgnore, raw: []byte{0x5a}},
		)

		if err := decodeExtensionContainer(per.NewReader(container), per.Aligned, eRABToBeModifyItemBearerModReqExtIEs, in, &in.iEExtensions); err != nil {
			t.Fatal(err)
		}

		if in.TransportInformation == nil || in.TransportInformation.ULGTPTEID != 4 {
			t.Fatalf("TransportInformation = %+v", in.TransportInformation)
		}

		losslessRoundTrip(t, in, "")
	})
	t.Run("E-RABToBeModifiedItemBearerModInd", func(t *testing.T) {
		losslessRoundTrip(t, &ERABToBeModifiedItemBearerModInd{ERABID: 8, TransportLayerAddress: goldTLA(), DLGTPTEID: 5, iEExtensions: ext(), extensions: add()}, "")
	})
	t.Run("E-RABToBeSetupItemHOReq", func(t *testing.T) {
		in := &ERABToBeSetupItemHOReq{ERABID: 9, TransportLayerAddress: goldTLA(), GTPTEID: 6, QoS: tnlQoS(), extensions: add()}

		container := tnlExtensionContainer(
			ieField{id: idUnknownExtension, crit: CriticalityIgnore, raw: []byte{0x5a}},
			ieField{id: IDDataForwardingNotPossible, crit: CriticalityIgnore, val: DataForwardingNotPossibleTrue},
		)

		var e ERABToBeSetupItemHOReqExtIEs
		if err := e.UnmarshalPER(per.NewReader(container), per.Aligned); err != nil {
			t.Fatal(err)
		}

		if e.DataForwardingNotPossible == nil {
			t.Fatal("Data-Forwarding-Not-Possible lost")
		}

		in.Extensions = &e

		losslessRoundTrip(t, in, "")
	})
	t.Run("E-RABAdmittedItem", func(t *testing.T) {
		losslessRoundTrip(t, &ERABAdmittedItem{
			ERABID: 10, TransportLayerAddress: goldTLA(), GTPTEID: 7,
			DLTransportLayerAddr: goldTLA(), DLGTPTEID: Ptr(GTPTEID(8)),
			ULTransportLayerAddr: goldTLA(), ULGTPTEID: Ptr(GTPTEID(9)),
			iEExtensions: ext(), extensions: add(),
		}, "")
	})
	t.Run("E-RABDataForwardingItem", func(t *testing.T) {
		losslessRoundTrip(t, &ERABDataForwardingItem{
			ERABID: 11, DLTransportLayerAddr: goldTLA(), DLGTPTEID: Ptr(GTPTEID(10)),
			iEExtensions: ext(), extensions: add(),
		}, "")
	})
	t.Run("E-RABToBeSwitchedDLItem", func(t *testing.T) {
		losslessRoundTrip(t, &ERABToBeSwitchedDLItem{ERABID: 12, TransportLayerAddress: goldTLA(), GTPTEID: 11, SecurityIndication: &SecurityIndication{IntegrityProtectionIndication: IntegrityProtectionNotNeeded}, iEExtensions: tnlUnknownMeta(), extensions: add()}, "")
	})
	t.Run("E-RABToBeSwitchedULItem", func(t *testing.T) {
		losslessRoundTrip(t, &ERABToBeSwitchedULItem{ERABID: 13, TransportLayerAddress: goldTLA(), GTPTEID: 12, iEExtensions: ext(), extensions: add()}, "")
	})
	t.Run("TransportInformation", func(t *testing.T) {
		losslessRoundTrip(t, &TransportInformation{TransportLayerAddress: goldTLA(), ULGTPTEID: 13, extensions: add()}, "")
	})
}

func TestTNLItemUnknownRejectExtensionNotComprehended(t *testing.T) {
	w := per.NewWriter()
	if err := encodeExtensionContainer(w, per.Aligned, []ieField{{id: idUnknownExtension, crit: CriticalityReject, raw: []byte{0x00}}}); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()

	var meta messageMeta

	err := decodeExtensionContainer(per.NewReader(w.Bytes()), per.Aligned, eRABToBeModifyItemBearerModReqExtIEs, new(ERABToBeModifiedItemBearerModReq), &meta)
	if err == nil {
		t.Fatal("unknown reject extension accepted")
	}
}

func TestTNLListsLossless(t *testing.T) {
	t.Run("E-RABToBeSetupListCtxtSUReq", func(t *testing.T) {
		l := ERABToBeSetupListCtxtSUReq{
			{ERABID: 1, QoS: goldQoS(), TransportLayerAddress: goldTLA(), GTPTEID: 1},
			{ERABID: 2, QoS: goldQoS(), TransportLayerAddress: goldTLA(), GTPTEID: 2, iEExtensions: tnlUnknownMeta(), containerCrit: ignoreCrit()},
		}
		losslessRoundTrip(t, &l, "")
	})
	t.Run("E-RABSetupListCtxtSURes", func(t *testing.T) {
		l := ERABSetupListCtxtSURes{{ERABID: 1, TransportLayerAddress: goldTLA(), GTPTEID: 1, containerCrit: rejectCrit()}}
		losslessRoundTrip(t, &l, "000032000a021f0a00000100000001")
	})
	t.Run("E-RABToBeSetupListBearerSUReq", func(t *testing.T) {
		l := ERABToBeSetupListBearerSUReq{{ERABID: 1, QoS: goldQoS(), TransportLayerAddress: goldTLA(), GTPTEID: 1, NASPDU: NASPDU{0x27}, containerCrit: ignoreCrit()}}
		losslessRoundTrip(t, &l, "")
	})
	t.Run("E-RABSetupListBearerSURes", func(t *testing.T) {
		l := ERABSetupListBearerSURes{{ERABID: 1, TransportLayerAddress: goldTLA(), GTPTEID: 1, containerCrit: rejectCrit()}}
		losslessRoundTrip(t, &l, "")
	})
	t.Run("E-RABToBeModifiedListBearerModReq", func(t *testing.T) {
		l := ERABToBeModifiedListBearerModReq{{ERABID: 1, QoS: goldQoS(), NASPDU: NASPDU{0x27}, TransportInformation: &TransportInformation{TransportLayerAddress: goldTLA(), ULGTPTEID: 1}, containerCrit: ignoreCrit()}}
		losslessRoundTrip(t, &l, "")
	})
	t.Run("E-RABToBeModifiedListBearerModInd", func(t *testing.T) {
		l := ERABToBeModifiedListBearerModInd{{ERABID: 1, TransportLayerAddress: goldTLA(), DLGTPTEID: 1, containerCrit: ignoreCrit()}}
		losslessRoundTrip(t, &l, "")
	})
	t.Run("E-RABNotToBeModifiedListBearerModInd", func(t *testing.T) {
		l := ERABNotToBeModifiedListBearerModInd{{ERABID: 1, TransportLayerAddress: goldTLA(), DLGTPTEID: 1, containerCrit: ignoreCrit()}}
		losslessRoundTrip(t, &l, "")
	})
	t.Run("E-RABToBeSetupListHOReq", func(t *testing.T) {
		l := ERABToBeSetupListHOReq{{ERABID: 1, TransportLayerAddress: goldTLA(), GTPTEID: 1, QoS: goldQoS(), containerCrit: ignoreCrit()}}
		losslessRoundTrip(t, &l, "")
	})
	t.Run("E-RABAdmittedList", func(t *testing.T) {
		l := ERABAdmittedList{{ERABID: 1, TransportLayerAddress: goldTLA(), GTPTEID: 1, containerCrit: rejectCrit()}}
		losslessRoundTrip(t, &l, "")
	})
	t.Run("E-RABSubjecttoDataForwardingList", func(t *testing.T) {
		l := ERABSubjecttoDataForwardingList{{ERABID: 1, ULTransportLayerAddr: goldTLA(), ULGTPTEID: Ptr(GTPTEID(1)), containerCrit: rejectCrit()}}
		losslessRoundTrip(t, &l, "")
	})
	t.Run("E-RABToBeSwitchedDLList", func(t *testing.T) {
		l := ERABToBeSwitchedDLList{{ERABID: 1, TransportLayerAddress: goldTLA(), GTPTEID: 1, containerCrit: ignoreCrit()}}
		losslessRoundTrip(t, &l, "")
	})
	t.Run("E-RABToBeSwitchedULList", func(t *testing.T) {
		l := ERABToBeSwitchedULList{{ERABID: 1, TransportLayerAddress: goldTLA(), GTPTEID: 1, containerCrit: rejectCrit()}}
		losslessRoundTrip(t, &l, "")
	})
}

func TestTNLListWireCriticality(t *testing.T) {
	tests := []struct {
		name string
		list per.Marshaler
		id   ProtocolIEID
		crit Criticality
	}{
		{"E-RABToBeSetupListCtxtSUReq", ERABToBeSetupListCtxtSUReq{{QoS: goldQoS(), TransportLayerAddress: goldTLA()}}, IDERABToBeSetupItemCtxtSUReq, CriticalityReject},
		{"E-RABSetupListCtxtSURes", ERABSetupListCtxtSURes{{TransportLayerAddress: goldTLA()}}, IDERABSetupItemCtxtSURes, CriticalityIgnore},
		{"E-RABToBeSetupListBearerSUReq", ERABToBeSetupListBearerSUReq{{QoS: goldQoS(), TransportLayerAddress: goldTLA()}}, IDERABToBeSetupItemBearerSUReq, CriticalityReject},
		{"E-RABSetupListBearerSURes", ERABSetupListBearerSURes{{TransportLayerAddress: goldTLA()}}, IDERABSetupItemBearerSURes, CriticalityIgnore},
		{"E-RABToBeModifiedListBearerModReq", ERABToBeModifiedListBearerModReq{{QoS: goldQoS()}}, IDERABToBeModifiedItemBearerModReq, CriticalityReject},
		{"E-RABToBeModifiedListBearerModInd", ERABToBeModifiedListBearerModInd{{TransportLayerAddress: goldTLA()}}, IDERABToBeModifiedItemBearerModInd, CriticalityReject},
		{"E-RABNotToBeModifiedListBearerModInd", ERABNotToBeModifiedListBearerModInd{{TransportLayerAddress: goldTLA()}}, IDERABNotToBeModifiedItemBearerModInd, CriticalityReject},
		{"E-RABToBeSetupListHOReq", ERABToBeSetupListHOReq{{QoS: goldQoS(), TransportLayerAddress: goldTLA()}}, IDERABToBeSetupItemHOReq, CriticalityReject},
		{"E-RABAdmittedList", ERABAdmittedList{{TransportLayerAddress: goldTLA()}}, IDERABAdmittedItem, CriticalityIgnore},
		{"E-RABSubjecttoDataForwardingList", ERABSubjecttoDataForwardingList{{}}, IDERABDataForwardingItem, CriticalityIgnore},
		{"E-RABToBeSwitchedDLList", ERABToBeSwitchedDLList{{TransportLayerAddress: goldTLA()}}, IDERABToBeSwitchedDLItem, CriticalityReject},
		{"E-RABToBeSwitchedULList", ERABToBeSwitchedULList{{TransportLayerAddress: goldTLA()}}, IDERABToBeSwitchedULItem, CriticalityIgnore},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := per.NewWriter()
			if err := tt.list.MarshalPER(w, per.Aligned); err != nil {
				t.Fatal(err)
			}

			w.AlignToByte()
			r := per.NewReader(w.Bytes())

			if err := per.DecodeLength(r, per.Aligned, 1, maxnoofERABs, true, func(int64) error { return nil }); err != nil {
				t.Fatal(err)
			}

			id, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, maxProtocolIEs)
			if err != nil {
				t.Fatal(err)
			}

			crit, err := per.DecodeEnumerated(r, per.Aligned, criticalityRootCount, false)
			if err != nil {
				t.Fatal(err)
			}

			if ProtocolIEID(id) != tt.id || Criticality(crit) != tt.crit {
				t.Errorf("item %v/%v, want %v/%v", ProtocolIEID(id), Criticality(crit), tt.id, tt.crit)
			}
		})
	}
}

func tnlGoldenCases() []newMessageCase {
	return []newMessageCase{
		{
			name: "PathSwitchRequestAcknowledgeERABToBeSwitchedULList",
			msg: &PathSwitchRequestAcknowledge{
				MMEUES1APID:        Ptr(MMEUES1APID(1)),
				ENBUES1APID:        Ptr(ENBUES1APID(2)),
				ERABToBeSwitchedUL: ERABToBeSwitchedULList{{ERABID: 5, TransportLayerAddress: TransportLayerAddress{10, 0, 0, 9}, GTPTEID: 0x01020304}},
				SecurityContext:    SecurityContext{NextHopChainingCount: 3},
			},
			parse:  func(b []byte) (any, error) { return ParsePathSwitchRequestAcknowledge(b) },
			choice: isSuccessful,
			golden: "20030047000004000040020001000840020002005f400f00005e400a0a1f0a0000090102030400280021180000000000000000000000000000000000000000000000000000000000000000",
		},
		{
			name: "ERABModifyRequestTransportInformation",
			msg: &ERABModifyRequest{
				MMEUES1APID: 1,
				ENBUES1APID: 2,
				ERABToBeModified: ERABToBeModifiedListBearerModReq{{
					ERABID:               5,
					QoS:                  goldQoS(),
					NASPDU:               NASPDU{0x07, 0x46},
					TransportInformation: &TransportInformation{TransportLayerAddress: TransportLayerAddress{10, 0, 0, 9}, ULGTPTEID: 0x01020304},
				}},
			},
			parse:  func(b []byte) (any, error) { return ParseERABModifyRequest(b) },
			choice: isInitiating,
			golden: "0006002f000003000000020001000800020002001e001c00002400174a000904020746000000b9000a07c00a00000901020304",
		},
		{
			name: "InitialContextSetupRequestERABLossless",
			msg: &InitialContextSetupRequest{
				MMEUES1APID:               1,
				ENBUES1APID:               2,
				UEAggregateMaximumBitRate: UEAggregateMaximumBitRate{DL: 1000, UL: 1000},
				ERABToBeSetup: ERABToBeSetupListCtxtSUReq{{
					ERABID:                5,
					QoS:                   goldQoS(),
					TransportLayerAddress: goldTLA(),
					GTPTEID:               1,
					CorrelationID:         &CorrelationID{0x01, 0x02, 0x03, 0x04},
					extensions:            tnlExtensionAdditions([]byte{0xaa}),
					containerCrit:         ignoreCrit(),
				}},
				UESecurityCapabilities: UESecurityCapabilities{},
			},
			parse:            func(b []byte) (any, error) { return ParseInitialContextSetupRequest(b) },
			choice:           isInitiating,
			golden:           "0009006a000006000000020001000800020002004200060803e82003e800180020000034401ba50009040f800a000001000000010000009c4004010203040101aa006b00050000000000004900200000000000000000000000000000000000000000000000000000000000000000",
			extensionNotices: 1,
		},
		{
			name: "InitialContextSetupRequestKnownERABExtensions",
			msg: &InitialContextSetupRequest{
				MMEUES1APID:               1,
				ENBUES1APID:               2,
				UEAggregateMaximumBitRate: UEAggregateMaximumBitRate{DL: 1000, UL: 1000},
				ERABToBeSetup: ERABToBeSetupListCtxtSUReq{{
					ERABID:                5,
					QoS:                   knownExtensionQoS(),
					TransportLayerAddress: goldTLA(),
					GTPTEID:               1,
					CorrelationID:         &CorrelationID{0x01, 0x02, 0x03, 0x04},
					SIPTOCorrelationID:    &CorrelationID{0x05, 0x06, 0x07, 0x08},
					BearerType:            Ptr(BearerTypeNonIP),
					EthernetType:          Ptr(EthernetTypeTrue),
					SecurityIndication:    &SecurityIndication{IntegrityProtectionIndication: IntegrityProtectionPreferred},
				}},
				UESecurityCapabilities: UESecurityCapabilities{},
			},
			parse:  func(b []byte) (any, error) { return ParseInitialContextSetupRequest(b) },
			choice: isInitiating,
			golden: goldenICSKnownERABExtensions,
		},
		{
			name: "PathSwitchRequestDLItemSecurityIndication",
			msg: &PathSwitchRequest{
				ENBUES1APID: 2,
				ERABToBeSwitchedDL: ERABToBeSwitchedDLList{{
					ERABID: 5, TransportLayerAddress: TransportLayerAddress{10, 1, 2, 3}, GTPTEID: 0x01020304,
					SecurityIndication: &SecurityIndication{IntegrityProtectionIndication: IntegrityProtectionRequired},
				}},
				SourceMMEUES1APID:      0x020000bf,
				EUTRANCGI:              Ptr(EUTRANCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 0x123c601}),
				TAI:                    Ptr(TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1}),
				UESecurityCapabilities: Ptr(UESecurityCapabilities{EncryptionAlgorithms: 0xc000, IntegrityProtectionAlgorithms: 0xc000}),
			},
			parse:  func(b []byte) (any, error) { return ParsePathSwitchRequest(b) },
			choice: isInitiating,
			golden: goldenPathSwitchDLSecurityIndication,
		},
	}
}

func knownExtensionQoS() ERABLevelQoSParameters {
	return ERABLevelQoSParameters{
		QCI: 1,
		ARP: AllocationAndRetentionPriority{PriorityLevel: 2},
		GBR: &GBRQosInformation{
			MaximumBitrateDL:            1000,
			MaximumBitrateUL:            1000,
			GuaranteedBitrateDL:         500,
			GuaranteedBitrateUL:         500,
			ExtendedMaximumBitrateDL:    Ptr(ExtendedBitRate(10000000001)),
			ExtendedMaximumBitrateUL:    Ptr(ExtendedBitRate(4000000000000)),
			ExtendedGuaranteedBitrateDL: Ptr(ExtendedBitRate(20000000000)),
			ExtendedGuaranteedBitrateUL: Ptr(ExtendedBitRate(30000000000)),
		},
		DownlinkPacketLossRate: Ptr(PacketLossRate(10)),
		UplinkPacketLossRate:   Ptr(PacketLossRate(1000)),
	}
}

const (
	goldenICSKnownERABExtensions         = ""
	goldenPathSwitchDLSecurityIndication = "0003004b0000060008000200020016001600001700114a1f0a010203010203040000014c40010000580005c0020000bf006440080000f110123c6010004340060000f1100001006b400518000c0000"
)

func tnlListCodecs() []func() tnlValue {
	return []func() tnlValue{
		func() tnlValue { return new(ERABToBeSetupListCtxtSUReq) },
		func() tnlValue { return new(ERABSetupListCtxtSURes) },
		func() tnlValue { return new(ERABToBeSetupListBearerSUReq) },
		func() tnlValue { return new(ERABSetupListBearerSURes) },
		func() tnlValue { return new(ERABToBeModifiedListBearerModReq) },
		func() tnlValue { return new(ERABToBeModifiedListBearerModInd) },
		func() tnlValue { return new(ERABNotToBeModifiedListBearerModInd) },
		func() tnlValue { return new(ERABToBeSetupListHOReq) },
		func() tnlValue { return new(ERABAdmittedList) },
		func() tnlValue { return new(ERABSubjecttoDataForwardingList) },
		func() tnlValue { return new(ERABToBeSwitchedDLList) },
		func() tnlValue { return new(ERABToBeSwitchedULList) },
	}
}

func FuzzERABListRoundTrip(f *testing.F) {
	add := tnlExtensionAdditions([]byte{0xaa})

	seeds := []per.Marshaler{
		ERABToBeSetupListCtxtSUReq{{ERABID: 1, QoS: tnlQoS(), TransportLayerAddress: goldTLA(), GTPTEID: 1, CorrelationID: &CorrelationID{1, 2, 3, 4}, extensions: add, containerCrit: ignoreCrit()}},
		ERABSetupListCtxtSURes{{ERABID: 1, TransportLayerAddress: goldTLA(), GTPTEID: 1, iEExtensions: tnlUnknownExtension()}},
		ERABToBeSetupListBearerSUReq{{ERABID: 1, QoS: goldQoS(), TransportLayerAddress: goldTLA(), NASPDU: NASPDU{0x07}}},
		ERABSetupListBearerSURes{{ERABID: 1, TransportLayerAddress: goldTLA()}},
		ERABToBeModifiedListBearerModReq{{ERABID: 1, QoS: goldQoS(), NASPDU: NASPDU{0x07}, TransportInformation: &TransportInformation{TransportLayerAddress: goldTLA(), ULGTPTEID: 1}}},
		ERABToBeModifiedListBearerModInd{{ERABID: 1, TransportLayerAddress: goldTLA(), extensions: add}},
		ERABNotToBeModifiedListBearerModInd{{ERABID: 1, TransportLayerAddress: goldTLA()}},
		ERABToBeSetupListHOReq{{ERABID: 1, TransportLayerAddress: goldTLA(), QoS: goldQoS(), Extensions: &ERABToBeSetupItemHOReqExtIEs{DataForwardingNotPossible: Ptr(DataForwardingNotPossibleTrue)}}},
		ERABAdmittedList{{ERABID: 1, TransportLayerAddress: goldTLA(), DLTransportLayerAddr: goldTLA(), DLGTPTEID: Ptr(GTPTEID(2))}},
		ERABSubjecttoDataForwardingList{{ERABID: 1, ULTransportLayerAddr: goldTLA(), ULGTPTEID: Ptr(GTPTEID(2)), iEExtensions: tnlUnknownExtension()}},
		ERABToBeSwitchedDLList{{ERABID: 1, TransportLayerAddress: goldTLA()}},
		ERABToBeSwitchedULList{{ERABID: 1, TransportLayerAddress: goldTLA(), containerCrit: rejectCrit()}},
	}

	for i, s := range seeds {
		w := per.NewWriter()
		if err := s.MarshalPER(w, per.Aligned); err != nil {
			f.Fatal(err)
		}

		w.AlignToByte()
		f.Add(uint8(i), w.Bytes())
	}

	codecs := tnlListCodecs()

	f.Fuzz(func(t *testing.T, kind uint8, data []byte) {
		newValue := codecs[int(kind)%len(codecs)]

		v := newValue()
		if err := v.UnmarshalPER(per.NewReader(data), per.Aligned); err != nil {
			return
		}

		w := per.NewWriter()
		if err := v.MarshalPER(w, per.Aligned); err != nil {
			t.Fatalf("encode decoded value: %v", err)
		}

		w.AlignToByte()
		b := w.Bytes()

		again := newValue()
		if err := again.UnmarshalPER(per.NewReader(b), per.Aligned); err != nil {
			t.Fatalf("decode re-encoded value: %v", err)
		}

		if !reflect.DeepEqual(again, v) {
			t.Fatalf("re-decoded\n got %+v\nwant %+v", again, v)
		}

		w = per.NewWriter()
		if err := again.MarshalPER(w, per.Aligned); err != nil {
			t.Fatal(err)
		}

		w.AlignToByte()

		if !bytes.Equal(w.Bytes(), b) {
			t.Fatalf("re-encoded %x, want %x", w.Bytes(), b)
		}
	})
}

func TestTNLKnownExtensionCriticality(t *testing.T) {
	type row struct {
		id   ProtocolIEID
		crit Criticality
	}

	rows := func(ids []ProtocolIEID, crits []Criticality) []row {
		out := make([]row, len(ids))
		for i := range ids {
			out[i] = row{ids[i], crits[i]}
		}

		return out
	}

	ig, rj := CriticalityIgnore, CriticalityReject

	for _, tc := range []struct {
		name string
		got  []row
		want []row
	}{
		{"E-RABToBeSetupItemCtxtSUReqExtIEs", rows(tableIDs(eRABToBeSetupItemCtxtSUReqExtIEs), tableCrits(eRABToBeSetupItemCtxtSUReqExtIEs)), []row{
			{IDCorrelationID, ig}, {IDSIPTOCorrelationID, ig}, {IDBearerType, rj}, {IDEthernetType, ig}, {IDSecurityIndication, rj},
		}},
		{"E-RABToBeSetupItemBearerSUReqExtIEs", rows(tableIDs(eRABToBeSetupItemBearerSUReqExtIEs), tableCrits(eRABToBeSetupItemBearerSUReqExtIEs)), []row{
			{IDCorrelationID, ig}, {IDSIPTOCorrelationID, ig}, {IDBearerType, rj}, {IDEthernetType, ig}, {IDSecurityIndication, rj},
		}},
		{"E-RABToBeSetupItemHOReq-ExtIEs", rows(tableIDs(eRABToBeSetupItemHOReqExtIEs), tableCrits(eRABToBeSetupItemHOReqExtIEs)), []row{
			{IDDataForwardingNotPossible, ig}, {IDBearerType, rj}, {IDEthernetType, ig}, {IDSecurityIndication, rj},
		}},
		{"E-RABToBeSwitchedDLItem-ExtIEs", rows(tableIDs(eRABToBeSwitchedDLItemExtIEs), tableCrits(eRABToBeSwitchedDLItemExtIEs)), []row{{IDSecurityIndication, ig}}},
		{"E-RABQoSParameters-ExtIEs", rows(tableIDs(eRABLevelQoSParametersExtIEs), tableCrits(eRABLevelQoSParametersExtIEs)), []row{
			{IDDownlinkPacketLossRate, ig}, {IDUplinkPacketLossRate, ig},
		}},
		{"GBR-QosInformation-ExtIEs", rows(tableIDs(gBRQosInformationExtIEs), tableCrits(gBRQosInformationExtIEs)), []row{
			{IDExtendedERABMaximumBitrateDL, ig}, {IDExtendedERABMaximumBitrateUL, ig}, {IDExtendedERABGuaranteedBitrateDL, ig}, {IDExtendedERABGuaranteedBitrateUL, ig},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !reflect.DeepEqual(tc.got, tc.want) {
				t.Fatalf("rows %v, want %v", tc.got, tc.want)
			}
		})
	}
}

func tableCrits[M any](table []ieSpec[M]) []Criticality {
	out := make([]Criticality, len(table))
	for i := range table {
		out[i] = table[i].crit
	}

	return out
}

func TestTNLItemKeepsManyUnknownExtensions(t *testing.T) {
	fields := make([]ieField, 0, 3*maxPreservedIEs)
	for i := range 3 * maxPreservedIEs {
		fields = append(fields, ieField{id: idUnknownExtension + ProtocolIEID(i), crit: CriticalityIgnore, raw: []byte{byte(i) | 1}})
	}

	in := &ERABToBeSetupItemCtxtSUReq{ERABID: 5, QoS: goldQoS(), TransportLayerAddress: goldTLA(), GTPTEID: 1, BearerType: Ptr(BearerTypeNonIP)}

	w := per.NewWriter()
	if err := encodeExtensionContainer(w, per.Aligned, fields); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()

	if err := decodeExtensionContainer(per.NewReader(w.Bytes()), per.Aligned, eRABToBeSetupItemCtxtSUReqExtIEs, in, &in.iEExtensions); err != nil {
		t.Fatal(err)
	}

	if got := len(in.iEExtensions.UnknownIEs()); got != 3*maxPreservedIEs {
		t.Fatalf("kept %d unknown extensions, want %d", got, 3*maxPreservedIEs)
	}

	losslessRoundTrip(t, in, "")
}

func TestMessageTruncatedOnDecodeRefusesToEncode(t *testing.T) {
	fields := uplinkNASFields()
	for i := range maxPreservedIEs + 1 {
		fields = append(fields, ieField{id: ProtocolIEID(40000 + i), crit: CriticalityIgnore, raw: []byte{0x00}})
	}

	msg, err := ParseUplinkNASTransport(container(t, fields...))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := msg.Marshal(); err == nil {
		t.Fatal("a message whose unknown IEs were dropped on decode re-encoded")
	}
}
