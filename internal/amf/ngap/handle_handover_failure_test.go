// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleHandoverFailure_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	amf := newTestAMF()
	msg := &ngapType.HandoverFailure{}

	assertNoPanic(t, "HandleHandoverFailure(empty IEs)", func() {
		ngap.HandleHandoverFailure(context.Background(), amf, ran, msg)
	})
}

func TestHandleHandoverFailure_MissingCause(t *testing.T) {
	ran := newTestRadio()
	amf := newTestAMF()
	msg := &ngapType.HandoverFailure{}
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.HandoverFailureIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.HandoverFailureIEsValue{
			Present:     ngapType.HandoverFailureIEsPresentAMFUENGAPID,
			AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 1},
		},
	})

	assertNoPanic(t, "HandleHandoverFailure(missing cause)", func() {
		ngap.HandleHandoverFailure(context.Background(), amf, ran, msg)
	})
}

// TestHandleHandoverFailure_SourceAmfUeDetached verifies that a handover
// failure is handled gracefully when the source UE's AMF UE context has been
// detached (e.g. due to a concurrent deregistration).
func TestHandleHandoverFailure_SourceAmfUeDetached(t *testing.T) {
	sourceRan := newTestRadio()
	targetRan := newTestRadio()
	amfInstance := newTestAMF()

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog

	sourceUe := &amf.RanUe{
		RanUeNgapID: 10,
		AmfUeNgapID: 100,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(sourceUe)
	sourceRan.RanUEs[10] = sourceUe

	targetUe := &amf.RanUe{
		RanUeNgapID: 2,
		AmfUeNgapID: 200,
		Radio:       targetRan,
		Log:         logger.AmfLog,
	}

	err := amf.AttachSourceUeTargetUe(sourceUe, targetUe)
	if err != nil {
		t.Fatalf("AttachSourceUeTargetUe: %v", err)
	}

	targetRan.RanUEs[2] = targetUe

	amfInstance.Radios[new(sctp.SCTPConn)] = sourceRan
	amfInstance.Radios[new(sctp.SCTPConn)] = targetRan

	// Simulate the AMF UE being detached from the source (deregistration race).
	amfUe.DetachRanUe()

	msg := &ngapType.HandoverFailure{}
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.HandoverFailureIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.HandoverFailureIEsValue{
			Present:     ngapType.HandoverFailureIEsPresentAMFUENGAPID,
			AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 200},
		},
	})
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.HandoverFailureIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDCause},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.HandoverFailureIEsValue{
			Present: ngapType.HandoverFailureIEsPresentCause,
			Cause: &ngapType.Cause{
				Present: ngapType.CausePresentRadioNetwork,
				RadioNetwork: &ngapType.CauseRadioNetwork{
					Value: ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem,
				},
			},
		},
	})

	assertNoPanic(t, "HandleHandoverFailure(source AmfUe detached)", func() {
		ngap.HandleHandoverFailure(context.Background(), amfInstance, targetRan, msg)
	})
}
