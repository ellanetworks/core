// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/s1ap"
)

func modeledIEsCorpus(t *testing.T) map[string][]byte {
	t.Helper()

	gummeiID := s1ap.GUMMEI{PLMNIdentity: testPLMN, MMEGroupID: s1ap.MMEGroupID{0x80, 0x01}, MMECode: 0x02}

	return map[string][]byte{
		"initial_context_setup_request_gummei_mme_ue_s1ap_id_2": mustMarshal(t, &s1ap.InitialContextSetupRequest{
			MMEUES1APID: 1,
			ENBUES1APID: 2,
			ERABToBeSetup: []s1ap.ERABToBeSetupItemCtxtSUReq{{
				ERABID:                5,
				QoS:                   s1ap.ERABLevelQoSParameters{QCI: 9, ARP: s1ap.AllocationAndRetentionPriority{PriorityLevel: 15}},
				TransportLayerAddress: s1ap.TransportLayerAddress{10, 0, 0, 1},
				GTPTEID:               1,
			}},
			GUMMEI:       &gummeiID,
			MMEUES1APID2: s1ap.Ptr(s1ap.MMEUES1APID(3)),
		}),
		"handover_request_gummei_mme_ue_s1ap_id_2": mustMarshal(t, &s1ap.HandoverRequest{
			MMEUES1APID:    1,
			HandoverType:   s1ap.HandoverTypeIntraLTE,
			Cause:          &s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 16},
			SourceToTarget: s1ap.TransparentContainer{0x00},
			ERABToBeSetup: []s1ap.ERABToBeSetupItemHOReq{{
				ERABID:                5,
				TransportLayerAddress: s1ap.TransportLayerAddress{10, 0, 0, 1},
				GTPTEID:               1,
				QoS:                   s1ap.ERABLevelQoSParameters{QCI: 9, ARP: s1ap.AllocationAndRetentionPriority{PriorityLevel: 15}},
			}},
			GUMMEI:       &gummeiID,
			MMEUES1APID2: s1ap.Ptr(s1ap.MMEUES1APID(3)),
		}),
		"path_switch_request_acknowledge_mme_ue_s1ap_id_2": mustMarshal(t, &s1ap.PathSwitchRequestAcknowledge{
			MMEUES1APID:  s1ap.Ptr(s1ap.MMEUES1APID(1)),
			ENBUES1APID:  s1ap.Ptr(s1ap.ENBUES1APID(2)),
			MMEUES1APID2: s1ap.Ptr(s1ap.MMEUES1APID(3)),
		}),
		"ue_context_release_request_gw_context_release_indication": mustMarshal(t, &s1ap.UEContextReleaseRequest{
			MMEUES1APID:                1,
			ENBUES1APID:                2,
			Cause:                      &s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkX2HandoverTriggered},
			GWContextReleaseIndication: s1ap.Ptr(s1ap.GWContextReleaseIndicationTrue),
		}),
	}
}

func TestDecodeS1APMessage_ModeledIEs(t *testing.T) {
	for name, raw := range modeledIEsCorpus(t) {
		t.Run(name, func(t *testing.T) {
			msg := DecodeS1APMessage(raw)

			if msg.Value.Error != "" {
				t.Fatalf("decode error: %s", msg.Value.Error)
			}

			pdu, err := s1ap.ParseRawPDU(raw)
			if err != nil {
				t.Fatal(err)
			}

			wire := pdu.IEs()
			if len(wire) != len(msg.Value.IEs) {
				t.Fatalf("rendered %d IEs, want the %d on the wire", len(msg.Value.IEs), len(wire))
			}

			for i, w := range wire {
				if msg.Value.IEs[i].ID.Value != int64(w.ID) {
					t.Errorf("IE %d = %d, want %d", i, msg.Value.IEs[i].ID.Value, w.ID)
				}
			}

			for _, e := range msg.Value.IEs {
				if e.ID.Unknown {
					t.Errorf("IE %d rendered as unknown", e.ID.Value)
				}

				if _, raw := e.Value.(rawIEValue); raw {
					t.Errorf("IE %s rendered as unmodeled", e.ID.Label)
				}
			}
		})
	}
}

func TestDecodeS1APMessage_GUMMEIAndMMEUES1APID2(t *testing.T) {
	for _, name := range []string{"initial_context_setup_request_gummei_mme_ue_s1ap_id_2", "handover_request_gummei_mme_ue_s1ap_id_2"} {
		t.Run(name, func(t *testing.T) {
			ies := DecodeS1APMessage(modeledIEsCorpus(t)[name]).Value.IEs
			if len(ies) < 2 {
				t.Fatalf("IEs = %+v", ies)
			}

			g, id2 := ies[len(ies)-2], ies[len(ies)-1]

			if g.ID.Value != int64(s1ap.IDGUMMEI) || g.Criticality.Value != int64(s1ap.CriticalityIgnore) {
				t.Fatalf("GUMMEI IE = %+v", g)
			}

			if v, ok := g.Value.(GUMMEI); !ok || v != gummei(s1ap.GUMMEI{PLMNIdentity: testPLMN, MMEGroupID: s1ap.MMEGroupID{0x80, 0x01}, MMECode: 0x02}) {
				t.Fatalf("GUMMEI = %#v", g.Value)
			}

			if id2.ID.Value != int64(s1ap.IDMMEUES1APID2) || id2.Criticality.Value != int64(s1ap.CriticalityIgnore) || id2.Value != uint32(3) {
				t.Fatalf("MME UE S1AP ID 2 IE = %+v", id2)
			}
		})
	}
}

func TestDecodeS1APMessage_UEContextReleaseRequestGWContextReleaseIndication(t *testing.T) {
	ies := DecodeS1APMessage(modeledIEsCorpus(t)["ue_context_release_request_gw_context_release_indication"]).Value.IEs

	last := ies[len(ies)-1]
	if last.ID.Value != int64(s1ap.IDGWContextReleaseIndication) || last.Criticality.Value != int64(s1ap.CriticalityReject) || last.Value != true {
		t.Fatalf("IEs = %+v, want GW Context Release Indication true last", ies)
	}
}
