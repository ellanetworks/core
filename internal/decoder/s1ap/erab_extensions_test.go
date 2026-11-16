// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ellanetworks/core/per/ap"
	"github.com/ellanetworks/core/s1ap"
)

func knownExtensionQoS() s1ap.ERABLevelQoSParameters {
	return s1ap.ERABLevelQoSParameters{
		QCI: 1,
		ARP: s1ap.AllocationAndRetentionPriority{PriorityLevel: 2},
		GBR: &s1ap.GBRQosInformation{
			MaximumBitrateDL: 1, MaximumBitrateUL: 1, GuaranteedBitrateDL: 1, GuaranteedBitrateUL: 1,
			ExtendedMaximumBitrateDL:    s1ap.Ptr(s1ap.ExtendedBitRate(10000000001)),
			ExtendedMaximumBitrateUL:    s1ap.Ptr(s1ap.ExtendedBitRate(10000000002)),
			ExtendedGuaranteedBitrateDL: s1ap.Ptr(s1ap.ExtendedBitRate(10000000003)),
			ExtendedGuaranteedBitrateUL: s1ap.Ptr(s1ap.ExtendedBitRate(10000000004)),
		},
		DownlinkPacketLossRate: s1ap.Ptr(s1ap.PacketLossRate(10)),
		UplinkPacketLossRate:   s1ap.Ptr(s1ap.PacketLossRate(20)),
	}
}

func TestDecodeS1APMessage_KnownERABExtensions(t *testing.T) {
	sec := &s1ap.SecurityIndication{IntegrityProtectionIndication: s1ap.IntegrityProtectionPreferred}
	tla := ap.TransportLayerAddressFromOctets([]byte{10, 0, 0, 1})
	qosKeys := []string{
		`"downlink_packet_loss_rate":10`, `"uplink_packet_loss_rate":20`,
		`"extended_maximum_bitrate_dl":10000000001`, `"extended_maximum_bitrate_ul":10000000002`,
		`"extended_guaranteed_bitrate_dl":10000000003`, `"extended_guaranteed_bitrate_ul":10000000004`,
	}
	setupKeys := []string{`"bearer_type"`, `"non-IP"`, `"ethernet_type"`, `"integrity_protection_indication"`, `"preferred"`}

	for _, c := range []struct {
		name string
		raw  []byte
		want []string
	}{
		{
			name: "initial_context_setup_request",
			raw: mustMarshal(t, &s1ap.InitialContextSetupRequest{
				MMEUES1APID: 1,
				ENBUES1APID: 2,
				ERABToBeSetup: s1ap.ERABToBeSetupListCtxtSUReq{{
					ERABID: 5, QoS: knownExtensionQoS(), TransportLayerAddress: tla, GTPTEID: 1,
					CorrelationID: &s1ap.CorrelationID{1, 2, 3, 4}, SIPTOCorrelationID: &s1ap.CorrelationID{5, 6, 7, 8},
					BearerType: s1ap.Ptr(s1ap.BearerTypeNonIP), EthernetType: s1ap.Ptr(s1ap.EthernetTypeTrue), SecurityIndication: sec,
				}},
			}),
			want: append(append([]string{`"correlation_id":"01020304"`, `"sipto_correlation_id":"05060708"`}, setupKeys...), qosKeys...),
		},
		{
			name: "erab_setup_request",
			raw: mustMarshal(t, &s1ap.ERABSetupRequest{
				MMEUES1APID: 1,
				ENBUES1APID: 2,
				ERABToBeSetup: s1ap.ERABToBeSetupListBearerSUReq{{
					ERABID: 6, QoS: knownExtensionQoS(), TransportLayerAddress: tla, GTPTEID: 2, NASPDU: s1ap.NASPDU{0x27},
					CorrelationID: &s1ap.CorrelationID{1, 2, 3, 4}, SIPTOCorrelationID: &s1ap.CorrelationID{5, 6, 7, 8},
					BearerType: s1ap.Ptr(s1ap.BearerTypeNonIP), EthernetType: s1ap.Ptr(s1ap.EthernetTypeTrue), SecurityIndication: sec,
				}},
			}),
			want: append(append([]string{`"correlation_id":"01020304"`, `"sipto_correlation_id":"05060708"`}, setupKeys...), qosKeys...),
		},
		{
			name: "erab_modify_request",
			raw: mustMarshal(t, &s1ap.ERABModifyRequest{
				MMEUES1APID: 1,
				ENBUES1APID: 2,
				ERABToBeModified: s1ap.ERABToBeModifiedListBearerModReq{{
					ERABID: 5, QoS: knownExtensionQoS(), NASPDU: s1ap.NASPDU{0x07},
				}},
			}),
			want: qosKeys,
		},
		{
			name: "handover_request",
			raw: mustMarshal(t, &s1ap.HandoverRequest{
				MMEUES1APID:    1,
				HandoverType:   s1ap.HandoverTypeUTRANtoLTE,
				Cause:          s1ap.Ptr(s1ap.CauseRadioNetworkHandoverDesirableForRadio.Cause()),
				SourceToTarget: testSourceeNBContainer(t),

				NASSecurityParameterstoEUTRAN: s1ap.NASSecurityParameterstoEUTRAN{0x0a, 0x0b},
				ERABToBeSetup: s1ap.ERABToBeSetupListHOReq{{
					ERABID: 5, TransportLayerAddress: tla, GTPTEID: 1, QoS: knownExtensionQoS(),
					DataForwardingNotPossible: s1ap.Ptr(s1ap.DataForwardingNotPossibleTrue),
					BearerType:                s1ap.Ptr(s1ap.BearerTypeNonIP),
					EthernetType:              s1ap.Ptr(s1ap.EthernetTypeTrue),
					SecurityIndication:        sec,
				}},
			}),
			want: append(append([]string{`"data_forwarding_not_possible":true`}, setupKeys...), qosKeys...),
		},
		{
			name: "path_switch_request",
			raw: mustMarshal(t, &s1ap.PathSwitchRequest{
				ENBUES1APID:            2,
				ERABToBeSwitchedDL:     s1ap.ERABToBeSwitchedDLList{{ERABID: 5, TransportLayerAddress: tla, GTPTEID: 1, SecurityIndication: sec}},
				SourceMMEUES1APID:      1,
				EUTRANCGI:              &s1ap.EUTRANCGI{PLMNIdentity: testPLMN, CellID: 1},
				TAI:                    &s1ap.TAI{PLMNIdentity: testPLMN, TAC: 1},
				UESecurityCapabilities: &s1ap.UESecurityCapabilities{},
			}),
			want: []string{`"integrity_protection_indication"`, `"preferred"`},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			msg := DecodeS1APMessage(c.raw)
			if msg.Value.Error != "" {
				t.Fatalf("decode error: %s", msg.Value.Error)
			}

			out, err := json.Marshal(msg)
			if err != nil {
				t.Fatal(err)
			}

			for _, w := range c.want {
				if !strings.Contains(string(out), w) {
					t.Errorf("rendering lacks %s: %s", w, out)
				}
			}
		})
	}
}
